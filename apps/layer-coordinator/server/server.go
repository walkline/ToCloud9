package server

import (
	"context"
	"sort"
	"sync"
	"time"

	pbCoordinator "github.com/walkline/ToCloud9/gen/layer-coordinator/pb"
	pbRegistry "github.com/walkline/ToCloud9/gen/servers-registry/pb"
)

type playerState struct {
	layerID, mapID, groupID            uint32
	serverAddress                      string
	pendingLayerID                     uint32
	pendingServerAddress               string
	online                             bool
	lastSeen, lastSwitch, pendingSince time.Time
}

// Server owns the optional player transition lifecycle. It delegates only the
// authoritative map/group destination decision to the servers registry.
type Server struct {
	pbCoordinator.UnimplementedLayerCoordinatorServiceServer
	registry pbRegistry.ServersRegistryServiceClient
	now      func() time.Time
	mu       sync.Mutex
	players  map[uint32]map[uint64]*playerState
	options  Options
}

type Options struct {
	MaxPopulation, TargetPopulationPercent, OverflowMarginPercent uint32
	SwitchCooldownSeconds, MaxSwitchesPerHour                     uint32
}

func New(registry pbRegistry.ServersRegistryServiceClient, options ...Options) *Server {
	var configured Options
	if len(options) > 0 {
		configured = options[0]
	}
	return &Server{registry: registry, now: time.Now, players: make(map[uint32]map[uint64]*playerState), options: configured}
}

