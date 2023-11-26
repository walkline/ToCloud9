package service

import (
	"context"
	"sort"

	"github.com/rs/zerolog/log"

	"github.com/walkline/ToCloud9/apps/servers-registry/repo"
	"github.com/walkline/ToCloud9/shared/events"
	"github.com/walkline/ToCloud9/shared/healthandmetrics"
)

type LoadBalancer interface {
	Register(ctx context.Context, server *repo.LoadBalancerServer) (*repo.LoadBalancerServer, error)
	BalancerForRealm(ctx context.Context, realmID uint32) (*repo.LoadBalancerServer, error)
	ListBalancersForRealm(ctx context.Context, realmID uint32) ([]repo.LoadBalancerServer, error)
}

type loadBalancerImpl struct {
	r         repo.LoadBalancerRepo
	checker   healthandmetrics.HealthChecker
	eProducer events.ServerRegistryProducer
	metrics   healthandmetrics.MetricsConsumer
}

func NewLoadBalancer(
	ctx context.Context, r repo.LoadBalancerRepo, checker healthandmetrics.HealthChecker,
	metrics healthandmetrics.MetricsConsumer, eProducer events.ServerRegistryProducer,
	supportedRealmIDs []uint32,
) (LoadBalancer, error) {
	service := &loadBalancerImpl{
		r:         r,
		checker:   checker,
		eProducer: eProducer,
		metrics:   metrics,
	}
	checker.AddFailedObserver(func(object healthandmetrics.HealthCheckObject, err error) {
		if gs, ok := object.(*repo.LoadBalancerServer); ok {
			service.onServerUnhealthy(gs, err)
		}
	})

	metrics.AddObserver(func(observable healthandmetrics.MetricsObservable, read *healthandmetrics.MetricsRead) {
		if gs, ok := observable.(*repo.LoadBalancerServer); ok {
			service.onMetricsUpdate(gs, read)
		}
	})

	for _, id := range supportedRealmIDs {
		servers, err := r.ListByRealm(ctx, id)
		if err != nil {
			return nil, err
		}

		for i := range servers {
			err = checker.AddHealthCheckObject(&servers[i])
			if err != nil {
				return nil, err
			}

			err = metrics.AddMetricsObservable(&servers[i])
			if err != nil {
				return nil, err
			}
		}
	}

	return service, nil
}

func (b *loadBalancerImpl) Register(ctx context.Context, server *repo.LoadBalancerServer) (*repo.LoadBalancerServer, error) {
	err := b.checker.AddHealthCheckObject(server)
	if err != nil {
		return nil, err
	}

	err = b.metrics.AddMetricsObservable(server)
	if err != nil {
		return nil, err
	}

	server, err = b.r.Add(ctx, server)
	if err != nil {
		return nil, err
	}

	err = b.eProducer.LBAdded(&events.ServerRegistryEventLBAddedPayload{
		ID:              server.ID,
		Address:         server.Address,
		HealthCheckAddr: server.HealthCheckAddr,
		RealmID:         server.RealmID,
	})
	if err != nil {
		log.Error().Err(err).Msg("can't produce unhealthy lb event")
	}

	return server, nil
}

func (b *loadBalancerImpl) BalancerForRealm(ctx context.Context, realmID uint32) (*repo.LoadBalancerServer, error) {
	balancers, err := b.r.ListByRealm(ctx, realmID)
	if err != nil {
		return nil, err
	}

	if len(balancers) == 0 {
		return nil, nil
	}

	sort.Slice(balancers, func(i, j int) bool {
		return balancers[i].ActiveConnections < balancers[j].ActiveConnections
	})

	return &balancers[0], nil
}

func (b *loadBalancerImpl) ListBalancersForRealm(ctx context.Context, realmID uint32) ([]repo.LoadBalancerServer, error) {
	return b.r.ListByRealm(ctx, realmID)
}

func (b *loadBalancerImpl) onServerUnhealthy(server *repo.LoadBalancerServer, err error) {
	log.Warn().
		Err(err).
		Str("healthCheckAddress", server.HealthCheckAddr).
		Msg("Load Balancer unhealthy! Removing...")

	err = b.r.Remove(context.TODO(), server.HealthCheckAddr)
	if err != nil {
		log.Error().Err(err).Msg("can't remove server")
	}

	err = b.metrics.RemoveMetricsObservable(server)
	if err != nil {
		log.Error().Err(err).Msg("can't remove lb from metrics consumer")
	}

	err = b.eProducer.LBRemovedUnhealthy(&events.ServerRegistryEventLBRemovedUnhealthyPayload{
		ID:              server.ID,
		Address:         server.Address,
		HealthCheckAddr: server.HealthCheckAddr,
		RealmID:         server.RealmID,
	})
	if err != nil {
		log.Error().Err(err).Msg("can't produce unhealthy lb event")
	}
}

func (b *loadBalancerImpl) onMetricsUpdate(server *repo.LoadBalancerServer, m *healthandmetrics.MetricsRead) {
	err := b.r.Update(context.Background(), server.ID, func(s repo.LoadBalancerServer) repo.LoadBalancerServer {
		s.ActiveConnections = m.ActiveConnections
		return s
	})
	if err != nil {
		log.Error().Err(err).Msg("can't update metrics for game load balancer")
	}
}
