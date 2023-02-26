package main

import (
	"database/sql"
	"net"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/nats-io/nats.go"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"

	"github.com/walkline/ToCloud9/apps/charserver/config"
	"github.com/walkline/ToCloud9/apps/charserver/repo"
	"github.com/walkline/ToCloud9/apps/charserver/server"
	"github.com/walkline/ToCloud9/gen/characters/pb"
	"github.com/walkline/ToCloud9/shared/events"
	shrepo "github.com/walkline/ToCloud9/shared/repo"
)

func main() {
	conf, err := config.LoadConfig()
	if err != nil {
		panic(err)
	}

	log.Logger = conf.Logger()

	// nats setup
	nc, err := nats.Connect(conf.NatsURL, nats.PingInterval(20*time.Second), nats.MaxPingsOutstanding(5), nats.Timeout(10*time.Second))
	if err != nil {
		log.Fatal().Err(err).Msg("can't connect to the Nats")
	}
	defer nc.Close()

	lis, err := net.Listen("tcp4", ":"+conf.Port)
	if err != nil {
		log.Fatal().Err(err).Msg("can't start listening")
	}

	cdb, err := sql.Open("mysql", conf.CharDBConnection)
	if err != nil {
		log.Fatal().Err(err).Msg("can't connect to char db")
	}

	wdb, err := sql.Open("mysql", conf.WorldDBConnection)
	if err != nil {
		log.Fatal().Err(err).Msg("can't connect to world db")
	}

	configureDBConn(cdb)
	configureDBConn(wdb)

	itemsTemplate, err := repo.NewItemsTemplateCache(wdb)
	if err != nil {
		panic(err)
	}

	charDB := shrepo.NewCharactersDB()
	charDB.SetDBForRealm(1, cdb)
	charRepo := repo.NewCharactersMYSQL(charDB)

	onlineCharsRepo := repo.NewCharactersOnlineInMem()
	lbEventsConsumer := events.NewLoadBalancerConsumer(
		nc,
		events.WithLBConsumerLoggedInHandler(onlineCharsRepo),
		events.WithLBConsumerLoggedOutHandler(onlineCharsRepo),
		events.WithLBConsumerCharsUpdatesHandler(onlineCharsRepo),
	)
	err = lbEventsConsumer.Listen()
	if err != nil {
		log.Fatal().Err(err).Msg("can't listen to load balancer updates")
	}
	defer lbEventsConsumer.Stop()

	grpcServer := grpc.NewServer()
	pb.RegisterCharactersServiceServer(grpcServer, server.NewCharServer(charRepo, onlineCharsRepo, onlineCharsRepo, itemsTemplate))

	log.Info().Str("address", lis.Addr().String()).Msg("🚀 Characters Server Started!")

	if err := grpcServer.Serve(lis); err != nil {
		panic(err)
	}
}

func configureDBConn(db *sql.DB) {
	db.SetMaxIdleConns(5)
	db.SetMaxOpenConns(10)
	db.SetConnMaxLifetime(time.Minute * 4)
	db.SetConnMaxIdleTime(time.Minute * 8)
}