func (s *Server) SelectGameServerForPlayer(ctx context.Context, req *pbRegistry.SelectGameServerForPlayerRequest) (*pbRegistry.SelectGameServerForPlayerResponse, error) {
	selection, err := s.registry.SelectGameServerForPlayer(ctx, req)
	if err != nil || selection.Status != pbRegistry.SelectGameServerForPlayerResponse_OK || selection.GameServer == nil {
		return selection, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	state := s.playerLocked(req.RealmID, req.PlayerGUID)
	state.online, state.lastSeen = true, s.now()
	state.mapID, state.groupID = req.MapID, req.GroupID
	if req.CurrentGameServerAddress != "" && req.CurrentGameServerAddress != selection.GameServer.Address {
		state.pendingLayerID, state.pendingServerAddress = selection.LayerID, selection.GameServer.Address
		state.pendingSince, state.lastSwitch = s.now(), s.now()
	} else {
		state.layerID, state.serverAddress = selection.LayerID, selection.GameServer.Address
	}
	return selection, nil
}

func (s *Server) PollPlayerLayerAction(ctx context.Context, req *pbRegistry.PollPlayerLayerActionRequest) (*pbRegistry.SelectGameServerForPlayerResponse, error) {
	s.mu.Lock()
	state := s.playerLocked(req.RealmID, req.PlayerGUID)
	state.online, state.lastSeen, state.mapID, state.groupID = true, s.now(), req.MapID, req.GroupID
	if state.pendingServerAddress != "" {
		if state.pendingServerAddress == req.CurrentGameServerAddress {
			state.layerID, state.serverAddress = state.pendingLayerID, state.pendingServerAddress
			state.pendingLayerID, state.pendingServerAddress = 0, ""
			s.mu.Unlock()
			return &pbRegistry.SelectGameServerForPlayerResponse{Api: req.Api, Status: pbRegistry.SelectGameServerForPlayerResponse_OK}, nil
		}
		pendingAddress, pendingLayer := state.pendingServerAddress, state.pendingLayerID
		s.mu.Unlock()
		return s.serverResponse(ctx, req.Api, req.RealmID, req.MapID, pendingAddress, pendingLayer)
	}
	s.mu.Unlock()
	if req.GroupID == 0 {
		available, err := s.registry.AvailableGameServersForMapAndRealm(ctx, &pbRegistry.AvailableGameServersForMapAndRealmRequest{Api: req.Api, RealmID: req.RealmID, MapID: req.MapID})
		if err != nil {
			return nil, err
		}
		for _, server := range available.GameServers {
			if server != nil && server.Address == req.CurrentGameServerAddress {
				s.mu.Lock()
				state := s.playerLocked(req.RealmID, req.PlayerGUID)
				state.layerID, state.serverAddress = server.LayerID, server.Address
				s.mu.Unlock()
				return &pbRegistry.SelectGameServerForPlayerResponse{Api: req.Api, Status: pbRegistry.SelectGameServerForPlayerResponse_OK}, nil
			}
		}
	}

	return s.SelectGameServerForPlayer(ctx, &pbRegistry.SelectGameServerForPlayerRequest{
		Api: req.Api, RealmID: req.RealmID, MapID: req.MapID, ZoneID: req.ZoneID,
		PlayerGUID: req.PlayerGUID, GroupID: req.GroupID,
		Reason:                   pbRegistry.SelectGameServerForPlayerRequest_MAP_CHANGE,
		CurrentGameServerAddress: req.CurrentGameServerAddress,
	})
}

func (s *Server) CompletePlayerLayerSwitch(_ context.Context, req *pbRegistry.CompletePlayerLayerSwitchRequest) (*pbRegistry.CompletePlayerLayerSwitchResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if state := s.players[req.RealmID][req.PlayerGUID]; state != nil {
		if req.Success {
			state.layerID, state.serverAddress = state.pendingLayerID, state.pendingServerAddress
		}
		state.pendingLayerID, state.pendingServerAddress = 0, ""
	}
	return &pbRegistry.CompletePlayerLayerSwitchResponse{Api: req.Api}, nil
}

func (s *Server) ReleasePlayerLayer(_ context.Context, req *pbRegistry.ReleasePlayerLayerRequest) (*pbRegistry.ReleasePlayerLayerResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if state := s.players[req.RealmID][req.PlayerGUID]; state != nil {
		state.online = false
		state.pendingLayerID, state.pendingServerAddress = 0, ""
	}
	return &pbRegistry.ReleasePlayerLayerResponse{Api: req.Api}, nil
}

func (s *Server) GetLayerStats(ctx context.Context, req *pbRegistry.GetLayerStatsRequest) (*pbRegistry.GetLayerStatsResponse, error) {
	servers, err := s.registry.ListGameServersForRealm(ctx, &pbRegistry.ListGameServersForRealmRequest{Api: req.Api, RealmID: req.RealmID})
	if err != nil {
		return nil, err
	}
	config, err := s.registry.GetMapLayerConfiguration(ctx, &pbRegistry.GetMapLayerConfigurationRequest{Api: req.Api, RealmID: req.RealmID})
	if err != nil {
		return nil, err
	}
	result := &pbRegistry.GetLayerStatsResponse{Api: req.Api, Enabled: len(config.Maps) > 0, MaxPopulation: s.options.MaxPopulation, TargetPopulationPercent: s.options.TargetPopulationPercent, OverflowMarginPercent: s.options.OverflowMarginPercent, SwitchCooldownSeconds: s.options.SwitchCooldownSeconds, MaxSwitchesPerHour: s.options.MaxSwitchesPerHour}
	cores := make(map[uint32]uint32)
	for _, server := range servers.GameServers {
		if server == nil || server.LayerID == 0 {
			continue
		}
		for _, mapID := range server.AssignedMaps {
			if mapID == req.MapID {
				cores[server.LayerID]++
				break
			}
		}
	}
	s.mu.Lock()
	populations := make(map[uint32]uint32)
	for guid, state := range s.players[req.RealmID] {
		if state.online && state.mapID == req.MapID {
			populations[state.layerID]++
		}
		if guid == req.PlayerGUID && state.online {
			result.CurrentLayerID = state.layerID
		}
	}
	s.mu.Unlock()
	for id, ready := range cores {
		result.Layers = append(result.Layers, &pbRegistry.GetLayerStatsResponse_Layer{LayerID: id, CurrentPlayers: populations[id], ReadyGameServers: ready})
	}
	sort.Slice(result.Layers, func(i, j int) bool { return result.Layers[i].LayerID < result.Layers[j].LayerID })
	return result, nil
}

func (s *Server) ForcePlayerLayer(ctx context.Context, req *pbRegistry.ForcePlayerLayerRequest) (*pbRegistry.ForcePlayerLayerResponse, error) {
	servers, err := s.registry.ListGameServersForRealm(ctx, &pbRegistry.ListGameServersForRealmRequest{Api: req.Api, RealmID: req.RealmID})
	if err != nil {
		return nil, err
	}
	var target *pbRegistry.GameServerDetailed
	for _, server := range servers.GameServers {
		if server != nil && server.LayerID == req.LayerID {
			for _, mapID := range server.AssignedMaps {
				if mapID == req.MapID {
					target = server
					break
				}
			}
		}
	}
	if target == nil {
		return &pbRegistry.ForcePlayerLayerResponse{Api: req.Api, Status: pbRegistry.ForcePlayerLayerResponse_NO_COMPATIBLE_CORE}, nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	state := s.players[req.RealmID][req.PlayerGUID]
	if state == nil || !state.online {
		return &pbRegistry.ForcePlayerLayerResponse{Api: req.Api, Status: pbRegistry.ForcePlayerLayerResponse_PLAYER_OFFLINE}, nil
	}
	state.pendingLayerID, state.pendingServerAddress, state.pendingSince = req.LayerID, target.Address, s.now()
	return &pbRegistry.ForcePlayerLayerResponse{Api: req.Api, Status: pbRegistry.ForcePlayerLayerResponse_OK}, nil
}

func (s *Server) playerLocked(realmID uint32, guid uint64) *playerState {
	if s.players[realmID] == nil {
		s.players[realmID] = make(map[uint64]*playerState)
	}
	if s.players[realmID][guid] == nil {
		s.players[realmID][guid] = &playerState{}
	}
	return s.players[realmID][guid]
}

func (s *Server) serverResponse(ctx context.Context, api string, realmID, mapID uint32, address string, layerID uint32) (*pbRegistry.SelectGameServerForPlayerResponse, error) {
	servers, err := s.registry.AvailableGameServersForMapAndRealm(ctx, &pbRegistry.AvailableGameServersForMapAndRealmRequest{Api: api, RealmID: realmID, MapID: mapID})
	if err != nil {
		return nil, err
	}
	for _, server := range servers.GameServers {
		if server != nil && server.Address == address {
			return &pbRegistry.SelectGameServerForPlayerResponse{Api: api, Status: pbRegistry.SelectGameServerForPlayerResponse_OK, GameServer: server, LayerID: layerID}, nil
		}
	}
	return &pbRegistry.SelectGameServerForPlayerResponse{Api: api, Status: pbRegistry.SelectGameServerForPlayerResponse_NO_SERVER}, nil
}
