package service

import (
	"context"
	"math/rand"
	"sort"

	"github.com/rs/zerolog/log"

	"github.com/walkline/ToCloud9/apps/servers-registry/repo"
	"github.com/walkline/ToCloud9/shared/healthandmetrics"
)

type GameServer interface {
	Register(ctx context.Context, server *repo.GameServer) error
	AvailableForMapAndRealm(ctx context.Context, mapID uint32, realmID uint32) ([]repo.GameServer, error)
	RandomServerForRealm(ctx context.Context, realmID uint32) (*repo.GameServer, error)
}

type gameServerImpl struct {
	r       repo.GameServerRepo
	checker healthandmetrics.HealthChecker
}

func NewGameServer(r repo.GameServerRepo, checker healthandmetrics.HealthChecker) GameServer {
	service := &gameServerImpl{
		r:       r,
		checker: checker,
	}
	checker.AddFailedObserver(func(object healthandmetrics.HealthCheckObject, err error) {
		if gs, ok := object.(*repo.GameServer); ok {
			service.onServerUnhealthy(gs, err)
		}
	})
	return service
}

func (g *gameServerImpl) Register(ctx context.Context, server *repo.GameServer) error {
	sort.Slice(server.AvailableMaps, func(i, j int) bool {
		return server.AvailableMaps[i] <= server.AvailableMaps[j]
	})

	if err := g.checker.AddHealthCheckObject(server); err != nil {
		return err
	}

	return g.r.Add(ctx, server)
}

func (g *gameServerImpl) AvailableForMapAndRealm(ctx context.Context, mapID uint32, realmID uint32) ([]repo.GameServer, error) {
	servers, err := g.r.ListByRealm(ctx, realmID)
	if err != nil {
		return nil, err
	}

	result := []repo.GameServer{}
	for _, server := range servers {
		if server.IsMapAvailable(mapID) {
			result = append(result, server)
		}
	}

	return result, nil
}

func (g *gameServerImpl) RandomServerForRealm(ctx context.Context, realmID uint32) (*repo.GameServer, error) {
	servers, err := g.r.ListByRealm(ctx, realmID)
	if err != nil {
		return nil, err
	}

	if len(servers) == 0 {
		return nil, nil
	}

	return &servers[rand.Intn(len(servers))], nil
}

func (g *gameServerImpl) onServerUnhealthy(server *repo.GameServer, err error) {
	log.Warn().
		Err(err).
		Str("address", server.Address).
		Msg("Game Server unhealthy! Removing...")

	err = g.r.Remove(context.TODO(), server.Address)
	if err != nil {
		log.Error().Err(err).Msg("can't remove server")
	}
}
