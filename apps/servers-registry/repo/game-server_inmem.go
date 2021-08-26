package repo

import (
	"context"
	"sync"
)

type gameServerInMemRepo struct {
	storage []GameServer
	mutex   sync.RWMutex
}

func NewGameServerInMemRepo() GameServerRepo {
	return &gameServerInMemRepo{}
}

func (g *gameServerInMemRepo) Add(ctx context.Context, server *GameServer) error {
	g.mutex.Lock()
	defer g.mutex.Unlock()

	g.storage = append(g.storage, *server)

	return nil
}

func (g *gameServerInMemRepo) Remove(ctx context.Context, address string) error {
	g.mutex.Lock()
	defer g.mutex.Unlock()

	for i := range g.storage {
		if g.storage[i].Address == address {
			g.storage = append(g.storage[:i], g.storage[i+1:]...)
			return nil
		}
	}

	return nil
}

func (g *gameServerInMemRepo) List(ctx context.Context) ([]GameServer, error) {
	g.mutex.RLock()
	defer g.mutex.RUnlock()

	return g.storage, nil
}

func (g *gameServerInMemRepo) ListByRealm(ctx context.Context, realmID uint32) ([]GameServer, error) {
	g.mutex.RLock()
	defer g.mutex.RUnlock()

	result := []GameServer{}
	for i := range g.storage {
		if g.storage[i].RealmID == realmID {
			result = append(result, g.storage[i])
		}
	}
	return result, nil
}
