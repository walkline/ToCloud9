package main

import (
	"fmt"
	"net"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"

	"github.com/walkline/ToCloud9/apps/chatserver/config"
	"github.com/walkline/ToCloud9/apps/chatserver/repo"
	"github.com/walkline/ToCloud9/apps/chatserver/server"
	"github.com/walkline/ToCloud9/apps/chatserver/service"
	"github.com/walkline/ToCloud9/gen/chat/pb"
)

func main() {
	cfg, err := config.LoadConfig()
	if err != nil {
		panic(err)
	}

	log.Logger = cfg.Logger()

	// nats setup
	nc, err := nats.Connect(cfg.NatsURL, nats.PingInterval(20*time.Second), nats.MaxPingsOutstanding(5), nats.Timeout(10*time.Second))
	if err != nil {
		log.Fatal().Err(err).Msg("can't connect to the Nats")
	}
	defer nc.Close()

	charRepo := repo.NewCharactersInMemRepo()

	// listeners setup
	charListener := service.NewCharactersListener(charRepo, nc)
	err = charListener.Listen()
	if err != nil {
		log.Fatal().Err(err).Msg("can't start listen to characters events-broadcaster")
	}

	srListener := service.NewServersRegistryListener(charRepo, nc)
	err = srListener.Listen()
	if err != nil {
		log.Fatal().Err(err).Msg("can't start listen to services registry events-broadcaster")
	}

	// grpc setup
	lis, err := net.Listen("tcp4", ":"+cfg.Port)
	if err != nil {
		log.Fatal().Err(err).Msg("can't listen for incoming connections")
	}

	grpcServer := grpc.NewServer()
	pb.RegisterChatServiceServer(
		grpcServer,
		server.NewChatService(charRepo),
	)

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
		err = charListener.Stop()
		if err != nil {
			log.Error().Err(err).Msgf("failed to stop listen to characters update")
		}
		wg.Done()
	}()

	log.Info().Str("address", lis.Addr().String()).Msg("🚀 Chat Service started!")

	if err := grpcServer.Serve(lis); err != nil {
		log.Fatal().Err(err).Msg("couldn't serve")
	}

	wg.Wait()

	log.Info().Msg("👍 Server successfully stopped.")
}
