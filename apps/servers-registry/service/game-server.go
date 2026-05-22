package service

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"net"
	"net/http"
	"sort"

	"github.com/rs/zerolog/log"

	"github.com/walkline/ToCloud9/apps/servers-registry/mapbalancing"
	"github.com/walkline/ToCloud9/apps/servers-registry/repo"
	"github.com/walkline/ToCloud9/shared/events"
	"github.com/walkline/ToCloud9/shared/healthandmetrics"
)

type GameServer interface {
	Register(ctx context.Context, server *repo.GameServer) error
	AvailableForMapAndRealm(ctx context.Context, mapID uint32, realmID uint32, isCrossRealm bool) ([]repo.GameServer, error)
	RandomServerForRealm(ctx context.Context, realmID uint32) (*repo.GameServer, error)
	ListForRealm(ctx context.Context, realmID uint32) ([]repo.GameServer, error)
	ListOfCrossRealms(ctx context.Context) ([]repo.GameServer, error)
	ListAll(ctx context.Context) ([]repo.GameServer, error)
	MapsLoadedForServer(ctx context.Context, serverID string, maps []uint32) (*repo.GameServer, error)
}

type gameServerImpl struct {
	r           repo.GameServerRepo
	checker     healthandmetrics.HealthChecker
	metrics     healthandmetrics.MetricsConsumer
	mapBalancer mapbalancing.MapDistributor
	eProducer   events.ServerRegistryProducer
}

func NewGameServer(
	ctx context.Context,
	r repo.GameServerRepo,
	checker healthandmetrics.HealthChecker,
	metrics healthandmetrics.MetricsConsumer,
	mapBalancer mapbalancing.MapDistributor,
	eProducer events.ServerRegistryProducer,
	supportedRealmIDs []uint32,
) (GameServer, error) {
	service := &gameServerImpl{
		r:           r,
		checker:     checker,
		metrics:     metrics,
		mapBalancer: mapBalancer,
		eProducer:   eProducer,
	}

	checker.AddFailedObserver(func(object healthandmetrics.HealthCheckObject, err error) {
		if gs, ok := object.(*repo.GameServer); ok {
			service.onServerUnhealthy(gs, err)
		}
	})

	checker.AddSuccessObserver(func(object healthandmetrics.HealthCheckObject) {
		if gs, ok := object.(*repo.GameServer); ok {
			service.onServerHealthy(gs)
		}
	})

	metrics.AddObserver(func(observable healthandmetrics.MetricsObservable, read *healthandmetrics.MetricsRead) {
		if gs, ok := observable.(*repo.GameServer); ok {
			service.onMetricsUpdate(gs, read)
		}
	})

	for _, id := range supportedRealmIDs {
		servers, err := r.ListByRealm(ctx, id)
		if err != nil {
			return nil, err
		}

		for i := range servers {
			if err = checker.AddHealthCheckObject(&servers[i]); err != nil {
				return nil, err
			}

			err = metrics.AddMetricsObservable(&servers[i])
			if err != nil {
				return nil, err
			}
		}
	}

	servers, err := r.ListOfCrossRealms(ctx)
	if err != nil {
		return nil, err
	}

	for i := range servers {
		if err = checker.AddHealthCheckObject(&servers[i]); err != nil {
			return nil, err
		}

		err = metrics.AddMetricsObservable(&servers[i])
		if err != nil {
			return nil, err
		}
	}

	return service, nil
}

func (g *gameServerImpl) Register(ctx context.Context, server *repo.GameServer) error {
	sort.Slice(server.AvailableMaps, func(i, j int) bool {
		return server.AvailableMaps[i] < server.AvailableMaps[j]
	})
	server.HealthDegraded = false

	if err := g.checker.AddHealthCheckObject(server); err != nil {
		return err
	}

	if err := g.metrics.AddMetricsObservable(server); err != nil {
		return err
	}

	if err := g.r.Upsert(ctx, server); err != nil {
		return err
	}

	var wsList []repo.GameServer
	var err error

	if server.IsCrossRealm {
		wsList, err = g.ListOfCrossRealms(ctx)
	} else {
		wsList, err = g.ListForRealm(ctx, server.RealmID)
	}

	if err != nil {
		return err
	}

	res, err := g.distributeMapsToServers(ctx, wsList)
	if err != nil {
		return fmt.Errorf("failed to register game server during maps ditribution, err: %w", err)
	}

	for _, gameServer := range res {
		if gameServer.ID == server.ID {
			server.AssignedMapsToHandle = gameServer.AssignedMapsToHandle
			break
		}
	}

	err = g.eProducer.GSAdded(&events.ServerRegistryEventGSAddedPayload{
		GameServer: events.GameServer{
			ID:                      server.ID,
			Address:                 server.Address,
			RealmID:                 server.RealmID,
			IsCrossRealm:            server.IsCrossRealm,
			AvailableMaps:           server.AvailableMaps,
			OldAssignedMapsToHandle: []uint32{},
			NewAssignedMapsToHandle: server.AssignedMapsToHandle,
		},
	})
	if err != nil {
		log.Error().Err(err).Str("serverID", server.ID).Msg("can't produce game server added event")
	}

	return nil
}

