package server

import (
	"context"
	"time"

	"github.com/rs/zerolog"

	"github.com/walkline/ToCloud9/gen/servers-registry/pb"
)

// serversRegistryDebugLoggerMiddleware middleware that adds debug logs for pb.ServersRegistryServiceServer.
type serversRegistryDebugLoggerMiddleware struct {
	pb.UnimplementedServersRegistryServiceServer
	realService pb.ServersRegistryServiceServer
	logger      zerolog.Logger
}

// NewServersRegistryDebugLoggerMiddleware returns middleware for pb.ServersRegistryServiceServer that logs requests for debug.
func NewServersRegistryDebugLoggerMiddleware(realService pb.ServersRegistryServiceServer, logger zerolog.Logger) pb.ServersRegistryServiceServer {
	return &serversRegistryDebugLoggerMiddleware{
		realService: realService,
		logger:      logger,
	}
}

func (s *serversRegistryDebugLoggerMiddleware) RegisterGameServer(ctx context.Context, request *pb.RegisterGameServerRequest) (*pb.RegisterGameServerResponse, error) {
	// Logs already inside.
	return s.realService.RegisterGameServer(ctx, request)
}

func (s *serversRegistryDebugLoggerMiddleware) AvailableGameServersForMapAndRealm(ctx context.Context, request *pb.AvailableGameServersForMapAndRealmRequest) (resp *pb.AvailableGameServersForMapAndRealmResponse, err error) {
	defer func(t time.Time) {
		event := s.logger.Debug().
			Uint32("mapID", request.MapID).
			Str("timeTook", time.Since(t).String())

		if resp != nil {
			event = event.Interface("servers", resp.GameServers)
		}

		event.Msg("Handled available game servers")
	}(time.Now())

	resp, err = s.realService.AvailableGameServersForMapAndRealm(ctx, request)
	return
}

func (s *serversRegistryDebugLoggerMiddleware) RandomGameServerForRealm(ctx context.Context, request *pb.RandomGameServerForRealmRequest) (resp *pb.RandomGameServerForRealmResponse, err error) {
	defer func(t time.Time) {
		s.logger.Debug().
			Interface("servers", resp.GameServer).
			Str("timeTook", time.Since(t).String()).
			Msg("Handled random game servers")
	}(time.Now())

	resp, err = s.realService.RandomGameServerForRealm(ctx, request)
	return
}

func (s *serversRegistryDebugLoggerMiddleware) ListGameServersForRealm(ctx context.Context, request *pb.ListGameServersForRealmRequest) (resp *pb.ListGameServersResponse, err error) {
	defer func(t time.Time) {
		s.logger.Debug().
			Uint32("realmID", request.RealmID).
			Str("timeTook", time.Since(t).String()).
			Msg("Handled list game servers for realm")
	}(time.Now())

	resp, err = s.realService.ListGameServersForRealm(ctx, request)
	return
}
func (s *serversRegistryDebugLoggerMiddleware) ListAllGameServers(ctx context.Context, request *pb.ListAllGameServersRequest) (resp *pb.ListGameServersResponse, err error) {
	defer func(t time.Time) {
		s.logger.Debug().
			Str("timeTook", time.Since(t).String()).
			Msg("Handled list all game servers")
	}(time.Now())

	resp, err = s.realService.ListAllGameServers(ctx, request)
	return

}
func (s *serversRegistryDebugLoggerMiddleware) GameServerMapsLoaded(ctx context.Context, request *pb.GameServerMapsLoadedRequest) (resp *pb.GameServerMapsLoadedResponse, err error) {
	defer func(t time.Time) {
		s.logger.Debug().
			Str("timeTook", time.Since(t).String()).
			Msg("Handled GameServerMapsLoaded")
	}(time.Now())

	resp, err = s.realService.GameServerMapsLoaded(ctx, request)
	return
}

func (s *serversRegistryDebugLoggerMiddleware) RegisterLoadBalancer(ctx context.Context, request *pb.RegisterLoadBalancerRequest) (*pb.RegisterLoadBalancerResponse, error) {
	// Logs already inside.
	return s.realService.RegisterLoadBalancer(ctx, request)
}

func (s *serversRegistryDebugLoggerMiddleware) LoadBalancerForRealms(ctx context.Context, request *pb.LoadBalancerForRealmsRequest) (resp *pb.LoadBalancerForRealmsResponse, err error) {
	defer func(t time.Time) {
		s.logger.Debug().
			Interface("servers", resp.LoadBalancers).
			Str("timeTook", time.Since(t).String()).
			Msg("Handled load balancers for realm")
	}(time.Now())

	resp, err = s.realService.LoadBalancerForRealms(ctx, request)
	return
}

func (s *serversRegistryDebugLoggerMiddleware) ListLoadBalancersForRealm(ctx context.Context, request *pb.ListLoadBalancersForRealmRequest) (resp *pb.ListLoadBalancersForRealmResponse, err error) {
	defer func(t time.Time) {
		s.logger.Debug().
			Str("timeTook", time.Since(t).String()).
			Msg("Handled list balancers for realm")
	}(time.Now())

	resp, err = s.realService.ListLoadBalancersForRealm(ctx, request)
	return
}
