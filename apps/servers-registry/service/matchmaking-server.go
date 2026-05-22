package service

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/walkline/ToCloud9/shared/events"
	"github.com/walkline/ToCloud9/shared/healthandmetrics"
)

type Matchmaking interface {
	Register(ctx context.Context, server *MatchmakingServer) (*MatchmakingServer, error)
}

type MatchmakingServer struct {
	ID              string
	Address         string
	HealthCheckAddr string
	StartedAtUnixMs int64
}

func (s *MatchmakingServer) HealthCheckAddress() string {
	return s.HealthCheckAddr
}

type matchmakingImpl struct {
	checker   healthandmetrics.HealthChecker
	eProducer events.ServerRegistryProducer

	mu      sync.Mutex
	current *MatchmakingServer
}

func NewMatchmaking(checker healthandmetrics.HealthChecker, eProducer events.ServerRegistryProducer) Matchmaking {
	service := &matchmakingImpl{
		checker:   checker,
		eProducer: eProducer,
	}

	checker.AddFailedObserver(func(object healthandmetrics.HealthCheckObject, err error) {
		if server, ok := object.(*MatchmakingServer); ok {
			service.onServerUnhealthy(server, err)
		}
	})

	return service
}

func (m *matchmakingImpl) Register(ctx context.Context, server *MatchmakingServer) (*MatchmakingServer, error) {
	_ = ctx

	if server == nil {
		return nil, fmt.Errorf("matchmaking server is nil")
	}
	if server.Address == "" {
		return nil, fmt.Errorf("matchmaking server address is empty")
	}
	if server.HealthCheckAddr == "" {
		return nil, fmt.Errorf("matchmaking health check address is empty")
	}
	if server.ID == "" {
		server.ID = server.Address
	}

	m.mu.Lock()

	if m.current != nil && sameMatchmakingInstance(m.current, server) {
		if err := m.checker.AddHealthCheckObject(m.current); err != nil {
			m.mu.Unlock()
			return nil, err
		}
		current := m.current
		m.mu.Unlock()
		return current, nil
	}

	restarted := m.current != nil
	if m.current != nil {
		if err := m.checker.RemoveHealthCheckObject(m.current); err != nil {
			m.mu.Unlock()
			return nil, err
		}
	}

	if err := m.checker.AddHealthCheckObject(server); err != nil {
		m.mu.Unlock()
		return nil, err
	}
	m.current = server
	m.mu.Unlock()

	if restarted {
		m.publishUnhealthy(server, fmt.Errorf("matchmaking service restarted"))
	}

	log.Info().
		Str("address", server.Address).
		Str("healthCheckAddress", server.HealthCheckAddr).
		Int64("startedAtUnixMs", server.StartedAtUnixMs).
		Msg("Registered matchmaking server")

	return server, nil
}

func (m *matchmakingImpl) onServerUnhealthy(server *MatchmakingServer, err error) {
	m.mu.Lock()
	if m.current != server {
		m.mu.Unlock()
		return
	}
	m.current = nil
	m.mu.Unlock()

	if removeErr := m.checker.RemoveHealthCheckObject(server); removeErr != nil {
		log.Error().Err(removeErr).Msg("can't remove matchmaking from health checker")
	}

	m.publishUnhealthy(server, err)
}

func (m *matchmakingImpl) publishUnhealthy(server *MatchmakingServer, err error) {
	log.Warn().
		Err(err).
		Str("address", server.Address).
		Str("healthCheckAddress", server.HealthCheckAddr).
		Msg("Matchmaking service unhealthy")

	if produceErr := m.eProducer.MatchmakingRemovedUnhealthy(&events.ServerRegistryEventMatchmakingRemovedUnhealthyPayload{
		MatchmakingService: events.MatchmakingService{
			Address:          server.Address,
			HealthCheckAddr:  server.HealthCheckAddr,
			ObservedAtUnixMs: time.Now().UnixMilli(),
		},
		Error: err.Error(),
	}); produceErr != nil {
		log.Error().Err(produceErr).Msg("can't produce unhealthy matchmaking event")
	}
}

func sameMatchmakingInstance(a, b *MatchmakingServer) bool {
	return a.Address == b.Address &&
		a.HealthCheckAddr == b.HealthCheckAddr &&
		a.StartedAtUnixMs != 0 &&
		a.StartedAtUnixMs == b.StartedAtUnixMs
}