func (g *gameServerImpl) AvailableForMapAndRealm(ctx context.Context, mapID uint32, realmID uint32, isCrossRealm bool) ([]repo.GameServer, error) {
	var (
		servers []repo.GameServer
		err     error
	)

	if isCrossRealm {
		servers, err = g.r.ListOfCrossRealms(ctx)
	} else {
		servers, err = g.r.ListByRealm(ctx, realmID)
	}
	if err != nil {
		return nil, err
	}

	admissionServers := gameServersAcceptingNewPlayers(servers)
	hasExplicitMapServer := false
	for _, server := range admissionServers {
		if !server.IsAllMapsAvailable() && containsMapID(server.AvailableMaps, mapID) {
			hasExplicitMapServer = true
			break
		}
	}

	result := []repo.GameServer{}
	for _, server := range admissionServers {
		if hasExplicitMapServer && server.IsAllMapsAvailable() {
			continue
		}

		if server.CanHandleMap(mapID) {
			result = append(result, server)
		}
	}

	if len(result) == 0 && hasDegradedMapOwner(servers, mapID) {
		for _, server := range admissionServers {
			if server.IsAllMapsAvailable() {
				result = append(result, server)
			}
		}
		if len(result) > 0 {
			log.Warn().
				Uint32("mapID", mapID).
				Uint32("realmID", realmID).
				Bool("isCrossRealm", isCrossRealm).
				Int("candidates", len(result)).
				Msg("Using healthy all-map game servers while assigned owner is health-degraded")
		}
	}

	return result, nil
}

func (g *gameServerImpl) RandomServerForRealm(ctx context.Context, realmID uint32) (*repo.GameServer, error) {
	servers, err := g.r.ListByRealm(ctx, realmID)
	if err != nil {
		return nil, err
	}
	servers = gameServersAcceptingNewPlayers(servers)

	if len(servers) == 0 {
		return nil, nil
	}

	return &servers[rand.Intn(len(servers))], nil
}

func (g *gameServerImpl) ListForRealm(ctx context.Context, realmID uint32) ([]repo.GameServer, error) {
	servers, err := g.r.ListByRealm(ctx, realmID)
	if err != nil {
		return nil, err
	}

	return servers, nil
}

func (g *gameServerImpl) ListAll(ctx context.Context) ([]repo.GameServer, error) {
	servers, err := g.r.ListAll(ctx)
	if err != nil {
		return nil, err
	}

	return servers, nil
}

func (g *gameServerImpl) ListOfCrossRealms(ctx context.Context) ([]repo.GameServer, error) {
	servers, err := g.r.ListOfCrossRealms(ctx)
	if err != nil {
		return nil, err
	}

	return servers, nil
}

func (g *gameServerImpl) MapsLoadedForServer(ctx context.Context, serverID string, maps []uint32) (*repo.GameServer, error) {
	server, err := g.r.One(ctx, serverID)
	if err != nil {
		return nil, err
	}

	if server == nil {
		return nil, fmt.Errorf("game server not found")
	}

	pendingBefore := len(server.AssignedButPendingMaps)
	newPendingMaps := []uint32{}
	for i := range server.AssignedButPendingMaps {
		hasMap := false
		for j := range maps {
			if server.AssignedButPendingMaps[i] == maps[j] {
				hasMap = true
				break
			}
		}
		if !hasMap {
			newPendingMaps = append(newPendingMaps, server.AssignedButPendingMaps[i])
		}
	}

	server.AssignedButPendingMaps = newPendingMaps
	log.Info().
		Str("serverID", serverID).
		Str("address", server.Address).
		Str("grpcAddress", server.GRPCAddress).
		Str("healthCheckAddress", server.HealthCheckAddr).
		Uint32("realmID", server.RealmID).
		Bool("isCrossRealm", server.IsCrossRealm).
		Int("loadedMaps", len(maps)).
		Int("pendingBefore", pendingBefore).
		Int("pendingAfter", len(newPendingMaps)).
		Msg("Game server maps marked ready")

	return server, g.r.Upsert(ctx, server)
}

