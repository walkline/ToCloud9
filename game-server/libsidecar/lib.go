package main

import "C"
import (
	"context"
	"strconv"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"

	"github.com/walkline/ToCloud9/game-server/libsidecar/config"
	"github.com/walkline/ToCloud9/game-server/libsidecar/consumer"
	"github.com/walkline/ToCloud9/game-server/libsidecar/guids"
	guidPB "github.com/walkline/ToCloud9/gen/guid/pb"
	"github.com/walkline/ToCloud9/gen/servers-registry/pb"
	"github.com/walkline/ToCloud9/shared/healthandmetrics"
)

const (
	libVer = "0.0.1"
)

var registryClient pb.ServersRegistryServiceClient
var healthCheckServer healthandmetrics.Server

var eventsHandlersQueue consumer.HandlersQueue
var natsConsumer consumer.Consumer

var cfg *config.Config

func init() {
	var err error
	cfg, err = config.LoadConfig()
	if err != nil {
		panic(err)
	}

	log := cfg.Logger()

	// nats setup
	nc, err := nats.Connect(
		cfg.NatsURL,
		nats.PingInterval(20*time.Second),
		nats.MaxPingsOutstanding(5),
		nats.Timeout(10*time.Second),
	)
	if err != nil {
		log.Fatal().Err(err).Msg("can't connect to the Nats")
	}
	//defer nc.Close()

	conn, err := grpc.Dial(cfg.ServersRegistryServiceAddress, grpc.WithInsecure())
	if err != nil {
		log.Fatal().Err(err).Msg("can't connect to the registry server")
	}

	registryClient = pb.NewServersRegistryServiceClient(conn)

	healthCheckServer = healthandmetrics.NewServer(cfg.HealthCheckPort, false)
	go healthCheckServer.ListenAndServe()

	eventsHandlersQueue = consumer.NewHandlersFIFOQueue()
	natsConsumer = consumer.NewNatsEventsConsumer(nc, NewGuildHandlerFabric(log), eventsHandlersQueue)
	err = natsConsumer.Start()
	if err != nil {
		log.Fatal().Err(err).Msg("can't start nats consumer")
	}

	SetupGuidServiceConnection()
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

var guidServiceClient guidPB.GuidServiceClient

func SetupGuidServiceConnection() {
	conn, err := grpc.Dial(cfg.GuidProviderServiceAddress, grpc.WithInsecure())
	if err != nil {
		log.Fatal().Err(err).Msg("can't connect to the guid provider service")
	}

	guidServiceClient = guidPB.NewGuidServiceClient(conn)
}

var charactersGuidsIterator guids.GuidProvider
var itemsGuidsIterator guids.GuidProvider

func SetupGuidProviders(realmID uint32) {
	const pctToTriggerUpdate float32 = 65

	var err error
	charactersGuidsIterator, err = guids.NewThreadUnsafeGuidProvider(
		context.Background(),
		guids.NewCharactersGRPCDiapasonsProvider(guidServiceClient, realmID, uint64(cfg.CharacterGuidsBufferSize)),
		pctToTriggerUpdate,
	)
	if err != nil {
		log.Fatal().Err(err).Msg("can't create characters guid provider")
	}

	itemsGuidsIterator, err = guids.NewThreadUnsafeGuidProvider(
		context.Background(),
		guids.NewItemsGRPCDiapasonsProvider(guidServiceClient, realmID, uint64(cfg.ItemGuidsBufferSize)),
		pctToTriggerUpdate,
	)
	if err != nil {
		log.Fatal().Err(err).Msg("can't create items guid provider")
	}
}

// TC9AddToRegistry adds game server to the servers registry that will make this server visible for game load balancer.
//export TC9AddToRegistry
func TC9AddToRegistry(port uint16, realmID uint32, availableMaps *C.char) {
	SetupGuidProviders(realmID)

	healthPort, err := strconv.Atoi(healthCheckServer.Port())
	if err != nil {
		panic(err)
	}

	registryClient.RegisterGameServer(context.TODO(), &pb.RegisterGameServerRequest{
		Api:               libVer,
		GamePort:          uint32(port),
		HealthPort:        uint32(healthPort),
		RealmID:           realmID,
		AvailableMaps:     C.GoString(availableMaps),
		PreferredHostName: cfg.PreferredHostname,
	})
}

// TC9ProcessEventsHooks calls all events hooks.
//export TC9ProcessEventsHooks
func TC9ProcessEventsHooks() {
	handler := eventsHandlersQueue.Pop()
	for handler != nil {
		handler.Handle()
		handler = eventsHandlersQueue.Pop()
	}
}

// TC9GetNextAvailableCharacterGuid returns next available characters GUID. Thread unsafe.
//export TC9GetNextAvailableCharacterGuid
func TC9GetNextAvailableCharacterGuid() uint64 {
	return charactersGuidsIterator.Next()
}

// TC9GetNextAvailableItemGuid returns next available item GUID. Thread unsafe.
//export TC9GetNextAvailableItemGuid
func TC9GetNextAvailableItemGuid() uint64 {
	return itemsGuidsIterator.Next()
}
