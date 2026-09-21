package main

import (
	"context"
	"database/sql"
	"fmt"
	"net"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/nats-io/nats.go"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"

	"github.com/walkline/ToCloud9/apps/guildserver"
	"github.com/walkline/ToCloud9/apps/guildserver/config"
	"github.com/walkline/ToCloud9/apps/guildserver/repo"
	"github.com/walkline/ToCloud9/apps/guildserver/server"
	"github.com/walkline/ToCloud9/apps/guildserver/service"
	pbChar "github.com/walkline/ToCloud9/gen/characters/pb"
	"github.com/walkline/ToCloud9/gen/guilds/pb"
	"github.com/walkline/ToCloud9/shared/events"
	shrepo "github.com/walkline/ToCloud9/shared/repo"
)

func init() {
	// set service identifier with almost random number
	guildserver.ServiceID = strconv.Itoa(time.Now().Nanosecond())
}

func main() {
	cfg, err := config.LoadConfig()
	if err != nil {
		panic(err)
	}

	log.Logger = cfg.Logger()

	// nats setup
	nc, err := nats.Connect(
		cfg.NatsURL,
		nats.PingInterval(20*time.Second),
		nats.MaxPingsOutstanding(5),
		nats.Timeout(10*time.Second),
		nats.Name("guildserver"),
	)
	if err != nil {
		log.Fatal().Err(err).Msg("can't connect to the Nats")
	}
	defer nc.Close()

	// grpc setup
	lis, err := net.Listen("tcp4", ":"+cfg.Port)
	if err != nil {
		log.Fatal().Err(err).Msg("can't listen for incoming connections")
	}

	grpcServer := grpc.NewServer()
	guildServer := server.NewGuildServer(createGuildService(cfg, nc))
	if cfg.LogLevel == zerolog.DebugLevel {
		guildServer = server.NewGuildsDebugLoggerMiddleware(guildServer, log.Logger)
	}
	pb.RegisterGuildServiceServer(grpcServer, guildServer)

	// graceful shutdown handling
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		sig := <-sigCh
		fmt.Println("")
		log.Info().Msgf("🧨 Got signal %v, attempting graceful shutdown...", sig)
		grpcServer.GracefulStop()
		wg.Done()
	}()

	log.Info().Str("address", lis.Addr().String()).Msg("🚀 Guild Service started!")

	if err := grpcServer.Serve(lis); err != nil {
		log.Fatal().Err(err).Msg("couldn't serve")
	}

	wg.Wait()

	log.Info().Msg("👍 Server successfully stopped.")
}

func createGuildService(cfg *config.Config, natsCon *nats.Conn) service.GuildService {
	charDB := shrepo.NewCharactersDB()
	for realmID, connStr := range cfg.CharDBConnection {
		cdb, err := sql.Open("mysql", connStr)
		if err != nil {
			log.Fatal().Err(err).Uint32("realmID", realmID).Msg("can't connect to char db")
		}
		configureDBConn(cdb)
		charDB.SetDBForRealm(realmID, cdb)
	}

	guildsRepo, err := repo.NewGuildsMySQLRepo(charDB)
	if err != nil {
		log.Fatal().Err(err).Msg("can't create guilds repo")
	}

	cache := service.NewGuildsInMemCache(guildsRepo)
	err = events.NewGatewayConsumer(
		natsCon,
		events.WithGWConsumerLoggedInHandler(cache),
		events.WithGWConsumerLoggedOutHandler(cache),
		events.WithGWConsumerCharsUpdatesHandler(cache),
	).Listen()
	if err != nil {
		log.Fatal().Err(err).Msg("can't listen to gateway updates")
	}

	err = cache.Warmup(context.Background(), 1)
	if err != nil {
		log.Fatal().Err(err).Msg("can't warmup guilds cache")
	}

	charsListener := service.NewCharactersListener(natsCon, cache)
	err = charsListener.Listen()
	if err != nil {
		log.Fatal().Err(err).Msg("can't listen to characters service events")
	}

	for realmID := range cfg.CharDBConnection {
		seedOnlineChars(cfg, cache, realmID)
	}

	return service.NewGuildService(cache, events.NewGuildServiceProducerNatsJSON(natsCon, guildserver.Ver))
}

// seedOnlineChars recovers the online state of characters from the characters
// service. Login events observed before this process started are gone, so
// without it every member hydrated from the DB stays offline until it relogs.
// Best effort: on failure the roster still works, only statuses degrade.
func seedOnlineChars(cfg *config.Config, cache service.GuildsCache, realmID uint32) {
	if cfg.CharServiceAddress == "" {
		return
	}

	conn, err := grpc.Dial(cfg.CharServiceAddress, grpc.WithInsecure())
	if err != nil {
		log.Warn().Err(err).Msg("can't connect to characters service, skipping online state recovery")
		return
	}
	defer conn.Close()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()

	resp, err := pbChar.NewCharactersServiceClient(conn).GetOnlineCharacters(ctx, &pbChar.GetOnlineCharactersRequest{
		Api:     guildserver.Ver,
		RealmID: realmID,
	})
	if err != nil {
		log.Warn().Err(err).Msg("can't fetch online characters, skipping online state recovery")
		return
	}

	cache.SeedOnlineChars(realmID, resp.CharacterGUIDs)
	log.Info().Int("count", len(resp.CharacterGUIDs)).Msg("recovered online state from characters service")
}

func configureDBConn(db *sql.DB) {
	db.SetMaxIdleConns(5)
	db.SetMaxOpenConns(10)
	db.SetConnMaxLifetime(time.Minute * 4)
	db.SetConnMaxIdleTime(time.Minute * 8)
}
