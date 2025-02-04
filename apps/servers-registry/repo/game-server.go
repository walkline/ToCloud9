package repo

import (
	"context"
	"sort"
)

type DiffData struct {
	Mean         uint32
	Median       uint32
	Percentile95 uint32
	Percentile99 uint32
	Max          uint32
}

type GameServer struct {
	ID              string
	Address         string
	HealthCheckAddr string
	GRPCAddress     string

	// If it's cross-realm then RealmID should be 0.
	RealmID      uint32
	IsCrossRealm bool

	AvailableMaps []uint32

	ActiveConnections uint32
	Diff              DiffData

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

func (g *GameServer) MetricsAddress() string {
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
	Update(ctx context.Context, id string, f func(*GameServer) *GameServer) error
	Remove(ctx context.Context, id string) error
	ListByRealm(ctx context.Context, realmID uint32) ([]GameServer, error)
	ListOfCrossRealms(ctx context.Context) ([]GameServer, error)
	ListAll(ctx context.Context) ([]GameServer, error)
	One(ctx context.Context, id string) (*GameServer, error)
}
