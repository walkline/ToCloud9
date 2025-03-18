package repo

import "context"

type GatewayServer struct {
	ID                string
	Address           string
	HealthCheckAddr   string
	RealmID           uint32
	ActiveConnections int
}

func (g *GatewayServer) HealthCheckAddress() string {
	return g.HealthCheckAddr
}

func (g *GatewayServer) MetricsAddress() string {
	return g.HealthCheckAddr
}

type GatewayRepo interface {
	Add(context.Context, *GatewayServer) (*GatewayServer, error)
	Update(ctx context.Context, id string, f func(GatewayServer) GatewayServer) error
	Remove(ctx context.Context, healthCheckAddress string) error
	ListByRealm(ctx context.Context, realmID uint32) ([]GatewayServer, error)
}
