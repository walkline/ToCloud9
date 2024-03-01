package main

import "C"
import (
	"net"
	"time"

	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"

	"github.com/walkline/ToCloud9/game-server/libsidecar/config"
	"github.com/walkline/ToCloud9/game-server/libsidecar/grpcapi"
	"github.com/walkline/ToCloud9/game-server/libsidecar/queue"
	"github.com/walkline/ToCloud9/gen/worldserver/pb"
)

var readRequestsQueue = queue.NewHandlersFIFOQueue()
var writeRequestsQueue = queue.NewHandlersFIFOQueue()

func SetupGRPCService(conf *config.Config) (net.Listener, *grpc.Server) {
	grpcapi.LibVer = libVer

	lis, err := net.Listen("tcp4", ":"+conf.GRPCPort)
	if err != nil {
		log.Fatal().Err(err).Msg("can't listen for incoming connections")
	}

	grpcServer := grpc.NewServer()
	pb.RegisterWorldServerServiceServer(
		grpcServer,
		grpcapi.NewWorldServerGRPCAPI(
			grpcapi.CppBindings{
				GetPlayerItemsByGuids:          GetPlayerItemsByGuidHandler,
				RemoveItemsWithGuidsFromPlayer: RemoveItemsWithGuidsFromPlayerHandler,
				AddExistingItemToPlayer:        AddExistingItemToPlayerHandler,
				GetMoneyForPlayer:              GetMoneyForPlayerHandler,
				ModifyMoneyForPlayer:           ModifyMoneyForPlayerHandler,
				CanPlayerInteractWithNPC:       CanPlayerInteractWithNPCAndFlagsHandler,
				CanPlayerInteractWithGO:        CanPlayerInteractWithGOAndTypeHandler,
			},
			time.Second*5,
			readRequestsQueue,
			writeRequestsQueue,
		),
	)

	return lis, grpcServer
}
