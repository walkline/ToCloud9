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

	"github.com/walkline/ToCloud9/apps/servers-registry/config"
	"github.com/walkline/ToCloud9/apps/servers-registry/repo"
	"github.com/walkline/ToCloud9/apps/servers-registry/server"
	"github.com/walkline/ToCloud9/apps/servers-registry/service"
	"github.com/walkline/ToCloud9/gen/servers-registry/pb"
	"github.com/walkline/ToCloud9/shared/events"
	"github.com/walkline/ToCloud9/shared/healthandmetrics"
)

func main() {
	conf, err := config.LoadConfig()
	if err != nil {
		panic(err)
	}

	log.Logger = conf.Logger()

	lis, err := net.Listen("tcp4", ":"+conf.Port)
	if err != nil {
		log.Fatal().Err(err).Msg("can't listen for incoming connections")
	}

	nc, err := nats.Connect(conf.NatsURL, nats.PingInterval(20*time.Second), nats.MaxPingsOutstanding(5), nats.Timeout(10*time.Second))
	if err != nil {
		log.Fatal().Err(err).Msg("can't connect to nats")
	}
	defer nc.Close()

	healthChecker := healthandmetrics.NewHealthChecker(time.Second*4, 4, healthandmetrics.NewHttpHealthCheckProcessor(time.Second*2))
	go healthChecker.Start()

	metricsConsumer := healthandmetrics.NewMetricsConsumer(time.Second*5, 3, healthandmetrics.NewHttpPrometheusMetricsReader(time.Second))
	go metricsConsumer.Start()

	grpcServer := grpc.NewServer()
	pb.RegisterServersRegistryServiceServer(
		grpcServer,
		server.NewServersRegistry(
			service.NewGameServer(repo.NewGameServerInMemRepo(), healthChecker),
			service.NewLoadBalancer(repo.NewLoadBalancerInMemRepo(), healthChecker, metricsConsumer, events.NewServerRegistryProducerNatsJSON(nc, "0.0.1")),
		),
	)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		sig := <-sigCh
		fmt.Println("")
		log.Info().Msgf("🧨 Got signal %v, attempting graceful shutdown...", sig)
		grpcServer.GracefulStop()
		wg.Done()
	}()

	log.Info().Str("address", lis.Addr().String()).Msg("🚀 Servers Registry started!")

	if err := grpcServer.Serve(lis); err != nil {
		log.Fatal().Err(err).Msg("couldn't serve")
	}

	wg.Wait()

	log.Info().Msg("👍 Server successfully stopped.")
}
