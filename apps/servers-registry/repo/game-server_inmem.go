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

func (g *gameServerInMemRepo) Upsert(ctx context.Context, server *GameServer) error {
	g.mutex.Lock()
	defer g.mutex.Unlock()

	if server.ID == "" {
		server.ID = server.Address
	}

	g.storage = append(g.storage, *server)

	return nil
}

func (g *gameServerInMemRepo) Update(ctx context.Context, id string, f func(*GameServer) *GameServer) error {
	g.mutex.Lock()
	defer g.mutex.Unlock()

	for i := range g.storage {
		if g.storage[i].ID == id {
			g.storage[i] = *f(&g.storage[i])
			return nil
		}
	}

	return nil
}

func (g *gameServerInMemRepo) Remove(ctx context.Context, id string) error {
	g.mutex.Lock()
	defer g.mutex.Unlock()

	for i := range g.storage {
		if g.storage[i].ID == id {
			g.storage = append(g.storage[:i], g.storage[i+1:]...)
			return nil
		}
	}

	return nil
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

func (g *gameServerInMemRepo) ListOfCrossRealms(ctx context.Context) ([]GameServer, error) {
	g.mutex.RLock()
	defer g.mutex.RUnlock()

	result := []GameServer{}
	for i := range g.storage {
		if g.storage[i].IsCrossRealm == true {
			result = append(result, g.storage[i])
		}
	}
	return result, nil
}

func (g *gameServerInMemRepo) ListAll(ctx context.Context) ([]GameServer, error) {
	g.mutex.RLock()
	defer g.mutex.RUnlock()

	result := make([]GameServer, 0, len(g.storage))
	for i := range g.storage {
		result = append(result, g.storage[i])

	}
	return result, nil
}

func (g *gameServerInMemRepo) One(ctx context.Context, id string) (*GameServer, error) {
	g.mutex.RLock()
	defer g.mutex.RUnlock()

	for i := range g.storage {
		if g.storage[i].ID == id {
			return &g.storage[i], nil
		}
	}
	return nil, nil
}
