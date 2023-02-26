package repo

import (
	"context"
	"fmt"
	"sync"
)

type loadBalancerInMemRepo struct {
	storage map[string]*LoadBalancerServer
	mutex   sync.RWMutex
	counter int
}

func NewLoadBalancerInMemRepo() LoadBalancerRepo {
	return &loadBalancerInMemRepo{
		storage: map[string]*LoadBalancerServer{},
	}
}

func (g *loadBalancerInMemRepo) Add(ctx context.Context, server *LoadBalancerServer) (*LoadBalancerServer, error) {
	g.mutex.Lock()
	defer g.mutex.Unlock()

	g.counter++
	server.ID = fmt.Sprintf("%d", g.counter)
	g.storage[server.Address] = server

	return server, nil
}

func (g *loadBalancerInMemRepo) Update(ctx context.Context, id string, f func(LoadBalancerServer) LoadBalancerServer) error {
	g.mutex.Lock()
	defer g.mutex.Unlock()

	for _, v := range g.storage {
		if v.ID == id {
			newVal := f(*v)
			g.storage[v.Address] = &newVal
			return nil
		}
	}

	return nil
}

func (g *loadBalancerInMemRepo) Remove(ctx context.Context, address string) error {
	g.mutex.Lock()
	defer g.mutex.Unlock()

	delete(g.storage, address)

	return nil
}

func (g *loadBalancerInMemRepo) ListByRealm(ctx context.Context, realmID uint32) ([]LoadBalancerServer, error) {
	g.mutex.RLock()
	defer g.mutex.RUnlock()

	result := []LoadBalancerServer{}
	for _, v := range g.storage {
		if v.RealmID == realmID {
			result = append(result, *v)
		}
	}
	return result, nil
}
