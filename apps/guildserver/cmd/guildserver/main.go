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
	cdb, err := sql.Open("mysql", cfg.CharDBConnection)
	if err != nil {
		log.Fatal().Err(err).Msg("can't connect to char db")
	}

	configureDBConn(cdb)

	charDB := shrepo.NewCharactersDB()
	charDB.SetDBForRealm(1, cdb)
	guildsRepo, err := repo.NewGuildsMySQLRepo(charDB)
	if err != nil {
		log.Fatal().Err(err).Msg("can't create guilds repo")
	}

	cache := service.NewGuildsInMemCache(guildsRepo)
	err = events.NewLoadBalancerConsumer(
		natsCon,
		events.WithLBConsumerLoggedInHandler(cache),
		events.WithLBConsumerLoggedOutHandler(cache),
		events.WithLBConsumerCharsUpdatesHandler(cache),
	).Listen()
	if err != nil {
		log.Fatal().Err(err).Msg("can't listen to load balancer updates")
	}

	err = cache.Warmup(context.Background(), 1)
	if err != nil {
		log.Fatal().Err(err).Msg("can't warmup guilds cache")
	}

	return service.NewGuildService(cache, events.NewGuildServiceProducerNatsJSON(natsCon, guildserver.Ver))
}

func configureDBConn(db *sql.DB) {
	db.SetMaxIdleConns(5)
	db.SetMaxOpenConns(10)
	db.SetConnMaxLifetime(time.Minute * 4)
	db.SetConnMaxIdleTime(time.Minute * 8)
}
