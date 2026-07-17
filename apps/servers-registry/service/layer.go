package service

import (
	"context"
	"fmt"
	"github.com/rs/zerolog/log"
	"sort"
	"sync"
	"time"

	"github.com/walkline/ToCloud9/apps/servers-registry/repo"
)

type LayerSelectReason uint8

const (
	LayerSelectLogin LayerSelectReason = iota
	LayerSelectMapChange
	LayerSelectGroupJoin
	LayerSelectManual
	LayerSelectLifecycle
)

type LayerSelectStatus uint8

const (
	LayerSelectOK LayerSelectStatus = iota
	LayerSelectNoServer
	LayerSelectThrottled
	LayerSelectHourlyLimit
)

type LayerSelection struct {
	Status     LayerSelectStatus
	Server     *repo.GameServer
	LayerID    uint32
	RetryAfter time.Duration
}

type Layer interface {
	Select(ctx context.Context, realmID, mapID, zoneID, groupID uint32, playerGUID, preferredPlayerGUID uint64, reason LayerSelectReason, currentAddress string) (LayerSelection, error)
	Poll(ctx context.Context, realmID, mapID, zoneID, groupID uint32, playerGUID uint64, currentAddress string) (LayerSelection, error)
	CompleteSwitch(realmID uint32, playerGUID uint64, success bool)
	Release(realmID uint32, playerGUID uint64)
	Stats(ctx context.Context, realmID, mapID uint32, playerGUID uint64) (LayerStats, error)
	Force(ctx context.Context, realmID uint32, playerGUID uint64, layerID, mapID uint32) LayerForceStatus
	RegistrationLayer(ctx context.Context, realmID uint32) uint32
	MapConfiguration(realmID uint32) map[uint32]uint32
	KubernetesAutoscalingEnabled() bool
	UpdateMapConfiguration(ctx context.Context, realmID uint32, config map[uint32]uint32) error
	BindGroup(ctx context.Context, realmID, groupID, mapID uint32, address string) error
	Run(ctx context.Context)
}

// RegistrationLayer assigns legacy sidecars, which omit layerID, across the
// operator-owned minimum layers. Explicit non-zero sidecar IDs never use this
// compatibility path.
func (l *layerService) RegistrationLayer(ctx context.Context, realmID uint32) uint32 {
	if !l.config.Enabled {
		return 0
	}
	servers, err := l.servers.ListForRealm(ctx, realmID)
	if err != nil {
		return 1
	}
	counts := make(map[uint32]uint32)
	for _, server := range servers {
		if server.LayerID > 0 && server.LayerID <= l.config.MinLayers {
			counts[server.LayerID]++
		}
	}
	selected := uint32(1)
	for id := uint32(2); id <= l.config.MinLayers; id++ {
		if counts[id] < counts[selected] {
			selected = id
		}
	}
	return selected
}

type LayerForceStatus uint8

const (
	LayerForceOK LayerForceStatus = iota
	LayerForcePlayerOffline
	LayerForceNotFound
	LayerForceNoCompatibleCore
)

type LayerStat struct {
	LayerID, CurrentPlayers, ReadyCores uint32
	Draining                            bool
}
type LayerStats struct {
	Enabled                                                                                                                        bool
	MaxPopulation, TargetPopulationPercent, OverflowMarginPercent, MinLayers, MaxLayers, SwitchCooldownSeconds, MaxSwitchesPerHour uint32
	CurrentLayerID, SwitchCooldownRemainingSeconds                                                                                 uint32
	Layers                                                                                                                         []LayerStat
}

type LayerConfig struct {
	Enabled                     bool
	MaxPopulation               uint32
	TargetPopulationPercent     uint32
	OverflowMarginPercent       uint32
	MinCapacityPercent          uint32
	MinCapacityDuration         time.Duration
	SwitchCooldown              time.Duration
	MaxSwitchesPerHour          uint32
	MinLayers                   uint32
	MaxLayers                   uint32
	ReconcileInterval           time.Duration
	RealmIDs                    []uint32
	Scopes                      []LayerScope
	Provisioner                 LayerProvisioner
	MapLayers                   map[uint32]uint32
	EnableKubernetesAutoscaling bool
}

