package main

import (
	"net"

	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/walkline/ToCloud9/apps/layer-coordinator/config"
	coordinatorServer "github.com/walkline/ToCloud9/apps/layer-coordinator/server"
	pbCoordinator "github.com/walkline/ToCloud9/gen/layer-coordinator/pb"
	pbRegistry "github.com/walkline/ToCloud9/gen/servers-registry/pb"
)

func main() {
	conf, err := config.LoadConfig()
	if err != nil {
		log.Fatal().Err(err).Msg("can't load config")
	}
	log.Logger = conf.Logger()
	registryConn, err := grpc.Dial(conf.ServersRegistryAddress, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatal().Err(err).Msg("can't connect to servers registry")
	}
	defer registryConn.Close()
	lis, err := net.Listen("tcp4", ":"+conf.Port)
	if err != nil {
		log.Fatal().Err(err).Msg("can't listen")
	}
	grpcServer := grpc.NewServer()
	pbCoordinator.RegisterLayerCoordinatorServiceServer(grpcServer, coordinatorServer.New(pbRegistry.NewServersRegistryServiceClient(registryConn), coordinatorServer.Options{
		MaxPopulation: conf.MaxPopulation, TargetPopulationPercent: conf.TargetPopulationPct,
		OverflowMarginPercent: conf.OverflowMarginPct, SwitchCooldownSeconds: conf.SwitchCooldownSeconds,
		MaxSwitchesPerHour: conf.MaxSwitchesPerHour,
	}))
	log.Info().Str("address", lis.Addr().String()).Msg("Layer Coordinator started")
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatal().Err(err).Msg("couldn't serve")
	}
}
