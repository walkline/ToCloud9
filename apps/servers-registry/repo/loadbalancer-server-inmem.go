package repo

import (
	"context"
	"fmt"
	"sync"
)

type loadBalancerInMemRepo struct {
	storage []LoadBalancerServer
	mutex   sync.RWMutex
	counter int
}

func NewLoadBalancerInMemRepo() LoadBalancerRepo {
	return &loadBalancerInMemRepo{}
}

func (g *loadBalancerInMemRepo) Add(ctx context.Context, server *LoadBalancerServer) (*LoadBalancerServer, error) {
	g.mutex.Lock()
	defer g.mutex.Unlock()

	g.counter++
	server.ID = fmt.Sprintf("%d", g.counter)
	g.storage = append(g.storage, *server)

	return server, nil
}

func (g *loadBalancerInMemRepo) Update(ctx context.Context, id string, f func(LoadBalancerServer) LoadBalancerServer) error {
	g.mutex.Lock()
	defer g.mutex.Unlock()

	for i := range g.storage {
		if g.storage[i].ID == id {
			g.storage[i] = f(g.storage[i])
			return nil
		}
	}

	return nil
}

func (g *loadBalancerInMemRepo) Remove(ctx context.Context, address string) error {
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

func (g *loadBalancerInMemRepo) List(ctx context.Context) ([]LoadBalancerServer, error) {
	g.mutex.RLock()
	defer g.mutex.RUnlock()

	return g.storage, nil
}

func (g *loadBalancerInMemRepo) ListByRealm(ctx context.Context, realmID uint32) ([]LoadBalancerServer, error) {
	g.mutex.RLock()
	defer g.mutex.RUnlock()

	result := []LoadBalancerServer{}
	for i := range g.storage {
		if g.storage[i].RealmID == realmID {
			result = append(result, g.storage[i])
		}
	}
	return result, nil
}
