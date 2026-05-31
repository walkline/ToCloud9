package main

import (
	"github.com/rs/zerolog/log"
	"github.com/walkline/ToCloud9/game-server/libsidecar/config"
	groupPb "github.com/walkline/ToCloud9/gen/group/pb"
	"google.golang.org/grpc"
)

var groupServiceClient groupPb.GroupServiceClient

func SetupGroupServiceConnection(cfg *config.Config) *grpc.ClientConn {
	log.Info().Str("address", cfg.GroupServiceAddress).Msg("connecting to group service")

	conn, err := grpc.Dial(cfg.GroupServiceAddress, grpc.WithInsecure())
	if err != nil {
		log.Fatal().Err(err).Msg("can't connect to the group server")
	}

	groupServiceClient = groupPb.NewGroupServiceClient(conn)
	return conn
}
