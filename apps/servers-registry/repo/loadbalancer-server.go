package repo

import "context"

type LoadBalancerServer struct {
	ID                string
	Address           string
	HealthCheckAddr   string
	RealmID           uint32
	ActiveConnections int
}

func (g *LoadBalancerServer) HealthCheckAddress() string {
	return g.HealthCheckAddr
}

func (g *LoadBalancerServer) MetricsAddress() string {
	return g.HealthCheckAddr
}

type LoadBalancerRepo interface {
	Add(context.Context, *LoadBalancerServer) (*LoadBalancerServer, error)
	Update(ctx context.Context, id string, f func(LoadBalancerServer) LoadBalancerServer) error
	Remove(ctx context.Context, address string) error
	ListByRealm(ctx context.Context, realmID uint32) ([]LoadBalancerServer, error)
}
