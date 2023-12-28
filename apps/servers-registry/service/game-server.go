package service

import (
	"context"
	"fmt"
	"math/rand"
	"sort"

	"github.com/rs/zerolog/log"

	"github.com/walkline/ToCloud9/apps/servers-registry/mapbalancing"
	"github.com/walkline/ToCloud9/apps/servers-registry/repo"
	"github.com/walkline/ToCloud9/shared/events"
	"github.com/walkline/ToCloud9/shared/healthandmetrics"
)

type GameServer interface {
	Register(ctx context.Context, server *repo.GameServer) error
	AvailableForMapAndRealm(ctx context.Context, mapID uint32, realmID uint32) ([]repo.GameServer, error)
	RandomServerForRealm(ctx context.Context, realmID uint32) (*repo.GameServer, error)
	ListForRealm(ctx context.Context, realmID uint32) ([]repo.GameServer, error)
	MapsLoadedForServer(ctx context.Context, serverID string, maps []uint32) (*repo.GameServer, error)
}

type gameServerImpl struct {
	r           repo.GameServerRepo
	checker     healthandmetrics.HealthChecker
	mapBalancer mapbalancing.MapDistributor
	eProducer   events.ServerRegistryProducer
}

func NewGameServer(
	ctx context.Context,
	r repo.GameServerRepo,
	checker healthandmetrics.HealthChecker,
	mapBalancer mapbalancing.MapDistributor,
	eProducer events.ServerRegistryProducer,
	supportedRealmIDs []uint32,
) (GameServer, error) {
	service := &gameServerImpl{
		r:           r,
		checker:     checker,
		mapBalancer: mapBalancer,
		eProducer:   eProducer,
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

	res, err := g.distributeMapsToServers(ctx, wsList)
	if err != nil {
		return fmt.Errorf("failed to register game server during maps ditribution, err: %w", err)
	}

	for _, gameServer := range res {
		if gameServer.ID == server.ID {
			server.AssignedMapsToHandle = gameServer.AssignedMapsToHandle
			break
		}
	}

	return nil
}

func (g *gameServerImpl) AvailableForMapAndRealm(ctx context.Context, mapID uint32, realmID uint32) ([]repo.GameServer, error) {
	servers, err := g.r.ListByRealm(ctx, realmID)
	if err != nil {
		return nil, err
	}

	result := []repo.GameServer{}
	for _, server := range servers {
		if server.CanHandleMap(mapID) {
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

func (g *gameServerImpl) MapsLoadedForServer(ctx context.Context, serverID string, maps []uint32) (*repo.GameServer, error) {
	server, err := g.r.One(ctx, serverID)
	if err != nil {
		return nil, err
	}

	if server == nil {
		return nil, fmt.Errorf("game server not found")
	}

	newPendingMaps := []uint32{}
	for i := range server.AssignedButPendingMaps {
		hasMap := false
		for j := range maps {
			if server.AssignedButPendingMaps[i] == maps[j] {
				hasMap = true
				break
			}
		}
		if !hasMap {
			newPendingMaps = append(newPendingMaps, server.AssignedButPendingMaps[i])
		}
	}

	server.AssignedButPendingMaps = newPendingMaps

	return server, g.r.Upsert(ctx, server)
}

func (g *gameServerImpl) onServerUnhealthy(server *repo.GameServer, err error) {
	log.Warn().
		Err(err).
		Str("address", server.Address).
		Msg("Game Server unhealthy! Removing...")

	err = g.r.Remove(context.Background(), server.ID)
	if err != nil {
		log.Error().Err(err).Msg("can't remove server")
		return
	}

	wsList, err := g.ListForRealm(context.Background(), server.RealmID)
	if err != nil {
		log.Error().Err(err).Msg("can't list servers")
		return
	}

	_, err = g.distributeMapsToServers(context.Background(), wsList)
	if err != nil {
		log.Error().Err(err).Msg("couldn't distribute maps to servers")
		return
	}
}

func (g *gameServerImpl) distributeMapsToServers(ctx context.Context, servers []repo.GameServer) ([]repo.GameServer, error) {
	serversBefore := make([]repo.GameServer, len(servers))
	for i, server := range servers {
		serversBefore[i] = server.Copy()
	}

	distributed := g.mapBalancer.Distribute(servers)

	res := make([]events.GameServer, len(distributed))
	for i := range distributed {
		res[i] = events.GameServer{
			ID:                      distributed[i].ID,
			Address:                 distributed[i].Address,
			RealmID:                 distributed[i].RealmID,
			AvailableMaps:           distributed[i].AvailableMaps,
			NewAssignedMapsToHandle: distributed[i].AssignedMapsToHandle,
		}

		for _, server := range serversBefore {
			if server.ID == distributed[i].ID {
				res[i].OldAssignedMapsToHandle = server.AssignedMapsToHandle
				break
			}
		}
	}

	for i := range distributed {
		// Mark new maps as pending.
		for _, server := range res {
			if server.ID == distributed[i].ID {
				// No need to have confirmation for assignment on startup.
				if len(server.OldAssignedMapsToHandle) > 0 {
					distributed[i].AssignedButPendingMaps = server.OnlyNewMaps()
				}
				break
			}
		}

		if err := g.r.Upsert(ctx, &distributed[i]); err != nil {
			return nil, err
		}
	}

	err := g.eProducer.GSMapsReassigned(&events.ServerRegistryEventGSMapsReassignedPayload{
		Servers: res,
	})
	if err != nil {
		return nil, fmt.Errorf("can't send event for maps reaasigned, err %w", err)
	}

	return distributed, nil
}
