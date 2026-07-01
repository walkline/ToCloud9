package main

import (
	"fmt"
	"net"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"

	"github.com/walkline/ToCloud9/apps/pathfinding/config"
	"github.com/walkline/ToCloud9/apps/pathfinding/server"
	"github.com/walkline/ToCloud9/apps/pathfinding/service"
	"github.com/walkline/ToCloud9/gen/pathfinding/pb"
)

func main() {
	cfg, err := config.LoadConfig()
	if err != nil {
		panic(err)
	}

	log.Logger = cfg.Logger()

	svc := service.NewPathFindingService(cfg.MmapsDir, cfg.MapsDir)

	lis, err := net.Listen("tcp4", ":"+cfg.Port)
	if err != nil {
		log.Fatal().Err(err).Msg("can't start listening")
	}

	grpcServer := grpc.NewServer()
	pb.RegisterPathfindingServiceServer(grpcServer, server.NewPathfindingServer(svc))

	// Graceful shutdown.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		sig := <-sigCh
		fmt.Println("")
		log.Info().Msgf("Got signal %v, attempting graceful shutdown...", sig)
		grpcServer.GracefulStop()
		wg.Done()
	}()

	log.Info().Str("address", lis.Addr().String()).Msg("Pathfinding Service started!")

	if err := grpcServer.Serve(lis); err != nil {
		log.Fatal().Err(err).Msg("couldn't serve")
	}

	wg.Wait()

	log.Info().Msg("Server successfully stopped.")
}