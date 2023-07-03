package repo

import (
	"context"
	"sort"
)

type GameServer struct {
	Address         string
	HealthCheckAddr string
	GRPCAddress     string
	RealmID         uint32
	AvailableMaps   []uint32

	// AssignedMapsToHandle list of maps that loadbalancer algorithm assigned for this server.
	AssignedMapsToHandle []uint32
}

func (g *GameServer) HealthCheckAddress() string {
	return g.HealthCheckAddr
}

func (g *GameServer) HasMapInHandleList(id uint32) bool {
	i := sort.Search(len(g.AssignedMapsToHandle), func(i int) bool { return g.AssignedMapsToHandle[i] >= id })
	// item exists
	if i < len(g.AssignedMapsToHandle) && g.AssignedMapsToHandle[i] == id {
		return true
	}

	return false
}

func (g *GameServer) IsAllMapsAvailable() bool {
	return len(g.AvailableMaps) == 0
}

type GameServerRepo interface {
	Upsert(context.Context, *GameServer) error
	Remove(ctx context.Context, address string) error
	ListByRealm(ctx context.Context, realmID uint32) ([]GameServer, error)
}
