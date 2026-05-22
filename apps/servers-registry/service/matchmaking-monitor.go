package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/walkline/ToCloud9/shared/events"
	"github.com/walkline/ToCloud9/shared/healthandmetrics"
)

const (
	defaultMatchmakingHealthCheckInterval = 4 * time.Second
	defaultMatchmakingHealthCheckTimeout  = 15 * time.Second
)

type MatchmakingHealthMonitor struct {
	serviceAddress     string
	healthCheckAddress string
	interval           time.Duration
	client             *http.Client
	eventsProducer     events.ServerRegistryProducer

	lastHealthy         *bool
	lastStartedAtUnixMs int64
}

type matchmakingHealthPayload struct {
	StartedAtUnixMs int64 `json:"startedAtUnixMs"`
}

func NewMatchmakingHealthMonitor(
	serviceAddress string,
	healthCheckAddress string,
	interval time.Duration,
	timeout time.Duration,
	eventsProducer events.ServerRegistryProducer,
) *MatchmakingHealthMonitor {
	if interval <= 0 {
		interval = defaultMatchmakingHealthCheckInterval
	}
	if timeout <= 0 {
		timeout = defaultMatchmakingHealthCheckTimeout
	}

	return &MatchmakingHealthMonitor{
		serviceAddress:     serviceAddress,
		healthCheckAddress: healthCheckAddress,
		interval:           interval,
		client: &http.Client{
			Timeout: timeout,
		},
		eventsProducer: eventsProducer,
	}
}

func (m *MatchmakingHealthMonitor) Start(ctx context.Context) {
	if m.healthCheckAddress == "" {
		return
	}

	m.checkOnce(ctx)

	ticker := time.NewTicker(m.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			m.checkOnce(ctx)
		case <-ctx.Done():
			return
		}
	}
}

func (m *MatchmakingHealthMonitor) checkOnce(ctx context.Context) {
	status, err := m.check(ctx)
	if err != nil {
		if m.lastHealthy == nil || *m.lastHealthy {
			m.publishUnhealthy(err)
		}
		healthy := false
		m.lastHealthy = &healthy
		return
	}

	if m.lastHealthy != nil && !*m.lastHealthy {
		m.publishRecovered()
	} else if m.lastHealthy != nil &&
		*m.lastHealthy &&
		status.StartedAtUnixMs != 0 &&
		m.lastStartedAtUnixMs != 0 &&
		status.StartedAtUnixMs != m.lastStartedAtUnixMs {
		m.publishUnhealthy(fmt.Errorf("matchmaking service restarted"))
	}
	m.lastStartedAtUnixMs = status.StartedAtUnixMs
	healthy := true
	m.lastHealthy = &healthy
}

func (m *MatchmakingHealthMonitor) check(ctx context.Context) (matchmakingHealthPayload, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+m.healthCheckAddress+healthandmetrics.HealthCheckURL, nil)
	if err != nil {
		return matchmakingHealthPayload{}, err
	}

	resp, err := m.client.Do(req)
	if err != nil {
		return matchmakingHealthPayload{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return matchmakingHealthPayload{}, fmt.Errorf("bad status code %d", resp.StatusCode)
	}

	var status matchmakingHealthPayload
	if err := json.NewDecoder(io.LimitReader(resp.Body, 4096)).Decode(&status); err != nil {
		return matchmakingHealthPayload{}, nil
	}

	return status, nil
}

func (m *MatchmakingHealthMonitor) publishUnhealthy(err error) {
	log.Warn().
		Err(err).
		Str("address", m.serviceAddress).
		Str("healthCheckAddress", m.healthCheckAddress).
		Msg("Matchmaking service unhealthy")

	if produceErr := m.eventsProducer.MatchmakingRemovedUnhealthy(&events.ServerRegistryEventMatchmakingRemovedUnhealthyPayload{
		MatchmakingService: m.payload(),
		Error:              err.Error(),
	}); produceErr != nil {
		log.Error().Err(produceErr).Msg("can't produce unhealthy matchmaking event")
	}
}

func (m *MatchmakingHealthMonitor) publishRecovered() {
	log.Info().
		Str("address", m.serviceAddress).
		Str("healthCheckAddress", m.healthCheckAddress).
		Msg("Matchmaking service recovered")

	if err := m.eventsProducer.MatchmakingRecovered(&events.ServerRegistryEventMatchmakingRecoveredPayload{
		MatchmakingService: m.payload(),
	}); err != nil {
		log.Error().Err(err).Msg("can't produce recovered matchmaking event")
	}
}

func (m *MatchmakingHealthMonitor) payload() events.MatchmakingService {
	return events.MatchmakingService{
		Address:          m.serviceAddress,
		HealthCheckAddr:  m.healthCheckAddress,
		ObservedAtUnixMs: time.Now().UnixMilli(),
	}
}
