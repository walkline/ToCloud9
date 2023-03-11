package main

import "C"
import (
	"context"
	"strconv"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/rs/zerolog/log"

	"github.com/walkline/ToCloud9/game-server/libsidecar/config"
	"github.com/walkline/ToCloud9/gen/servers-registry/pb"
	"github.com/walkline/ToCloud9/shared/healthandmetrics"
)

const (
	libVer = "0.0.1"
)

func initLib() (*config.Config, healthandmetrics.Server, ShutdownFunc) {
	cfg, err := config.LoadConfig()
	if err != nil {
		panic(err)
	}

	log := cfg.Logger()

	nc, err := nats.Connect(
		cfg.NatsURL,
		nats.PingInterval(20*time.Second),
		nats.MaxPingsOutstanding(5),
		nats.Timeout(10*time.Second),
	)
	if err != nil {
		log.Fatal().Err(err).Msg("can't connect to the Nats")
	}

	healthCheckServer := healthandmetrics.NewServer(cfg.HealthCheckPort, false)
	go healthCheckServer.ListenAndServe()

	natsConsumer := SetupEventsListener(nc, log)

	srvRegConn := SetupServersRegistryConnection(cfg)

	guidConn := SetupGuidServiceConnection(cfg)

	grpcListener, grpcServer := SetupGRPCService(cfg)
	go func() {
		if err := grpcServer.Serve(grpcListener); err != nil {
			log.Fatal().Err(err).Msg("can't serve grpc server")
		}
	}()

	return cfg, healthCheckServer, func() {
		log.Info().Msg("🧨 Attempting graceful shutdown sidecar...")

		ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
		defer cancel()

		if err = healthCheckServer.Shutdown(ctx); err != nil {
			log.Fatal().Err(err).Msg("failed to shutdown healthcheck server")
		}

		grpcServer.GracefulStop()

		if err = natsConsumer.Stop(); err != nil {
			log.Fatal().Err(err).Msg("failed to close nats consumer")
		}

		nc.Close()

		if err = srvRegConn.Close(); err != nil {
			log.Fatal().Err(err).Msg("failed to close servers registry connection")
		}

		if err = guidConn.Close(); err != nil {
			log.Fatal().Err(err).Msg("failed to close guid service connection")
		}

		log.Info().Msg("👍 Sidecar successfully stopped.")
	}
}

func main() {
	// used for debug
	//registryClient.RegisterGameServer(context.TODO(), &pb.RegisterGameServerRequest{
	//	Api:           libVer,
	//	GamePort:      1,
	//	HealthPort:    8900,
	//	RealmID:       1,
	//	AvailableMaps: "",
	//})
	//time.Sleep(time.Second*10)
}

type ShutdownFunc func()

var shutdownFunc ShutdownFunc

// TC9InitLib inits lib by starting services like grpc and healthcheck.
// Adds game server to the servers registry that will make this server visible for game load balancer.
//export TC9InitLib
func TC9InitLib(port uint16, realmID uint32, availableMaps *C.char) {
	cfg, healthCheckServer, shutdown := initLib()
	shutdownFunc = shutdown

	SetupGuidProviders(realmID, cfg)

	healthPort, err := strconv.Atoi(healthCheckServer.Port())
	if err != nil {
		panic(err)
	}

	_, err = registryClient.RegisterGameServer(context.TODO(), &pb.RegisterGameServerRequest{
		Api:               libVer,
		GamePort:          uint32(port),
		HealthPort:        uint32(healthPort),
		RealmID:           realmID,
		AvailableMaps:     C.GoString(availableMaps),
		PreferredHostName: cfg.PreferredHostname,
	})
	if err != nil {
		log.Fatal().Err(err).Msg("couldn't register game server")
	}
}

// TC9GracefulShutdown gracefully stops all running services.
//export TC9GracefulShutdown
func TC9GracefulShutdown() {
	shutdownFunc()
}
