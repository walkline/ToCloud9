package main

import (
	"database/sql"
	"net"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"

	"github.com/walkline/ToCloud9/apps/authserver/config"
	"github.com/walkline/ToCloud9/apps/authserver/repo"
	"github.com/walkline/ToCloud9/apps/authserver/service"
	"github.com/walkline/ToCloud9/apps/authserver/session"
	pbServ "github.com/walkline/ToCloud9/gen/servers-registry/pb"
	sharedRepo "github.com/walkline/ToCloud9/shared/repo"
)

func main() {
	conf, err := config.LoadConfig()
	if err != nil {
		panic(err)
	}

	log.Logger = conf.Logger()

	authDB, err := sql.Open("mysql", conf.AuthDBConnection)
	if err != nil {
		log.Fatal().Err(err).Msg("can't connect to auth db")
	}
	defer authDB.Close()

	configureDBConn(authDB)

	stmtsBuilder := repo.StatementsBuilderForSchema(sharedRepo.ParseSchemaType(conf.DBSchemaType))

	authRepo, err := repo.NewAccountMySQLRepo(authDB, stmtsBuilder, sharedRepo.ParseSchemaType(conf.DBSchemaType))
	if err != nil {
		log.Fatal().Err(err).Msg("can't create auth repo")
	}

	realmRepo, err := repo.NewRealmMySQLRepo(authDB, stmtsBuilder)
	if err != nil {
		log.Fatal().Err(err).Msg("can't create realm repo")
	}

	serversRegistry := servRegistryService(conf)

	realmService := service.NewRealmService(realmRepo, serversRegistry)

	l, err := net.Listen("tcp4", "0.0.0.0:"+conf.Port)
	if err != nil {
		log.Fatal().Err(err).Msg("can't start listening")
	}
	defer l.Close()

	log.Info().
		Str("address", l.Addr().String()).
		Msg("🚀 Auth Server started!")

	for {
		conn, err := l.Accept()
		if err != nil {
			log.Fatal().Err(err).Msg("can't accept connection")
		}

		s := session.NewAuthSession(conn, authRepo, realmService)
		go func() {
			// blocks until connection close
			s.ListenAndProcess()
		}()
	}
}

func servRegistryService(cfg *config.Config) pbServ.ServersRegistryServiceClient {
	conn, err := grpc.Dial(cfg.ServersRegistryServiceAddress, grpc.WithInsecure())
	if err != nil {
		log.Fatal().Err(err).Msg("can't connect to servers registry service")
	}

	return pbServ.NewServersRegistryServiceClient(conn)
}

func configureDBConn(db *sql.DB) {
	db.SetMaxIdleConns(5)
	db.SetMaxOpenConns(10)
	db.SetConnMaxLifetime(time.Minute * 4)
	db.SetConnMaxIdleTime(time.Minute * 8)
}
