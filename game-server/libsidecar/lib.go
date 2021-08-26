package main

import (
	"C"
	"context"
	"strconv"

	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"

	"github.com/walkline/ToCloud9/game-server/libsidecar/config"
	"github.com/walkline/ToCloud9/gen/servers-registry/pb"
	"github.com/walkline/ToCloud9/shared/healthandmetrics"
)

const (
	libVer = "0.0.1"
)

var registryClient pb.ServersRegistryServiceClient
var healthCheckServer healthandmetrics.Server

var cfg *config.Config

func init() {
	var err error
	cfg, err = config.LoadConfig()
	if err != nil {
		panic(err)
	}

	conn, err := grpc.Dial(cfg.ServersRegistryServiceAddress, grpc.WithInsecure())
	if err != nil {
		log.Fatal().Err(err).Msg("can't connect to the registry server")
	}

	registryClient = pb.NewServersRegistryServiceClient(conn)

	healthCheckServer = healthandmetrics.NewServer(cfg.HealthCheckPort, false)
	go healthCheckServer.ListenAndServe()
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

//export AddToRegistry
func AddToRegistry(port uint16, realmID uint32, availableMaps *C.char) {
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
