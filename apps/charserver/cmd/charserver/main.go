package main

import (
	"database/sql"
	"net"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/nats-io/nats.go"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"

	"github.com/walkline/ToCloud9/apps/charserver"
	"github.com/walkline/ToCloud9/apps/charserver/config"
	"github.com/walkline/ToCloud9/apps/charserver/repo"
	"github.com/walkline/ToCloud9/apps/charserver/server"
	"github.com/walkline/ToCloud9/apps/charserver/service"
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
	nc, err := nats.Connect(
		conf.NatsURL,
		nats.PingInterval(20*time.Second),
		nats.MaxPingsOutstanding(5),
		nats.Timeout(10*time.Second),
		nats.Name("charserver"),
	)
	if err != nil {
		log.Fatal().Err(err).Msg("can't connect to the Nats")
	}
	defer nc.Close()

	lis, err := net.Listen("tcp4", ":"+conf.Port)
	if err != nil {
		log.Fatal().Err(err).Msg("can't start listening")
	}

	charDB := shrepo.NewCharactersDB()
	for realmID, connStr := range conf.CharDBConnection {
		cdb, err := sql.Open("mysql", connStr)
		if err != nil {
			log.Fatal().Err(err).Uint32("realmID", realmID).Msg("can't connect to char db")
		}
		configureDBConn(cdb)
		charDB.SetDBForRealm(realmID, cdb)
	}

	wdb, err := sql.Open("mysql", conf.WorldDBConnection)
	if err != nil {
		log.Fatal().Err(err).Msg("can't connect to world db")
	}

	configureDBConn(wdb)

	itemsTemplate, err := repo.NewItemsTemplateCache(wdb)
	if err != nil {
		panic(err)
	}

	charRepo := repo.NewCharactersMYSQL(charDB)

	onlineCharsRepo := repo.NewCharactersOnlineInMem()
	gwEventsConsumer := events.NewGatewayConsumer(
		nc,
		events.WithGWConsumerLoggedInHandler(onlineCharsRepo),
		events.WithGWConsumerLoggedOutHandler(onlineCharsRepo),
		events.WithGWConsumerCharsUpdatesHandler(onlineCharsRepo),
	)
	err = gwEventsConsumer.Listen()
	if err != nil {
		log.Fatal().Err(err).Msg("can't listen to gateway updates")
	}
	defer gwEventsConsumer.Stop()

	srHandler := service.NewServersRegistryListener(onlineCharsRepo, events.NewCharactersServiceProducerNatsJSON(nc, charserver.Ver), nc)
	err = srHandler.Listen()
	if err != nil {
		log.Fatal().Err(err).Msg("can't listen to servers registry updates")
	}
	defer srHandler.Stop()

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