func (g *gameServerImpl) onServerHealthy(server *repo.GameServer) {
	wasDegraded := false
	err := g.r.Update(context.Background(), server.ID, func(s *repo.GameServer) *repo.GameServer {
		wasDegraded = s.HealthDegraded
		s.HealthDegraded = false
		return s
	})
	if err != nil {
		log.Error().
			Err(err).
			Str("address", server.Address).
			Str("healthCheckAddress", server.HealthCheckAddr).
			Str("serverID", server.ID).
			Msg("can't clear degraded game server health state")
		return
	}
	if wasDegraded {
		log.Info().
			Str("address", server.Address).
			Str("healthCheckAddress", server.HealthCheckAddr).
			Str("serverID", server.ID).
			Uint32("realmID", server.RealmID).
			Bool("isCrossRealm", server.IsCrossRealm).
			Msg("Game Server health recovered; accepting new player placement")
	}
}

func (g *gameServerImpl) onServerUnhealthy(server *repo.GameServer, err error) {
	if isDegradedGameServerHealthError(err) {
		updateErr := g.r.Update(context.Background(), server.ID, func(s *repo.GameServer) *repo.GameServer {
			s.HealthDegraded = true
			return s
		})
		if updateErr != nil {
			log.Error().
				Err(updateErr).
				Str("address", server.Address).
				Str("healthCheckAddress", server.HealthCheckAddr).
				Str("serverID", server.ID).
				Msg("can't mark degraded game server as non-admitting")
		}

		log.Warn().
			Err(err).
			Str("address", server.Address).
			Str("healthCheckAddress", server.HealthCheckAddr).
			Str("serverID", server.ID).
			Uint32("realmID", server.RealmID).
			Bool("isCrossRealm", server.IsCrossRealm).
			Msg("Game Server world-loop health degraded; preserving ownership but draining new player placement")
		degraded := server.Copy()
		degraded.HealthDegraded = true
		if addErr := g.checker.AddHealthCheckObject(&degraded); addErr != nil {
			log.Error().Err(addErr).Str("serverID", server.ID).Msg("can't re-add degraded game server to health checker")
		}
		return
	}

	log.Warn().
		Err(err).
		Str("address", server.Address).
		Msg("Game Server unhealthy! Removing...")

	err = g.r.Remove(context.Background(), server.ID)
	if err != nil {
		log.Error().Err(err).Msg("can't remove server")
		return
	}

	err = g.eProducer.GSRemoved(&events.ServerRegistryEventGSRemovedPayload{
		GameServer: events.GameServer{
			ID:                      server.ID,
			Address:                 server.Address,
			RealmID:                 server.RealmID,
			IsCrossRealm:            server.IsCrossRealm,
			AvailableMaps:           server.AvailableMaps,
			OldAssignedMapsToHandle: server.AssignedMapsToHandle,
			NewAssignedMapsToHandle: server.AssignedMapsToHandle,
		},
	})
	if err != nil {
		log.Error().Err(err).Str("serverID", server.ID).Msg("can't produce game server removed event")
	}

	err = g.metrics.RemoveMetricsObservable(server)
	if err != nil {
		log.Error().Err(err).Msg("can't remove gameserver from metrics consumer")
	}

	var wsList []repo.GameServer

	if server.IsCrossRealm {
		wsList, err = g.ListOfCrossRealms(context.Background())
	} else {
		wsList, err = g.ListForRealm(context.Background(), server.RealmID)
	}

	if err != nil {
		log.Error().Err(err).Msg("can't list servers")
		return
	}

	_, err = g.distributeMapsToServers(context.Background(), wsList)
	if err != nil {
		log.Error().Err(err).Msg("couldn't distribute maps to servers")
		return
	}
}

