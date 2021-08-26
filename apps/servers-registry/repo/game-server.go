package repo

import (
	"context"
	"sort"
)

type GameServer struct {
	Address         string
	HealthCheckAddr string
	RealmID         uint32
	AvailableMaps   []uint32
}

func (g *GameServer) HealthCheckAddress() string {
	return g.HealthCheckAddr
}

func (g *GameServer) IsMapAvailable(id uint32) bool {
	// all maps available
	if len(g.AvailableMaps) == 0 {
		return true
	}

	i := sort.Search(len(g.AvailableMaps), func(i int) bool { return g.AvailableMaps[i] >= id })
	// item exists
	if i < len(g.AvailableMaps) && g.AvailableMaps[i] == id {
		return true
	}

	return false
}

type GameServerRepo interface {
	Add(context.Context, *GameServer) error
	Remove(ctx context.Context, address string) error
	List(ctx context.Context) ([]GameServer, error)
	ListByRealm(ctx context.Context, realmID uint32) ([]GameServer, error)
}
