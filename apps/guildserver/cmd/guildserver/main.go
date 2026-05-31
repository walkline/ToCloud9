package main

import (
	"context"
	"database/sql"
	"fmt"
	"net"
	"os"
	"os/signal"
	"sort"
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
	pbGuid "github.com/walkline/ToCloud9/gen/guid/pb"
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

	worldDB, err := sql.Open("mysql", cfg.WorldDBConnection)
	if err != nil {
		log.Fatal().Err(err).Msg("can't connect to world db")
	}
	configureDBConn(worldDB)

	guildsRepo, err := repo.NewGuildsMySQLRepoWithWorldDB(charDB, worldDB)
	if err != nil {
		log.Fatal().Err(err).Msg("can't create guilds repo")
	}

	guidConn, err := grpc.Dial(cfg.GuidProviderServiceAddress, grpc.WithInsecure())
	if err != nil {
		log.Fatal().Err(err).Str("address", cfg.GuidProviderServiceAddress).Msg("can't connect to guid service")
	}
	itemGUIDAllocator := service.NewGuidServiceItemGUIDAllocator(pbGuid.NewGuidServiceClient(guidConn))

	cache := service.NewGuildsInMemCache(guildsRepo)
	guildService := service.NewGuildServiceWithBankRepoAndItemGUIDAllocator(cache, guildsRepo, itemGUIDAllocator, events.NewGuildServiceProducerNatsJSON(natsCon, guildserver.Ver))
	err = events.NewGatewayConsumer(
		natsCon,
		events.WithGWConsumerLoggedInHandler(cache),
		events.WithGWConsumerLoggedOutHandler(cache),
		events.WithGWConsumerCharsUpdatesHandler(cache),
		events.WithGWConsumerGuildCreatedHandler(guildService),
	).Listen()
	if err != nil {
		log.Fatal().Err(err).Msg("can't listen to gateway updates")
	}

	if err = warmupGuildCache(context.Background(), cache, cfg.CharDBConnection); err != nil {
		log.Fatal().Err(err).Msg("can't warmup guilds cache")
	}

	return guildService
}

type guildCacheWarmer interface {
	Warmup(ctx context.Context, realmID uint32) error
}

func warmupGuildCache(ctx context.Context, cache guildCacheWarmer, realmConnections map[uint32]string) error {
	realmIDs := make([]int, 0, len(realmConnections))
	for realmID := range realmConnections {
		realmIDs = append(realmIDs, int(realmID))
	}
	sort.Ints(realmIDs)

	for _, realmID := range realmIDs {
		if err := cache.Warmup(ctx, uint32(realmID)); err != nil {
			return fmt.Errorf("warmup guild cache for realm %d: %w", realmID, err)
		}
	}

	return nil
}

func configureDBConn(db *sql.DB) {
	db.SetMaxIdleConns(5)
	db.SetMaxOpenConns(10)
	db.SetConnMaxLifetime(time.Minute * 4)
	db.SetConnMaxIdleTime(time.Minute * 8)
}
