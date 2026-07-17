package main

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	nats "github.com/nats-io/nats.go"
	redis "github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"

	"github.com/walkline/ToCloud9/apps/servers-registry/config"
	"github.com/walkline/ToCloud9/apps/servers-registry/mapbalancing/binpack"
	"github.com/walkline/ToCloud9/apps/servers-registry/repo"
	"github.com/walkline/ToCloud9/apps/servers-registry/server"
	"github.com/walkline/ToCloud9/apps/servers-registry/service"
	"github.com/walkline/ToCloud9/gen/servers-registry/pb"
	"github.com/walkline/ToCloud9/shared/events"
	"github.com/walkline/ToCloud9/shared/healthandmetrics"
)

func main() {
	mainContext, cancel := context.WithCancel(context.Background())
	defer cancel()

	conf, err := config.LoadConfig()
	if err != nil {
		panic(err)
	}

	log.Logger = conf.Logger()

	lis, err := net.Listen("tcp4", ":"+conf.Port)
	if err != nil {
		log.Fatal().Err(err).Msg("can't listen for incoming connections")
	}

	nc, err := nats.Connect(
		conf.NatsURL,
		nats.PingInterval(20*time.Second),
		nats.MaxPingsOutstanding(5),
		nats.Timeout(10*time.Second),
		nats.Name("servers-registry"),
	)
	if err != nil {
		log.Fatal().Err(err).Msg("can't connect to nats")
	}
	defer nc.Close()

	opt, err := redis.ParseURL(conf.RedisConnection)
	if err != nil {
		log.Fatal().Err(err).Msg("can't connect to the redis")
	}

	rdb := redis.NewClient(opt)
	pingRes := rdb.Ping(context.Background())
	if pingRes.Err() != nil {
		log.Fatal().Err(err).Msg("can't connect to redis")
	}
	defer rdb.Close()

	healthChecker := healthandmetrics.NewHealthChecker(time.Second*4, 4, healthandmetrics.NewHttpHealthCheckProcessor(time.Second*15))
	go healthChecker.Start()

	metricsConsumer := healthandmetrics.NewMetricsConsumer(time.Second*5, 3, healthandmetrics.NewHttpPrometheusMetricsReader(time.Second))
	go metricsConsumer.Start()

	supportedRealms := conf.RealmsID
	gameServersService, err := service.NewGameServer(
		mainContext,
		repo.NewGameServerRedisRepo(rdb),
		healthChecker,
		metricsConsumer,
		binpack.NewBinPackBalancer(binpack.DefaultMapsWeight), // TODO: implement providing custom maps weight list.
		events.NewServerRegistryProducerNatsJSON(nc, "0.0.1"),
		supportedRealms,
	)
	if err != nil {
		log.Fatal().Err(err).Msg("can't create game server service")
	}

	gatewayService, err := service.NewGateway(
		mainContext,
		repo.NewGatewayRedisRepo(rdb),
		healthChecker,
		metricsConsumer,
		events.NewServerRegistryProducerNatsJSON(nc, "0.0.1"),
		[]uint32{1},
	)
	if err != nil {
		log.Fatal().Err(err).Msg("can't create gateway service")
	}

	scopes := make([]service.LayerScope, len(conf.Layering.Scopes))
	mapLayers := make(map[uint32]uint32, len(conf.Layering.Maps))
	for _, item := range conf.Layering.Maps {
		mapLayers[item.MapID] = item.Layers
	}
	for _, spec := range conf.Layering.MapSpecs {
		parts := strings.SplitN(spec, ":", 2)
		if len(parts) != 2 {
			log.Fatal().Str("mapLayer", spec).Msg("invalid LAYER_MAPS entry, expected mapID:layers")
		}
		mapID, mapErr := strconv.ParseUint(parts[0], 10, 32)
		layers, layerErr := strconv.ParseUint(parts[1], 10, 32)
		if mapErr != nil || layerErr != nil || layers == 0 {
			log.Fatal().Str("mapLayer", spec).Msg("invalid LAYER_MAPS entry")
		}
		mapLayers[uint32(mapID)] = uint32(layers)
	}
	for i, scope := range conf.Layering.Scopes {
		scopes[i] = service.LayerScope{Name: scope.Name, MapIDs: scope.MapIDs, ZoneIDs: scope.ZoneIDs, MaxPopulation: scope.MaxPopulation}
	}
	if len(scopes) == 0 && (len(conf.Layering.ScopeMapIDs) > 0 || len(conf.Layering.ScopeZoneIDs) > 0) {
		scopes = append(scopes, service.LayerScope{Name: "environment-scope", MapIDs: conf.Layering.ScopeMapIDs, ZoneIDs: conf.Layering.ScopeZoneIDs, MaxPopulation: conf.Layering.ScopeMaxPopulation})
	}
	layerService := service.NewLayer(gameServersService, service.LayerConfig{
		Enabled:                 conf.Layering.Enabled,
		MaxPopulation:           conf.Layering.MaxPopulation,
		TargetPopulationPercent: conf.Layering.TargetPopulationPct,
		OverflowMarginPercent:   conf.Layering.OverflowMarginPct,
		SwitchCooldown:          time.Duration(conf.Layering.SwitchCooldownSeconds) * time.Second,
		MaxSwitchesPerHour:      conf.Layering.MaxSwitchesPerHour,
		ReconcileInterval:       time.Duration(conf.Layering.ReconcileIntervalSecs) * time.Second,
		RealmIDs:                supportedRealms, Scopes: scopes, MapLayers: mapLayers,
	})
	for _, realmID := range supportedRealms {
		if err := gameServersService.UpdateMapLayerConfiguration(mainContext, realmID, mapLayers); err != nil {
			log.Fatal().Err(err).Uint32("realmID", realmID).Msg("can't apply map layer configuration")
		}
	}
	if conf.Layering.Enabled {
		go layerService.Run(mainContext)
	}
	registryService := server.NewServersRegistry(gameServersService, gatewayService, layerService)
	if conf.LogLevel == zerolog.DebugLevel {
		registryService = server.NewServersRegistryDebugLoggerMiddleware(registryService, log.Logger)
	}

	grpcServer := grpc.NewServer()
	pb.RegisterServersRegistryServiceServer(
		grpcServer,
		registryService,
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
