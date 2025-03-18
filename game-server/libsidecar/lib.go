package main

/*
#include <stdint.h>
*/
import "C"
import (
	"context"
	"strconv"
	"time"
	"unsafe"

	"github.com/nats-io/nats.go"
	"github.com/rs/zerolog/log"

	"github.com/walkline/ToCloud9/game-server/libsidecar/config"
	"github.com/walkline/ToCloud9/gen/servers-registry/pb"
	"github.com/walkline/ToCloud9/shared/healthandmetrics"
)

const (
	libVer = "0.0.1"
	matchmakingSupportedVer
)

var (
	RealmID      uint32
	IsCrossRealm bool
)

func initLib(realmID uint32) (*config.Config, healthandmetrics.Server, ShutdownFunc) {
	ctx, cancel := context.WithCancel(context.Background())

	RealmID = realmID

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
		nats.Name("gameserver"),
	)
	if err != nil {
		log.Fatal().Err(err).Msg("can't connect to the Nats")
	}

	healthandmetrics.EnableActiveConnectionsMetrics()
	healthandmetrics.EnableDelayMetrics()

	healthCheckServer := healthandmetrics.NewServer(cfg.HealthCheckPort, monitoringHttpHandler())
	go healthCheckServer.ListenAndServe()

	natsConsumer := SetupEventsListener(nc, realmID, log)

	srvRegConn := SetupServersRegistryConnection(cfg)

	guidConn := SetupGuidServiceConnection(cfg)

	SetupMatchmakingConnection(ctx, cfg)

	grpcListener, grpcServer := SetupGRPCService(cfg)
	go func() {
		log.Info().Msg("🚀 gRPC Service started...")
		if err := grpcServer.Serve(grpcListener); err != nil {
			log.Fatal().Err(err).Msg("can't serve grpc server")
		}
	}()

	// TODO: replace closing part with context

	return cfg, healthCheckServer, func() {
		log.Info().Msg("🧨 Attempting graceful shutdown sidecar...")

		cancel()

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

// AssignedGameServerID is ID assigned by servers registry to this game server.
var AssignedGameServerID string

// TC9InitLib inits lib by starting services like grpc and healthcheck.
// Adds game server to the servers registry that will make this server visible for gateway.
//
//export TC9InitLib
func TC9InitLib(port uint16, realmID uint32, isCrossRealm bool, availableMaps *C.char, assignedMaps **C.uint32_t, assignedMapsSize *C.int) {
	IsCrossRealm = isCrossRealm

	cfg, healthCheckServer, shutdown := initLib(realmID)
	shutdownFunc = shutdown

	SetupGuidProviders(realmID, cfg)

	healthPort, err := strconv.Atoi(healthCheckServer.Port())
	if err != nil {
		panic(err)
	}

	grpcPort, err := strconv.Atoi(cfg.GRPCPort)
	if err != nil {
		panic(err)
	}

	res, err := registryClient.RegisterGameServer(context.TODO(), &pb.RegisterGameServerRequest{
		Api:               libVer,
		GamePort:          uint32(port),
		HealthPort:        uint32(healthPort),
		GrpcPort:          uint32(grpcPort),
		RealmID:           realmID,
		IsCrossRealm:      isCrossRealm,
		AvailableMaps:     C.GoString(availableMaps),
		PreferredHostName: cfg.PreferredHostname,
	})
	if err != nil {
		log.Fatal().Err(err).Msg("couldn't register game server")
	}

	AssignedGameServerID = res.Id

	if len(res.AssignedMaps) > 0 {
		*assignedMaps = (*C.uint32_t)(C.malloc(C.size_t(len(res.AssignedMaps)) * C.size_t(unsafe.Sizeof(C.uint32_t(0)))))
		pItr := (*C.uint32_t)(unsafe.Pointer(*assignedMaps))
		for _, assignedMap := range res.AssignedMaps {
			*pItr = C.uint32_t(assignedMap)
			pItr = (*C.uint32_t)(unsafe.Pointer(uintptr(unsafe.Pointer(pItr)) + uintptr(unsafe.Sizeof(C.uint32_t(0)))))
		}
	}

	*assignedMapsSize = C.int(len(res.AssignedMaps))
}

// TC9GracefulShutdown gracefully stops all running services.
//
//export TC9GracefulShutdown
func TC9GracefulShutdown() {
	shutdownFunc()
}

// TC9ProcessGRPCOrHTTPRequests calls all grpc or http handlers in queue.
//
//export TC9ProcessGRPCOrHTTPRequests
func TC9ProcessGRPCOrHTTPRequests() {
	// Parallel read processing disabled, since goroutines setup time is bigger than benefits for the low amount of requests.
	// Can be enabled if read requests increase.

	//// TODO: make this configurable.
	//const readGoroutineCount = 4
	//
	//// Handle read operations.
	//// Read operation is safe to process in parallel.
	//wg := sync.WaitGroup{}
	//wg.Add(readGoroutineCount)
	//for i := 0; i < readGoroutineCount; i++ {
	//	go func() {
	//		defer wg.Done()
	//
	//		handler := readRequestsQueue.Pop()
	//		for handler != nil {
	//			handler.Handle()
	//			handler = readRequestsQueue.Pop()
	//		}
	//	}()
	//}
	//
	//wg.Wait()
	//
	//// Handle write operations.
	//// Since TC is not tread-safe for write operations, we can have only 1 goroutine to process.
	//handler := writeRequestsQueue.Pop()
	//for handler != nil {
	//	handler.Handle()
	//	handler = writeRequestsQueue.Pop()
	//}

	handler := readRequestsQueue.Pop()
	for handler != nil {
		handler.Handle()
		handler = readRequestsQueue.Pop()
	}

	handler = writeRequestsQueue.Pop()
	for handler != nil {
		handler.Handle()
		handler = writeRequestsQueue.Pop()
	}
}

// TC9ReadyToAcceptPlayersFromMaps notifies servers registry that this server
// loaded maps related data and ready to accept players from those maps.
//
//export TC9ReadyToAcceptPlayersFromMaps
func TC9ReadyToAcceptPlayersFromMaps(maps *C.uint32_t, mapsLen C.int) {
	mapsSlice := make([]uint32, int(mapsLen))
	pItr := (*C.uint32_t)(unsafe.Pointer(maps))
	for i := range mapsSlice {
		mapsSlice[i] = uint32(*pItr)
		pItr = (*C.uint32_t)(unsafe.Pointer(uintptr(unsafe.Pointer(pItr)) + uintptr(unsafe.Sizeof(C.uint32_t(0)))))
	}
	go func() {
		_, err := registryClient.GameServerMapsLoaded(context.Background(), &pb.GameServerMapsLoadedRequest{
			Api:          libVer,
			GameServerID: AssignedGameServerID,
			MapsLoaded:   mapsSlice,
		})
		if err != nil {
			log.Err(err).Msg("can't mark maps as loaded failed")
		}
	}()
}
