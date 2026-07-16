package service

import (
	"context"
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
	Select(ctx context.Context, realmID, mapID uint32, playerGUID, preferredPlayerGUID uint64, reason LayerSelectReason, currentAddress string) (LayerSelection, error)
	Release(realmID uint32, playerGUID uint64)
}

type LayerConfig struct {
	Enabled            bool
	MaxPopulation      uint32
	SwitchCooldown     time.Duration
	MaxSwitchesPerHour uint32
}

type playerLayerAssignment struct {
	layerID       uint32
	serverAddress string
	switches      []time.Time
	lastSwitch    time.Time
	online        bool
	offlineSince  time.Time
}

type layerService struct {
	servers GameServer
	config  LayerConfig
	now     func() time.Time

	mu          sync.Mutex
	assignments map[uint32]map[uint64]*playerLayerAssignment
}

func NewLayer(servers GameServer, config LayerConfig) Layer {
	if config.MaxPopulation == 0 {
		config.MaxPopulation = 1000
	}
	return &layerService{
		servers:     servers,
		config:      config,
		now:         time.Now,
		assignments: make(map[uint32]map[uint64]*playerLayerAssignment),
	}
}

func (l *layerService) Select(ctx context.Context, realmID, mapID uint32, playerGUID, preferredPlayerGUID uint64, reason LayerSelectReason, currentAddress string) (LayerSelection, error) {
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

	targetLayer, found := l.targetLayer(servers, realmAssignments, current, preferredPlayerGUID)
	if !found {
		return LayerSelection{Status: LayerSelectNoServer}, nil
	}
	target := leastLoadedServer(servers, targetLayer)
	if target == nil {
		return LayerSelection{Status: LayerSelectNoServer}, nil
	}

	isSwitch := reason == LayerSelectGroupJoin || reason == LayerSelectManual
	if isSwitch && currentAddress != "" && currentAddress != target.Address {
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
	current.layerID = target.LayerID
	current.serverAddress = target.Address
	current.online = true
	current.offlineSince = time.Time{}
	realmAssignments[playerGUID] = current
	copy := target.Copy()
	return LayerSelection{Status: LayerSelectOK, Server: &copy, LayerID: target.LayerID}, nil
}

func (l *layerService) targetLayer(servers []repo.GameServer, assignments map[uint64]*playerLayerAssignment, current *playerLayerAssignment, preferred uint64) (uint32, bool) {
	if !l.config.Enabled {
		return servers[0].LayerID, true
	}
	available := make(map[uint32]bool)
	for _, server := range servers {
		available[server.LayerID] = true
	}
	if preferred != 0 {
		if assignment := assignments[preferred]; assignment != nil && assignment.online && available[assignment.layerID] {
			return assignment.layerID, true
		}
	}
	if current != nil && available[current.layerID] {
		return current.layerID, true
	}
	populations := make(map[uint32]uint32)
	for _, assignment := range assignments {
		if assignment.online {
			populations[assignment.layerID]++
		}
	}
	layers := make([]uint32, 0, len(available))
	for layerID := range available {
		layers = append(layers, layerID)
	}
	sort.Slice(layers, func(i, j int) bool { return layers[i] < layers[j] })
	for _, layerID := range layers {
		if populations[layerID] < l.config.MaxPopulation {
			return layerID, true
		}
	}
	// All configured layers are full. Keep accepting on the least populated
	// layer so players are not locked out when the external autoscaler lags.
	best := layers[0]
	for _, layerID := range layers[1:] {
		if populations[layerID] < populations[best] {
			best = layerID
		}
	}
	return best, true
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
