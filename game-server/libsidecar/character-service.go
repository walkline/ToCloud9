package main

import (
	"github.com/rs/zerolog/log"
	"github.com/walkline/ToCloud9/game-server/libsidecar/config"
	charPb "github.com/walkline/ToCloud9/gen/characters/pb"
	"google.golang.org/grpc"
)

var characterServiceClient charPb.CharactersServiceClient

func SetupCharacterServiceConnection(cfg *config.Config) *grpc.ClientConn {
	log.Info().Str("address", cfg.CharacterServiceAddress).Msg("connecting to character service")

	conn, err := grpc.Dial(cfg.CharacterServiceAddress, grpc.WithInsecure())
	if err != nil {
		log.Fatal().Err(err).Msg("can't connect to the character server")
	}

	characterServiceClient = charPb.NewCharactersServiceClient(conn)
	return conn
}
