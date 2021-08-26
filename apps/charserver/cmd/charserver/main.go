package main

import (
	"database/sql"
	"net"

	_ "github.com/go-sql-driver/mysql"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"

	"github.com/walkline/ToCloud9/apps/charserver/config"
	"github.com/walkline/ToCloud9/apps/charserver/repo"
	"github.com/walkline/ToCloud9/apps/charserver/server"
	"github.com/walkline/ToCloud9/gen/characters/pb"
)

func main() {
	conf, err := config.LoadConfig()
	if err != nil {
		panic(err)
	}

	log.Logger = conf.Logger()

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

	itemsTemplate, err := repo.NewItemsTemplateCache(wdb)
	if err != nil {
		panic(err)
	}

	charDB := repo.NewCharactersDB()
	charDB.SetDBForRealm(1, cdb)
	charRepo := repo.NewCharactersMYSQL(charDB)

	grpcServer := grpc.NewServer()
	pb.RegisterCharactersServiceServer(grpcServer, server.NewCharServer(charRepo, itemsTemplate))

	log.Info().Str("address", lis.Addr().String()).Msg("🚀 Characters Server Started!")

	if err := grpcServer.Serve(lis); err != nil {
		panic(err)
	}
}
