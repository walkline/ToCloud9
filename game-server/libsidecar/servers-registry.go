package main

import (
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"

	"github.com/walkline/ToCloud9/game-server/libsidecar/config"
	"github.com/walkline/ToCloud9/gen/servers-registry/pb"
)

var registryClient pb.ServersRegistryServiceClient

func SetupServersRegistryConnection(cfg *config.Config) *grpc.ClientConn {
	conn, err := grpc.Dial(cfg.ServersRegistryServiceAddress, grpc.WithInsecure())
	if err != nil {
		log.Fatal().Err(err).Msg("can't connect to the registry server")
	}

	registryClient = pb.NewServersRegistryServiceClient(conn)

	return conn
}
