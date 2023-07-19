package grpcapi

import (
	"errors"
	"time"

	"github.com/walkline/ToCloud9/game-server/libsidecar/queue"
	"github.com/walkline/ToCloud9/gen/worldserver/pb"
)

var ErrTimeout = errors.New("request timeouted")

var LibVer string

type RequestQueue struct {
}

type WorldServerGRPCAPI struct {
	bindings   CppBindings
	timeout    time.Duration
	readQueue  queue.HandlersQueue
	writeQueue queue.HandlersQueue
}

func NewWorldServerGRPCAPI(bindings CppBindings, timeout time.Duration, readQueue, writeQueue queue.HandlersQueue) pb.WorldServerServiceServer {
	return &WorldServerGRPCAPI{
		bindings:   bindings,
		timeout:    timeout,
		readQueue:  readQueue,
		writeQueue: writeQueue,
	}
}
