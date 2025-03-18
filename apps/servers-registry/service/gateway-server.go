package service

import (
	"context"
	"sort"

	"github.com/rs/zerolog/log"

	"github.com/walkline/ToCloud9/apps/servers-registry/repo"
	"github.com/walkline/ToCloud9/shared/events"
	"github.com/walkline/ToCloud9/shared/healthandmetrics"
)

type Gateway interface {
	Register(ctx context.Context, server *repo.GatewayServer) (*repo.GatewayServer, error)
	GatewayForRealm(ctx context.Context, realmID uint32) (*repo.GatewayServer, error)
	GatewaysForRealm(ctx context.Context, realmID uint32) ([]repo.GatewayServer, error)
}

type gatewayImpl struct {
	r         repo.GatewayRepo
	checker   healthandmetrics.HealthChecker
	eProducer events.ServerRegistryProducer
	metrics   healthandmetrics.MetricsConsumer
}

func NewGateway(
	ctx context.Context, r repo.GatewayRepo, checker healthandmetrics.HealthChecker,
	metrics healthandmetrics.MetricsConsumer, eProducer events.ServerRegistryProducer,
	supportedRealmIDs []uint32,
) (Gateway, error) {
	service := &gatewayImpl{
		r:         r,
		checker:   checker,
		eProducer: eProducer,
		metrics:   metrics,
	}
	checker.AddFailedObserver(func(object healthandmetrics.HealthCheckObject, err error) {
		if gs, ok := object.(*repo.GatewayServer); ok {
			service.onServerUnhealthy(gs, err)
		}
	})

	metrics.AddObserver(func(observable healthandmetrics.MetricsObservable, read *healthandmetrics.MetricsRead) {
		if gs, ok := observable.(*repo.GatewayServer); ok {
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

func (b *gatewayImpl) Register(ctx context.Context, server *repo.GatewayServer) (*repo.GatewayServer, error) {
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

	err = b.eProducer.GatewayAdded(&events.ServerRegistryEventGWAddedPayload{
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

func (b *gatewayImpl) GatewayForRealm(ctx context.Context, realmID uint32) (*repo.GatewayServer, error) {
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

func (b *gatewayImpl) GatewaysForRealm(ctx context.Context, realmID uint32) ([]repo.GatewayServer, error) {
	return b.r.ListByRealm(ctx, realmID)
}

func (b *gatewayImpl) onServerUnhealthy(server *repo.GatewayServer, err error) {
	log.Warn().
		Err(err).
		Str("healthCheckAddress", server.HealthCheckAddr).
		Msg("Gateway unhealthy! Removing...")

	err = b.r.Remove(context.TODO(), server.HealthCheckAddr)
	if err != nil {
		log.Error().Err(err).Msg("can't remove server")
	}

	err = b.metrics.RemoveMetricsObservable(server)
	if err != nil {
		log.Error().Err(err).Msg("can't remove gateway from metrics consumer")
	}

	err = b.eProducer.GatewayRemovedUnhealthy(&events.ServerRegistryEventGWRemovedUnhealthyPayload{
		ID:              server.ID,
		Address:         server.Address,
		HealthCheckAddr: server.HealthCheckAddr,
		RealmID:         server.RealmID,
	})
	if err != nil {
		log.Error().Err(err).Msg("can't produce unhealthy gateway event")
	}
}

func (b *gatewayImpl) onMetricsUpdate(server *repo.GatewayServer, m *healthandmetrics.MetricsRead) {
	err := b.r.Update(context.Background(), server.ID, func(s repo.GatewayServer) repo.GatewayServer {
		s.ActiveConnections = m.ActiveConnections
		return s
	})
	if err != nil {
		log.Error().Err(err).Msg("can't update metrics for gateway")
	}
}
