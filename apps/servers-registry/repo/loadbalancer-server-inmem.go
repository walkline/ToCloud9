package repo

import (
	"context"
	"fmt"
	"sync"
)

type gatewayInMemRepo struct {
	storage map[string]*GatewayServer
	mutex   sync.RWMutex
	counter int
}

func NewGatewayInMemRepo() GatewayRepo {
	return &gatewayInMemRepo{
		storage: map[string]*GatewayServer{},
	}
}

func (g *gatewayInMemRepo) Add(ctx context.Context, server *GatewayServer) (*GatewayServer, error) {
	g.mutex.Lock()
	defer g.mutex.Unlock()

	g.counter++
	server.ID = fmt.Sprintf("%d", g.counter)
	g.storage[server.HealthCheckAddr] = server

	return server, nil
}

func (g *gatewayInMemRepo) Update(ctx context.Context, id string, f func(GatewayServer) GatewayServer) error {
	g.mutex.Lock()
	defer g.mutex.Unlock()

	for _, v := range g.storage {
		if v.ID == id {
			newVal := f(*v)
			g.storage[v.HealthCheckAddr] = &newVal
			return nil
		}
	}

	return nil
}

func (g *gatewayInMemRepo) Remove(ctx context.Context, healthCheckAddress string) error {
	g.mutex.Lock()
	defer g.mutex.Unlock()

	delete(g.storage, healthCheckAddress)

	return nil
}

func (g *gatewayInMemRepo) ListByRealm(ctx context.Context, realmID uint32) ([]GatewayServer, error) {
	g.mutex.RLock()
	defer g.mutex.RUnlock()

	result := []GatewayServer{}
	for _, v := range g.storage {
		if v.RealmID == realmID {
			result = append(result, *v)
		}
	}
	return result, nil
}
