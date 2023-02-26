package service

import (
	"context"
	"math/rand"
	"sort"

	"github.com/rs/zerolog/log"

	"github.com/walkline/ToCloud9/apps/servers-registry/mapbalancing"
	"github.com/walkline/ToCloud9/apps/servers-registry/repo"
	"github.com/walkline/ToCloud9/shared/healthandmetrics"
)

type GameServer interface {
	Register(ctx context.Context, server *repo.GameServer) error
	AvailableForMapAndRealm(ctx context.Context, mapID uint32, realmID uint32) ([]repo.GameServer, error)
	RandomServerForRealm(ctx context.Context, realmID uint32) (*repo.GameServer, error)
	ListForRealm(ctx context.Context, realmID uint32) ([]repo.GameServer, error)
}

type gameServerImpl struct {
	r           repo.GameServerRepo
	checker     healthandmetrics.HealthChecker
	mapBalancer mapbalancing.MapDistributor
}

func NewGameServer(ctx context.Context, r repo.GameServerRepo, checker healthandmetrics.HealthChecker, mapBalancer mapbalancing.MapDistributor, supportedRealmIDs []uint32) (GameServer, error) {
	service := &gameServerImpl{
		r:           r,
		checker:     checker,
		mapBalancer: mapBalancer,
	}

	checker.AddFailedObserver(func(object healthandmetrics.HealthCheckObject, err error) {
		if gs, ok := object.(*repo.GameServer); ok {
			service.onServerUnhealthy(gs, err)
		}
	})

	for _, id := range supportedRealmIDs {
		servers, err := r.ListByRealm(ctx, id)
		if err != nil {
			return nil, err
		}

		for i := range servers {
			if err = checker.AddHealthCheckObject(&servers[i]); err != nil {
				return nil, err
			}
		}
	}

	return service, nil
}

func (g *gameServerImpl) Register(ctx context.Context, server *repo.GameServer) error {
	sort.Slice(server.AvailableMaps, func(i, j int) bool {
		return server.AvailableMaps[i] <= server.AvailableMaps[j]
	})

	if err := g.checker.AddHealthCheckObject(server); err != nil {
		return err
	}

	if err := g.r.Upsert(ctx, server); err != nil {
		return err
	}

	wsList, err := g.ListForRealm(ctx, server.RealmID)
	if err != nil {
		return err
	}

	return g.distributeMapsToServers(ctx, wsList)
}

func (g *gameServerImpl) AvailableForMapAndRealm(ctx context.Context, mapID uint32, realmID uint32) ([]repo.GameServer, error) {
	servers, err := g.r.ListByRealm(ctx, realmID)
	if err != nil {
		return nil, err
	}

	result := []repo.GameServer{}
	for _, server := range servers {
		if server.HasMapInHandleList(mapID) {
			result = append(result, server)
		}
	}

	return append(result), nil
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

func (g *gameServerImpl) ListForRealm(ctx context.Context, realmID uint32) ([]repo.GameServer, error) {
	servers, err := g.r.ListByRealm(ctx, realmID)
	if err != nil {
		return nil, err
	}

	return servers, nil
}

func (g *gameServerImpl) onServerUnhealthy(server *repo.GameServer, err error) {
	log.Warn().
		Err(err).
		Str("address", server.Address).
		Msg("Game Server unhealthy! Removing...")

	err = g.r.Remove(context.Background(), server.Address)
	if err != nil {
		log.Error().Err(err).Msg("can't remove server")
		return
	}

	wsList, err := g.ListForRealm(context.Background(), server.RealmID)
	if err != nil {
		log.Error().Err(err).Msg("can't list servers")
		return
	}

	err = g.distributeMapsToServers(context.Background(), wsList)
	if err != nil {
		log.Error().Err(err).Msg("couldn't distribute maps to servers")
		return
	}
}

func (g *gameServerImpl) distributeMapsToServers(ctx context.Context, servers []repo.GameServer) error {
	distributed := g.mapBalancer.Distribute(servers)
	for i := range distributed {
		if err := g.r.Upsert(ctx, &distributed[i]); err != nil {
			return err
		}
	}

	return nil
}
