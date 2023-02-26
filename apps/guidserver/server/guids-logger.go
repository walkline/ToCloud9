package server

import (
	"context"
	"time"

	"github.com/rs/zerolog"

	"github.com/walkline/ToCloud9/gen/guid/pb"
)

// guidServerLoggerMiddleware is guild server logger.
type guidServerLoggerMiddleware struct {
	realService pb.GuidServiceServer
	logger      zerolog.Logger
}

// NewGuidsDebugLoggerMiddleware returns middleware for pb.GuidServiceServer that logs requests for debug.
func NewGuidsDebugLoggerMiddleware(realService pb.GuidServiceServer, logger zerolog.Logger) pb.GuidServiceServer {
	return &guidServerLoggerMiddleware{
		realService: realService,
		logger:      logger,
	}
}

// GetGUIDPool returns available GUIDs for given realm and guid type.
func (g *guidServerLoggerMiddleware) GetGUIDPool(ctx context.Context, request *pb.GetGUIDPoolRequest) (res *pb.GetGUIDPoolRequestResponse, err error) {
	defer func(t time.Time) {
		g.logger.Debug().
			Uint32("type", uint32(request.GuidType)).
			Uint32("realmID", request.RealmID).
			Uint64("desiredPoolSize", request.DesiredPoolSize).
			Err(err).
			Msgf("Handled GetGUIDPool for %v.", time.Since(t))
	}(time.Now())

	res, err = g.realService.GetGUIDPool(ctx, request)
	return

}
