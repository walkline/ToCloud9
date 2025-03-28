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

	"github.com/walkline/ToCloud9/apps/groupserver"
	"github.com/walkline/ToCloud9/apps/groupserver/config"
	"github.com/walkline/ToCloud9/apps/groupserver/repo"
	"github.com/walkline/ToCloud9/apps/groupserver/server"
	"github.com/walkline/ToCloud9/apps/groupserver/service"
	pbChar "github.com/walkline/ToCloud9/gen/characters/pb"
	"github.com/walkline/ToCloud9/gen/group/pb"
	"github.com/walkline/ToCloud9/shared/events"
	shrepo "github.com/walkline/ToCloud9/shared/repo"
)

func init() {
	// set service identifier with almost random number
	groupserver.ServiceID = strconv.Itoa(time.Now().Nanosecond())
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
		nats.Name("groupserver"),
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
	groupServer := server.NewGroupServer(createGroupService(cfg, nc))
	if cfg.LogLevel == zerolog.DebugLevel {
		groupServer = server.NewGroupsDebugLoggerMiddleware(groupServer, log.Logger)
	}
	pb.RegisterGroupServiceServer(grpcServer, groupServer)

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

	log.Info().Str("address", lis.Addr().String()).Msg("🚀 Group Service started!")

	if err := grpcServer.Serve(lis); err != nil {
		log.Fatal().Err(err).Msg("couldn't serve")
	}

	wg.Wait()

	log.Info().Msg("👍 Server successfully stopped.")
}

func createGroupService(cfg *config.Config, natsCon *nats.Conn) service.GroupsService {
	charDB := shrepo.NewCharactersDB()
	for realmID, connStr := range cfg.CharDBConnection {
		cdb, err := sql.Open("mysql", connStr)
		if err != nil {
			log.Fatal().Err(err).Uint32("realmID", realmID).Msg("can't connect to char db")
		}
		configureDBConn(cdb)
		charDB.SetDBForRealm(realmID, cdb)
	}

	groupsRepo := repo.NewMysqlGroupsRepo(charDB)

	cache := service.NewInMemGroupsCache(groupsRepo)
	err := events.NewGatewayConsumer(
		natsCon,
		events.WithGWConsumerLoggedInHandler(cache),
		events.WithGWConsumerLoggedOutHandler(cache),
	).Listen()
	if err != nil {
		log.Fatal().Err(err).Msg("can't listen to gateway updates")
	}

	err = cache.Warmup(context.Background(), 1)
	if err != nil {
		log.Fatal().Err(err).Msg("can't warmup groups cache")
	}

	charClient := charService(cfg)

	s := service.NewGroupsService(cache, charClient, events.NewGroupServiceProducerNatsJSON(natsCon, groupserver.Ver))

	// TODO: combine this with consumer for cache
	err = events.NewGatewayConsumer(
		natsCon,
		events.WithGWConsumerLoggedInHandler(s),
		events.WithGWConsumerLoggedOutHandler(s),
	).Listen()
	if err != nil {
		log.Fatal().Err(err).Msg("can't listen to gateway updates")
	}

	return s
}

func charService(cnf *config.Config) pbChar.CharactersServiceClient {
	conn, err := grpc.Dial(cnf.CharServiceAddress, grpc.WithInsecure())
	if err != nil {
		log.Fatal().Err(err).Msg("can't connect to characters service")
	}

	return pbChar.NewCharactersServiceClient(conn)
}

func configureDBConn(db *sql.DB) {
	db.SetMaxIdleConns(5)
	db.SetMaxOpenConns(10)
	db.SetConnMaxLifetime(time.Minute * 4)
	db.SetConnMaxIdleTime(time.Minute * 8)
}