func isDegradedGameServerHealthError(err error) bool {
	var statusErr *healthandmetrics.HTTPStatusError
	if errors.As(err, &statusErr) {
		return statusErr.StatusCode == http.StatusServiceUnavailable || statusErr.StatusCode == http.StatusGatewayTimeout
	}

	var netErr net.Error
	return errors.As(err, &netErr) && netErr.Timeout()
}

func (g *gameServerImpl) distributeMapsToServers(ctx context.Context, servers []repo.GameServer) ([]repo.GameServer, error) {
	serversBefore := make([]repo.GameServer, len(servers))
	for i, server := range servers {
		serversBefore[i] = server.Copy()
	}

	distributed := g.mapBalancer.Distribute(servers)

	res := make([]events.GameServer, len(distributed))
	for i := range distributed {
		res[i] = events.GameServer{
			ID:                      distributed[i].ID,
			Address:                 distributed[i].Address,
			RealmID:                 distributed[i].RealmID,
			IsCrossRealm:            distributed[i].IsCrossRealm,
			AvailableMaps:           distributed[i].AvailableMaps,
			NewAssignedMapsToHandle: distributed[i].AssignedMapsToHandle,
		}

		for _, server := range serversBefore {
			if server.ID == distributed[i].ID {
				res[i].OldAssignedMapsToHandle = server.AssignedMapsToHandle
				break
			}
		}
	}

	for i := range distributed {
		for _, server := range res {
			if server.ID == distributed[i].ID {
				for _, previousServer := range serversBefore {
					if previousServer.ID == distributed[i].ID {
						distributed[i].AssignedButPendingMaps = pendingMapsAfterReassignment(previousServer, server)
						break
					}
				}
				break
			}
		}

		if err := g.r.Upsert(ctx, &distributed[i]); err != nil {
			return nil, err
		}
	}

	err := g.eProducer.GSMapsReassigned(&events.ServerRegistryEventGSMapsReassignedPayload{
		Servers: res,
	})
	if err != nil {
		return nil, fmt.Errorf("can't send event for maps reaasigned, err %w", err)
	}

	return distributed, nil
}

func pendingMapsAfterReassignment(previous repo.GameServer, reassigned events.GameServer) []uint32 {
	pending := make([]uint32, 0, len(previous.AssignedButPendingMaps)+len(reassigned.NewAssignedMapsToHandle))
	seen := map[uint32]struct{}{}

	addPending := func(mapID uint32) {
		if _, ok := seen[mapID]; ok {
			return
		}
		seen[mapID] = struct{}{}
		pending = append(pending, mapID)
	}

	for _, mapID := range previous.AssignedButPendingMaps {
		if containsMapID(reassigned.NewAssignedMapsToHandle, mapID) {
			addPending(mapID)
		}
	}

	for _, mapID := range reassigned.OnlyNewMaps() {
		addPending(mapID)
	}

	sort.Slice(pending, func(i, j int) bool {
		return pending[i] < pending[j]
	})

	return pending
}

func containsMapID(maps []uint32, mapID uint32) bool {
	for _, candidate := range maps {
		if candidate == mapID {
			return true
		}
	}
	return false
}

func gameServersAcceptingNewPlayers(servers []repo.GameServer) []repo.GameServer {
	result := make([]repo.GameServer, 0, len(servers))
	for _, server := range servers {
		if server.AcceptsNewPlayers() {
			result = append(result, server)
		}
	}
	return result
}

func hasDegradedMapOwner(servers []repo.GameServer, mapID uint32) bool {
	for _, server := range servers {
		if server.HealthDegraded && server.CanHandleMap(mapID) {
			return true
		}
	}
	return false
}

func (g *gameServerImpl) onMetricsUpdate(server *repo.GameServer, m *healthandmetrics.MetricsRead) {
	err := g.r.Update(context.Background(), server.ID, func(s *repo.GameServer) *repo.GameServer {
		s.ActiveConnections = uint32(m.ActiveConnections)
		s.Diff.Mean = uint32(m.DelayMean)
		s.Diff.Median = uint32(m.DelayMedian)
		s.Diff.Percentile99 = uint32(m.Delay99Percentile)
		s.Diff.Percentile95 = uint32(m.Delay95Percentile)
		s.Diff.Max = uint32(m.DelayMax)
		return s
	})
	if err != nil {
		log.Error().Err(err).Msg("can't update metrics for game server")
	}
}
