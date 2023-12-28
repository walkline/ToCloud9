package repo

import (
	"context"
	"sort"
)

type GameServer struct {
	ID              string
	Address         string
	HealthCheckAddr string
	GRPCAddress     string
	RealmID         uint32
	AvailableMaps   []uint32

	// AssignedMapsToHandle list of maps that loadbalancer algorithm assigned for this server.
	AssignedMapsToHandle []uint32

	// AssignedButPendingMaps list of new maps that were assigned,
	// but we are still waiting for confirmation from GameServer,
	// that these maps are loaded and game server is ready to handle them.
	AssignedButPendingMaps []uint32
}

func (g *GameServer) HealthCheckAddress() string {
	return g.HealthCheckAddr
}

func (g *GameServer) CanHandleMap(id uint32) bool {
	i := sort.Search(len(g.AssignedMapsToHandle), func(i int) bool { return g.AssignedMapsToHandle[i] >= id })
	// item exists
	if i < len(g.AssignedMapsToHandle) && g.AssignedMapsToHandle[i] == id {
		for _, pendingMap := range g.AssignedButPendingMaps {
			if pendingMap == id {
				return false
			}
		}
		return true
	}

	return false
}

func (g *GameServer) IsAllMapsAvailable() bool {
	return len(g.AvailableMaps) == 0
}

func (g *GameServer) Copy() GameServer {
	cp := *g
	copy(cp.AvailableMaps, g.AvailableMaps)
	copy(cp.AssignedMapsToHandle, g.AssignedMapsToHandle)
	copy(cp.AssignedButPendingMaps, g.AssignedButPendingMaps)
	return cp
}

type GameServerRepo interface {
	Upsert(context.Context, *GameServer) error
	Remove(ctx context.Context, id string) error
	ListByRealm(ctx context.Context, realmID uint32) ([]GameServer, error)
	One(ctx context.Context, id string) (*GameServer, error)
}