func (l *layerService) Stats(ctx context.Context, realmID, mapID uint32, playerGUID uint64) (LayerStats, error) {
	servers, err := l.servers.AvailableForMapAndRealm(ctx, mapID, realmID, false)
	if err != nil {
		return LayerStats{}, err
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	result := LayerStats{Enabled: l.config.Enabled, MaxPopulation: l.config.MaxPopulation, TargetPopulationPercent: l.config.TargetPopulationPercent, OverflowMarginPercent: l.config.OverflowMarginPercent, MinLayers: l.config.MinLayers, MaxLayers: l.config.MaxLayers, SwitchCooldownSeconds: uint32(l.config.SwitchCooldown / time.Second), MaxSwitchesPerHour: l.config.MaxSwitchesPerHour}
	if assignment := l.assignments[realmID][playerGUID]; assignment != nil && assignment.online {
		result.CurrentLayerID = assignment.layerID
		if remaining := l.config.SwitchCooldown - l.now().Sub(assignment.lastSwitch); !assignment.lastSwitch.IsZero() && remaining > 0 {
			result.SwitchCooldownRemainingSeconds = uint32((remaining + time.Second - 1) / time.Second)
		}
	}
	if l.mapLayers[realmID][mapID] > 1 {
		sort.Slice(servers, func(i, j int) bool { return servers[i].ID < servers[j].ID })
		for i := range servers {
			layerID := uint32(i + 1)
			var population uint32
			for _, assignment := range l.assignments[realmID] {
				if assignment.online && assignment.mapID == mapID && assignment.layerID == layerID {
					population++
				}
			}
			result.Layers = append(result.Layers, LayerStat{LayerID: layerID, CurrentPlayers: population, ReadyCores: 1})
		}
		return result, nil
	}
	cores := make(map[uint32]uint32)
	for _, server := range servers {
		if server.LayerID > 0 {
			cores[server.LayerID]++
		}
	}
	ids := make(map[uint32]bool)
	for id := range cores {
		ids[id] = true
	}
	for id := range l.draining[realmID] {
		ids[id] = true
	}
	for id := range ids {
		result.Layers = append(result.Layers, LayerStat{LayerID: id, CurrentPlayers: l.layerPopulationLocked(realmID, id), ReadyCores: cores[id], Draining: !l.draining[realmID][id].IsZero()})
	}
	sort.Slice(result.Layers, func(i, j int) bool { return result.Layers[i].LayerID < result.Layers[j].LayerID })
	return result, nil
}

func (l *layerService) Force(ctx context.Context, realmID uint32, playerGUID uint64, layerID, mapID uint32) LayerForceStatus {
	servers, err := l.servers.AvailableForMapAndRealm(ctx, mapID, realmID, false)
	if err != nil {
		return LayerForceNoCompatibleCore
	}
	l.mu.Lock()
	configuredCount := l.mapLayers[realmID][mapID]
	l.mu.Unlock()
	if configuredCount > 1 {
		sort.Slice(servers, func(i, j int) bool { return servers[i].ID < servers[j].ID })
		if layerID == 0 || layerID > uint32(len(servers)) || layerID > configuredCount {
			return LayerForceNotFound
		}
		target := servers[layerID-1]
		l.mu.Lock()
		defer l.mu.Unlock()
		assignment := l.assignments[realmID][playerGUID]
		if assignment == nil || !assignment.online {
			return LayerForcePlayerOffline
		}
		assignment.pendingLayerID, assignment.pendingServerAddress, assignment.pendingSince = layerID, target.Address, l.now()
		return LayerForceOK
	}
	var target *repo.GameServer
	layerExists := false
	all, err := l.servers.ListForRealm(ctx, realmID)
	if err == nil {
		for _, server := range all {
			if server.LayerID == layerID {
				layerExists = true
				break
			}
		}
	}
	for i := range servers {
		if servers[i].LayerID == layerID {
			cp := servers[i].Copy()
			target = &cp
			break
		}
	}
	if !layerExists {
		return LayerForceNotFound
	}
	if target == nil {
		return LayerForceNoCompatibleCore
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	assignment := l.assignments[realmID][playerGUID]
	if assignment == nil || !assignment.online {
		return LayerForcePlayerOffline
	}
	if assignment.layerID == layerID {
		return LayerForceOK
	}
	assignment.pendingLayerID, assignment.pendingServerAddress, assignment.pendingSince = layerID, target.Address, l.now()
	return LayerForceOK
}

type LayerScope struct {
	Name          string
	MapIDs        []uint32
	ZoneIDs       []uint32
	MaxPopulation uint32
}

type playerLayerAssignment struct {
	layerID              uint32
	serverAddress        string
	groupID              uint32
	switches             []time.Time
	lastSwitch           time.Time
	online               bool
	offlineSince         time.Time
	mapID                uint32
	zoneID               uint32
	pendingLayerID       uint32
	pendingServerAddress string
	pendingSince         time.Time
	lastSeen             time.Time
}

type layerService struct {
	servers GameServer
	config  LayerConfig
	now     func() time.Time

	mu                  sync.Mutex
	assignments         map[uint32]map[uint64]*playerLayerAssignment
	draining            map[uint32]map[uint32]time.Time
	underpopulatedSince map[uint32]map[uint32]time.Time
	mapLayers           map[uint32]map[uint32]uint32
	groupBindings       map[groupMapKey]groupMapBinding
}

type groupMapKey struct{ realmID, groupID, mapID uint32 }
type groupMapBinding struct {
	serverAddress string
	layerID       uint32
}

func NewLayer(servers GameServer, config LayerConfig) Layer {
	if config.MaxPopulation == 0 {
		config.MaxPopulation = 1000
	}
	if config.TargetPopulationPercent == 0 || config.TargetPopulationPercent > 100 {
		config.TargetPopulationPercent = 90
	}
	if config.OverflowMarginPercent > 100 {
		config.OverflowMarginPercent = 100
	}
	if config.MinCapacityPercent > 100 {
		config.MinCapacityPercent = 100
	}
	if config.MinCapacityDuration <= 0 {
		config.MinCapacityDuration = 5 * time.Minute
	}
	if config.MinLayers == 0 {
		config.MinLayers = 1
	}
	if config.MaxLayers < config.MinLayers {
		config.MaxLayers = config.MinLayers
	}
	if config.ReconcileInterval <= 0 {
		config.ReconcileInterval = 5 * time.Second
	}
	if config.Provisioner == nil {
		config.Provisioner = NoopLayerProvisioner{}
	}
	l := &layerService{
		servers:             servers,
		config:              config,
		now:                 time.Now,
		assignments:         make(map[uint32]map[uint64]*playerLayerAssignment),
		draining:            make(map[uint32]map[uint32]time.Time),
		underpopulatedSince: make(map[uint32]map[uint32]time.Time),
		mapLayers:           make(map[uint32]map[uint32]uint32),
		groupBindings:       make(map[groupMapKey]groupMapBinding),
	}
	for _, realmID := range config.RealmIDs {
		l.mapLayers[realmID] = cloneMapLayerCounts(config.MapLayers)
	}
	return l
}

func (l *layerService) Select(ctx context.Context, realmID, mapID, zoneID, groupID uint32, playerGUID, preferredPlayerGUID uint64, reason LayerSelectReason, currentAddress string) (LayerSelection, error) {
	servers, err := l.servers.AvailableForMapAndRealm(ctx, mapID, realmID, false)
	if err != nil {
		return LayerSelection{}, err
	}
	if len(servers) == 0 {
		return LayerSelection{Status: LayerSelectNoServer}, nil
	}

	l.mu.Lock()
	defer l.mu.Unlock()
	if l.assignments[realmID] == nil {
		l.assignments[realmID] = make(map[uint64]*playerLayerAssignment)
	}
	realmAssignments := l.assignments[realmID]
	now := l.now()
	for guid, assignment := range realmAssignments {
		if !assignment.online && !assignment.offlineSince.IsZero() && now.Sub(assignment.offlineSince) >= time.Hour &&
			(assignment.lastSwitch.IsZero() || now.Sub(assignment.lastSwitch) >= time.Hour) {
			delete(realmAssignments, guid)
		}
	}
	current := realmAssignments[playerGUID]
	if l.mapLayers[realmID][mapID] > 1 {
		// When the whole group has left this map (for example to enter an
		// instance), its old outdoor binding must not pin it to the layer it
		// departed. The first member returning chooses a fresh destination;
		// subsequent members see that member on the map and follow the new
		// binding. This is performed under the registry lock to avoid races.
		if groupID != 0 && reason == LayerSelectMapChange && current != nil && current.mapID != mapID {
			groupPresentOnMap := false
			for guid, assignment := range realmAssignments {
				if guid != playerGUID && assignment.online && assignment.groupID == groupID && assignment.mapID == mapID {
					groupPresentOnMap = true
					break
				}
			}
			if !groupPresentOnMap {
				delete(l.groupBindings, groupMapKey{realmID, groupID, mapID})
			}
		}
		selection := l.selectMapLayerLocked(realmID, mapID, groupID, servers)
		if selection.Server == nil {
			return selection, nil
		}
		if current == nil {
			current = &playerLayerAssignment{}
		}
		current.layerID, current.serverAddress = selection.LayerID, selection.Server.Address
		current.online, current.lastSeen, current.offlineSince = true, now, time.Time{}
		current.mapID, current.zoneID, current.groupID = mapID, zoneID, groupID
		realmAssignments[playerGUID] = current
		return selection, nil
	}

	targetLayer, found := l.targetLayer(realmID, mapID, zoneID, servers, realmAssignments, current, preferredPlayerGUID, reason)
	if !found {
		return LayerSelection{Status: LayerSelectNoServer}, nil
	}
	target := leastLoadedServer(servers, targetLayer)
	if target == nil {
		return LayerSelection{Status: LayerSelectNoServer}, nil
	}

	isSwitch := reason == LayerSelectMapChange || reason == LayerSelectGroupJoin || reason == LayerSelectManual || reason == LayerSelectLifecycle
	policyControlledSwitch := reason == LayerSelectGroupJoin || reason == LayerSelectManual
	if policyControlledSwitch && currentAddress != "" && currentAddress != target.Address {
		if current == nil {
			current = &playerLayerAssignment{}
		}
		if retry := l.config.SwitchCooldown - now.Sub(current.lastSwitch); !current.lastSwitch.IsZero() && retry > 0 {
			return LayerSelection{Status: LayerSelectThrottled, RetryAfter: retry}, nil
		}
		cutoff := now.Add(-time.Hour)
		kept := current.switches[:0]
		for _, switchedAt := range current.switches {
			if switchedAt.After(cutoff) {
				kept = append(kept, switchedAt)
			}
		}
		current.switches = kept
		if l.config.MaxSwitchesPerHour > 0 && uint32(len(current.switches)) >= l.config.MaxSwitchesPerHour {
			retry := time.Until(current.switches[0].Add(time.Hour))
			if l.now != nil { // keep tests with injected clocks deterministic
				retry = current.switches[0].Add(time.Hour).Sub(now)
			}
			return LayerSelection{Status: LayerSelectHourlyLimit, RetryAfter: retry}, nil
		}
		current.lastSwitch = now
		current.switches = append(current.switches, now)
	}

	if current == nil {
		current = &playerLayerAssignment{}
	}
	if isSwitch && currentAddress != "" && currentAddress != target.Address {
		current.pendingLayerID = target.LayerID
		current.pendingServerAddress = target.Address
		current.pendingSince = now
	} else {
		current.layerID = target.LayerID
		current.serverAddress = target.Address
	}
	current.online = true
	current.lastSeen = now
	current.offlineSince = time.Time{}
	current.mapID = mapID
	current.zoneID = zoneID
	current.groupID = groupID
	realmAssignments[playerGUID] = current
	copy := target.Copy()
	return LayerSelection{Status: LayerSelectOK, Server: &copy, LayerID: target.LayerID}, nil
}

func (l *layerService) selectMapLayerLocked(realmID, mapID, groupID uint32, servers []repo.GameServer) LayerSelection {
	sort.Slice(servers, func(i, j int) bool { return servers[i].ID < servers[j].ID })
	if groupID != 0 {
		key := groupMapKey{realmID, groupID, mapID}
		if binding, ok := l.groupBindings[key]; ok {
			for i := range servers {
				if servers[i].Address == binding.serverAddress {
					cp := servers[i].Copy()
					return LayerSelection{Status: LayerSelectOK, Server: &cp, LayerID: uint32(i + 1)}
				}
			}
			delete(l.groupBindings, key)
		}
	}
	if len(servers) == 0 {
		return LayerSelection{Status: LayerSelectNoServer}
	}
	selected := 0
	populations := make([]uint32, len(servers))
	trackedOnMap := false
	for _, assignment := range l.assignments[realmID] {
		if !assignment.online || assignment.mapID != mapID {
			continue
		}
		if assignment.layerID > 0 && assignment.layerID <= uint32(len(populations)) {
			populations[assignment.layerID-1]++
			trackedOnMap = true
		}
	}
	for i := 1; i < len(servers); i++ {
		lessLoaded := populations[i] < populations[selected]
		if !trackedOnMap {
			lessLoaded = servers[i].ActiveConnections < servers[selected].ActiveConnections
		}
		equalLoad := populations[i] == populations[selected]
		if !trackedOnMap {
			equalLoad = servers[i].ActiveConnections == servers[selected].ActiveConnections
		}
		if lessLoaded || (equalLoad && (servers[i].ActiveConnections < servers[selected].ActiveConnections ||
			(servers[i].ActiveConnections == servers[selected].ActiveConnections && servers[i].ID < servers[selected].ID))) {
			selected = i
		}
	}
	cp := servers[selected].Copy()
	result := LayerSelection{Status: LayerSelectOK, Server: &cp, LayerID: uint32(selected + 1)}
	if groupID != 0 {
		l.groupBindings[groupMapKey{realmID, groupID, mapID}] = groupMapBinding{cp.Address, result.LayerID}
	}
	return result
}

func (l *layerService) MapConfiguration(realmID uint32) map[uint32]uint32 {
	l.mu.Lock()
	defer l.mu.Unlock()
	return cloneMapLayerCounts(l.mapLayers[realmID])
}
func (l *layerService) KubernetesAutoscalingEnabled() bool {
	return l.config.EnableKubernetesAutoscaling
}
func (l *layerService) UpdateMapConfiguration(ctx context.Context, realmID uint32, config map[uint32]uint32) error {
	for _, count := range config {
		if count == 0 {
			return fmt.Errorf("layer count must be positive")
		}
	}
	if err := l.servers.UpdateMapLayerConfiguration(ctx, realmID, config); err != nil {
		return err
	}
	l.mu.Lock()
	l.mapLayers[realmID] = cloneMapLayerCounts(config)
	for key := range l.groupBindings {
		if key.realmID == realmID {
			delete(l.groupBindings, key)
		}
	}
	l.mu.Unlock()
	return nil
}
func (l *layerService) BindGroup(ctx context.Context, realmID, groupID, mapID uint32, address string) error {
	if groupID == 0 {
		return fmt.Errorf("group ID must be non-zero")
	}
	servers, err := l.servers.AvailableForMapAndRealm(ctx, mapID, realmID, false)
	if err != nil {
		return err
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	for i := range servers {
		if servers[i].Address == address {
			sort.Slice(servers, func(i, j int) bool { return servers[i].ID < servers[j].ID })
			layerID := uint32(1)
			for j := range servers {
				if servers[j].Address == address {
					layerID = uint32(j + 1)
					break
				}
			}
			l.groupBindings[groupMapKey{realmID, groupID, mapID}] = groupMapBinding{address, layerID}
			return nil
		}
	}
	return fmt.Errorf("game server %s is unavailable for map %d", address, mapID)
}

func (l *layerService) targetLayer(realmID, mapID, zoneID uint32, servers []repo.GameServer, assignments map[uint64]*playerLayerAssignment, current *playerLayerAssignment, preferred uint64, reason LayerSelectReason) (uint32, bool) {
	if !l.config.Enabled {
		return servers[0].LayerID, true
	}
	available := make(map[uint32]bool)
	for _, server := range servers {
		if l.draining[realmID] == nil || l.draining[realmID][server.LayerID].IsZero() {
			available[server.LayerID] = true
		}
	}
	populations := make(map[uint32]uint32)
	scope := l.scopeFor(mapID, zoneID)
	for _, assignment := range assignments {
		if assignment.online && scope.matches(assignment.mapID, assignment.zoneID) {
			layerID := assignment.layerID
			if assignment.pendingLayerID != 0 {
				layerID = assignment.pendingLayerID
			}
			populations[layerID]++
		}
	}
	if preferred != 0 {
		if assignment := assignments[preferred]; assignment != nil && assignment.online && available[assignment.layerID] {
			if current != nil && current.layerID == assignment.layerID {
				return assignment.layerID, true
			}
			if populations[assignment.layerID] < scope.overflowPopulation(l.config.MaxPopulation, l.config.OverflowMarginPercent) {
				return assignment.layerID, true
			}
			// Party reunification may use the overflow margin, but never exceed
			// the layer hard cap.
			return 0, false
		}
	}
	// An ordinary heartbeat or explicit same-map operation never rebalances an
	// active character. Login and map transitions are safe placement points:
	// the character is already detached from (or reconnecting to) a core.
	if current != nil && reason != LayerSelectLogin && reason != LayerSelectMapChange && available[current.layerID] {
		return current.layerID, true
	}
	layers := make([]uint32, 0, len(available))
	for layerID := range available {
		layers = append(layers, layerID)
	}
	if len(layers) == 0 {
		return 0, false
	}
	sort.Slice(layers, func(i, j int) bool { return layers[i] < layers[j] })
	for _, layerID := range layers {
		if populations[layerID] < scope.targetPopulation(l.config.MaxPopulation, l.config.TargetPopulationPercent) {
			return layerID, true
		}
	}
	// Provisioning is asynchronous. Use only the configured overflow margin
	// while a requested layer starts; never place above the hard cap.
	var best uint32
	for _, layerID := range layers {
		if populations[layerID] < scope.overflowPopulation(l.config.MaxPopulation, l.config.OverflowMarginPercent) &&
			(best == 0 || populations[layerID] < populations[best]) {
			best = layerID
		}
	}
	return best, best != 0
}

func (l *layerService) Poll(ctx context.Context, realmID, mapID, zoneID, groupID uint32, playerGUID uint64, currentAddress string) (LayerSelection, error) {
	servers, err := l.servers.AvailableForMapAndRealm(ctx, mapID, realmID, false)
	if err != nil {
		return LayerSelection{}, err
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.assignments[realmID] == nil {
		l.assignments[realmID] = make(map[uint64]*playerLayerAssignment)
	}
	assignment := l.assignments[realmID][playerGUID]
	if assignment != nil && assignment.pendingLayerID != 0 {
		if currentAddress == assignment.pendingServerAddress {
			assignment.layerID, assignment.serverAddress = assignment.pendingLayerID, assignment.pendingServerAddress
			assignment.pendingLayerID, assignment.pendingServerAddress, assignment.pendingSince = 0, "", time.Time{}
			return LayerSelection{Status: LayerSelectOK}, nil
		}
		for i := range servers {
			if servers[i].Address == assignment.pendingServerAddress {
				cp := servers[i].Copy()
				return LayerSelection{Status: LayerSelectOK, Server: &cp, LayerID: assignment.pendingLayerID}, nil
			}
		}
		if l.now().Sub(assignment.pendingSince) < 30*time.Second {
			return LayerSelection{Status: LayerSelectNoServer}, nil
		}
		assignment.pendingLayerID, assignment.pendingServerAddress, assignment.pendingSince = 0, "", time.Time{}
	}
	if l.mapLayers[realmID][mapID] > 1 {
		ordered := append([]repo.GameServer(nil), servers...)
		sort.Slice(ordered, func(i, j int) bool { return ordered[i].ID < ordered[j].ID })
		currentAvailable := false
		currentLayerID := uint32(0)
		for i := range ordered {
			if ordered[i].Address == currentAddress {
				currentAvailable = true
				currentLayerID = uint32(i + 1)
				break
			}
		}
		key := groupMapKey{realmID, groupID, mapID}
		if groupID != 0 {
			if _, exists := l.groupBindings[key]; !exists && currentAvailable {
				layerID := uint32(1)
				for i := range ordered {
					if ordered[i].Address == currentAddress {
						layerID = uint32(i + 1)
						break
					}
				}
				l.groupBindings[key] = groupMapBinding{currentAddress, layerID}
			}
			selection := l.selectMapLayerLocked(realmID, mapID, groupID, servers)
			if selection.Server != nil && selection.Server.Address != currentAddress {
				return selection, nil
			}
		}
		if currentAvailable {
			if assignment == nil {
				assignment = &playerLayerAssignment{}
			}
			assignment.layerID, assignment.serverAddress = currentLayerID, currentAddress
			assignment.online, assignment.offlineSince = true, time.Time{}
			assignment.mapID, assignment.zoneID, assignment.groupID, assignment.lastSeen = mapID, zoneID, groupID, l.now()
			l.assignments[realmID][playerGUID] = assignment
			return LayerSelection{Status: LayerSelectOK}, nil
		}
		return l.selectMapLayerLocked(realmID, mapID, groupID, servers), nil
	}
	if assignment == nil || !assignment.online {
		// Registry state is intentionally in-memory, while gateways and cores can
		// outlive a registry rollout. Reconstruct an online assignment from the
		// heartbeat's current core address so GM force-switches and population
		// accounting recover without requiring players to reconnect.
		for i := range servers {
			if servers[i].Address != currentAddress {
				continue
			}
			if assignment == nil {
				assignment = &playerLayerAssignment{}
			}
			assignment.layerID = servers[i].LayerID
			assignment.serverAddress = currentAddress
			assignment.online = true
			assignment.offlineSince = time.Time{}
			assignment.mapID, assignment.zoneID, assignment.groupID = mapID, zoneID, groupID
			assignment.lastSeen = l.now()
			l.assignments[realmID][playerGUID] = assignment
			break
		}
		return LayerSelection{Status: LayerSelectOK}, nil
	}
	assignment.mapID, assignment.zoneID, assignment.groupID = mapID, zoneID, groupID
	assignment.lastSeen = l.now()
	// Poll is deliberately heartbeat/retry-only. Population changes and drain
	// state must never redirect a character while it is actively playing.
	return LayerSelection{Status: LayerSelectOK}, nil
}

func (l *layerService) CompleteSwitch(realmID uint32, playerGUID uint64, success bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	assignment := l.assignments[realmID][playerGUID]
	if assignment == nil || assignment.pendingLayerID == 0 {
		return
	}
	if success {
		assignment.layerID, assignment.serverAddress = assignment.pendingLayerID, assignment.pendingServerAddress
	}
	assignment.pendingLayerID, assignment.pendingServerAddress, assignment.pendingSince = 0, "", time.Time{}
}

func (l *layerService) Run(ctx context.Context) {
	ticker := time.NewTicker(l.config.ReconcileInterval)
	defer ticker.Stop()
	l.reconcile(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			l.reconcile(ctx)
		}
	}
}

func (l *layerService) reconcile(ctx context.Context) {
	for _, realmID := range l.config.RealmIDs {
		servers, err := l.servers.ListForRealm(ctx, realmID)
		if err != nil {
			continue
		}
		active := make(map[uint32]bool)
		for _, server := range servers {
			if server.LayerID > 0 {
				active[server.LayerID] = true
			}
		}
		l.mu.Lock()
		for _, assignment := range l.assignments[realmID] {
			if assignment.online && !assignment.lastSeen.IsZero() && l.now().Sub(assignment.lastSeen) > 30*time.Second {
				assignment.online = false
				assignment.offlineSince = l.now()
				assignment.pendingLayerID, assignment.pendingServerAddress, assignment.pendingSince = 0, "", time.Time{}
			}
		}
		desired := l.desiredLayersLocked(realmID)
		if l.draining[realmID] == nil {
			l.draining[realmID] = make(map[uint32]time.Time)
		}
		if l.underpopulatedSince[realmID] == nil {
			l.underpopulatedSince[realmID] = make(map[uint32]time.Time)
		}
		// Scale-down is deliberately per-layer and passive. A low-population
		// non-base layer must remain below the configured floor for the full
		// duration before it stops receiving new placements. Existing players
		// remain there until logout or another natural safe transition.
		usableLayers := uint32(len(active))
		var highestUsableLayer uint32
		for layerID := range l.draining[realmID] {
			if active[layerID] && usableLayers > 0 {
				usableLayers--
			}
		}
		for layerID := range active {
			if l.draining[realmID][layerID].IsZero() && layerID > highestUsableLayer {
				highestUsableLayer = layerID
			}
		}
		for layerID := range active {
			// Provisioner-owned IDs are contiguous, so only retire the highest
			// usable layer. Removing a middle ID would make EnsureLayer recreate it.
			if layerID <= l.config.MinLayers || layerID != highestUsableLayer || !l.draining[realmID][layerID].IsZero() {
				continue
			}
			population := l.layerPopulationLocked(realmID, layerID)
			minimum := percentageCapacity(l.config.MaxPopulation, l.config.MinCapacityPercent)
			if usableLayers <= desired || population > minimum {
				delete(l.underpopulatedSince[realmID], layerID)
				continue
			}
			if l.underpopulatedSince[realmID][layerID].IsZero() {
				l.underpopulatedSince[realmID][layerID] = l.now()
				continue
			}
			if l.now().Sub(l.underpopulatedSince[realmID][layerID]) >= l.config.MinCapacityDuration {
				l.draining[realmID][layerID] = l.now()
				delete(l.underpopulatedSince[realmID], layerID)
				usableLayers--
			}
		}
		toDelete := make([]uint32, 0)
		for layerID := range l.draining[realmID] {
			if l.layerPopulationLocked(realmID, layerID) == 0 {
				toDelete = append(toDelete, layerID)
			}
		}
		l.mu.Unlock()
		if !l.config.EnableKubernetesAutoscaling {
			continue
		}
		// Minimum layers are operator-owned base deployments. Every layer above
		// that floor is provisioner-owned; EnsureLayer is deliberately idempotent
		// so partially-created multi-core layers are repaired on every reconcile.
		for layerID := l.config.MinLayers + 1; layerID <= desired; layerID++ {
			if err := l.config.Provisioner.EnsureLayer(ctx, realmID, layerID); err != nil {
				log.Error().Err(err).Uint32("realmID", realmID).Uint32("layerID", layerID).Msg("can't provision layer")
			}
		}
		for _, layerID := range toDelete {
			if err := l.config.Provisioner.DeleteLayer(ctx, realmID, layerID); err == nil {
				l.mu.Lock()
				delete(l.draining[realmID], layerID)
				l.mu.Unlock()
			} else {
				log.Error().Err(err).Uint32("realmID", realmID).Uint32("layerID", layerID).Msg("can't delete layer")
			}
		}
	}
}

func (l *layerService) desiredLayersLocked(realmID uint32) uint32 {
	desired := l.config.MinLayers
	if len(l.mapLayers[realmID]) > 0 {
		for _, count := range l.mapLayers[realmID] {
			if count > desired {
				desired = count
			}
		}
		if desired > l.config.MaxLayers {
			desired = l.config.MaxLayers
		}
		return desired
	}
	for _, scope := range l.effectiveScopes() {
		var population uint32
		for _, assignment := range l.assignments[realmID] {
			if assignment.online && scope.matches(assignment.mapID, assignment.zoneID) {
				population++
			}
		}
		limit := scope.targetPopulation(l.config.MaxPopulation, l.config.TargetPopulationPercent)
		var needed uint32
		if population > 0 {
			// Reaching the target requests the next layer. The overflow margin
			// absorbs arrivals while that layer starts and loads its maps.
			needed = population/limit + 1
		}
		if needed > desired {
			desired = needed
		}
	}
	if desired < l.config.MinLayers {
		desired = l.config.MinLayers
	}
	if desired > l.config.MaxLayers {
		desired = l.config.MaxLayers
	}
	return desired
}

func (l *layerService) layerPopulationLocked(realmID, layerID uint32) uint32 {
	var population uint32
	for _, assignment := range l.assignments[realmID] {
		if assignment.online && (assignment.layerID == layerID || assignment.pendingLayerID == layerID) {
			population++
		}
	}
	return population
}

func (l *layerService) effectiveScopes() []LayerScope {
	if len(l.config.Scopes) > 0 {
		return l.config.Scopes
	}
	return []LayerScope{{Name: "realm", MaxPopulation: l.config.MaxPopulation}}
}
func (l *layerService) scopeFor(mapID, zoneID uint32) LayerScope {
	for _, scope := range l.effectiveScopes() {
		if scope.matches(mapID, zoneID) {
			return scope
		}
	}
	return LayerScope{Name: "realm", MaxPopulation: l.config.MaxPopulation}
}
func (s LayerScope) matches(mapID, zoneID uint32) bool {
	if len(s.MapIDs) == 0 && len(s.ZoneIDs) == 0 {
		return true
	}
	for _, id := range s.ZoneIDs {
		if id == zoneID {
			return true
		}
	}
	for _, id := range s.MapIDs {
		if id == mapID {
			return true
		}
	}
	return false
}
func (s LayerScope) maxPopulation(fallback uint32) uint32 {
	if s.MaxPopulation > 0 {
		return s.MaxPopulation
	}
	if fallback > 0 {
		return fallback
	}
	return 1
}

func (s LayerScope) targetPopulation(fallback, percent uint32) uint32 {
	return percentageCapacity(s.maxPopulation(fallback), percent)
}

func (s LayerScope) overflowPopulation(fallback, marginPercent uint32) uint32 {
	maximum := s.maxPopulation(fallback)
	return maximum + percentageCapacity(maximum, marginPercent)
}

func percentageCapacity(capacity, percent uint32) uint32 {
	if percent == 0 {
		return 0
	}
	result := (uint64(capacity)*uint64(percent) + 99) / 100
	if result == 0 {
		return 1
	}
	return uint32(result)
}

func leastLoadedServer(servers []repo.GameServer, layerID uint32) *repo.GameServer {
	var selected *repo.GameServer
	for i := range servers {
		if servers[i].LayerID != layerID {
			continue
		}
		if selected == nil || servers[i].ActiveConnections < selected.ActiveConnections ||
			(servers[i].ActiveConnections == selected.ActiveConnections && servers[i].ID < selected.ID) {
			selected = &servers[i]
		}
	}
	return selected
}

func (l *layerService) Release(realmID uint32, playerGUID uint64) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if assignment := l.assignments[realmID][playerGUID]; assignment != nil {
		// Keep switch history so relogging cannot bypass cooldown/hourly limits,
		// but exclude the player from population and group-layer lookups.
		assignment.online = false
		assignment.offlineSince = l.now()
	}
}
