package main

import "C"
import (
	"net"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"

	"github.com/walkline/ToCloud9/game-server/libsidecar/config"
	"github.com/walkline/ToCloud9/game-server/libsidecar/grpcapi"
	"github.com/walkline/ToCloud9/game-server/libsidecar/queue"
	"github.com/walkline/ToCloud9/gen/worldserver/pb"
)

var grpcReadRequestsQueue = queue.NewHandlersFIFOQueue()
var grpcWriteRequestsQueue = queue.NewHandlersFIFOQueue()

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
			},
			time.Second*5,
			grpcReadRequestsQueue,
			grpcWriteRequestsQueue,
		),
	)

	return lis, grpcServer
}

// TC9ProcessGRPCRequests calls all grpc handlers in queue.
//
//export TC9ProcessGRPCRequests
func TC9ProcessGRPCRequests() {
	// TODO: make this configurable.
	const readGoroutineCount = 4

	// Handle read operations.
	// Read operation is safe to process in parallel.
	wg := sync.WaitGroup{}
	wg.Add(readGoroutineCount)
	for i := 0; i < readGoroutineCount; i++ {
		go func() {
			defer wg.Done()

			handler := grpcReadRequestsQueue.Pop()
			for handler != nil {
				handler.Handle()
				handler = grpcReadRequestsQueue.Pop()
			}
		}()
	}

	wg.Wait()

	// Handle write operations.
	// Since TC is not tread-safe for write operations, we can have only 1 goroutine to process.
	handler := grpcWriteRequestsQueue.Pop()
	for handler != nil {
		handler.Handle()
		handler = grpcWriteRequestsQueue.Pop()
	}
}
