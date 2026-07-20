package service

import (
	"context"
	"fmt"
	"sort"

	"github.com/walkline/ToCloud9/apps/servers-registry/repo"
)

type LayerSelectionStatus uint8

const (
	LayerSelectionOK LayerSelectionStatus = iota
	LayerSelectionNoServer
	LayerSelectionNotFound
)

type LayerSelection struct {
	Status LayerSelectionStatus
	Server *repo.GameServer
}

type LayerStat struct {
	Players uint32
	Server  repo.GameServer
}

type Layer interface {
	Select(context.Context, uint32, uint32, uint32, string) (LayerSelection, error)
	BindGroup(context.Context, uint32, uint32, uint32, string) error
	Configuration(context.Context, uint32) (map[uint32]uint32, error)
	UpdateConfiguration(context.Context, uint32, map[uint32]uint32) error
	Stats(context.Context, uint32, uint32) (uint32, []LayerStat, error)
}

type layerService struct {
	servers GameServer
	store   repo.LayerStore
}

func NewLayer(servers GameServer, store repo.LayerStore) Layer {
	return &layerService{servers: servers, store: store}
}

func (l *layerService) Select(ctx context.Context, realmID, mapID, groupID uint32, preferredAlias string) (LayerSelection, error) {
	servers, err := l.servers.AvailableForMapAndRealm(ctx, mapID, realmID, false)
	if err != nil {
		return LayerSelection{}, err
	}
	if len(servers) == 0 {
		return LayerSelection{Status: LayerSelectionNoServer}, nil
	}
	if preferredAlias != "" {
		for i := range servers {
			if servers[i].Alias == preferredAlias {
				return LayerSelection{Status: LayerSelectionOK, Server: &servers[i]}, nil
			}
		}
		return LayerSelection{Status: LayerSelectionNotFound}, nil
	}
	if groupID == 0 {
		return LayerSelection{Status: LayerSelectionOK, Server: leastLoaded(servers)}, nil
	}

	boundID, err := l.store.GroupBinding(ctx, realmID, groupID, mapID)
	if err != nil {
		return LayerSelection{}, err
	}
	if server := serverByID(servers, boundID); server != nil {
		return LayerSelection{Status: LayerSelectionOK, Server: server}, nil
	}

	selected := leastLoaded(servers)
	var winner string
	if boundID == "" {
		winner, err = l.store.BindGroup(ctx, realmID, groupID, mapID, selected.ID)
	} else {
		winner, err = l.store.ReplaceGroupBinding(ctx, realmID, groupID, mapID, boundID, selected.ID)
	}
	if err != nil {
		return LayerSelection{}, err
	}
	if server := serverByID(servers, winner); server != nil {
		return LayerSelection{Status: LayerSelectionOK, Server: server}, nil
	}
	return LayerSelection{Status: LayerSelectionNoServer}, nil
}

func (l *layerService) BindGroup(ctx context.Context, realmID, groupID, mapID uint32, serverID string) error {
	if groupID == 0 || serverID == "" {
		return fmt.Errorf("group ID and gameserver ID are required")
	}
	servers, err := l.servers.AvailableForMapAndRealm(ctx, mapID, realmID, false)
	if err != nil {
		return err
	}
	if serverByID(servers, serverID) == nil {
		return fmt.Errorf("gameserver %s is not available for map %d", serverID, mapID)
	}
	return l.store.SetGroupBinding(ctx, realmID, groupID, mapID, serverID)
}

func (l *layerService) Configuration(ctx context.Context, realmID uint32) (map[uint32]uint32, error) {
	return l.store.Configuration(ctx, realmID)
}

func (l *layerService) UpdateConfiguration(ctx context.Context, realmID uint32, config map[uint32]uint32) error {
	for mapID, count := range config {
		if count == 0 {
			return fmt.Errorf("map %d has zero layers", mapID)
		}
	}
	if err := l.store.SetConfiguration(ctx, realmID, config); err != nil {
		return err
	}
	return l.servers.RedistributeRealm(ctx, realmID)
}

func (l *layerService) Stats(ctx context.Context, realmID, mapID uint32) (uint32, []LayerStat, error) {
	config, err := l.store.Configuration(ctx, realmID)
	if err != nil {
		return 0, nil, err
	}
	servers, err := l.servers.AvailableForMapAndRealm(ctx, mapID, realmID, false)
	if err != nil {
		return 0, nil, err
	}
	stats := make([]LayerStat, 0, len(servers))
	for _, server := range servers {
		stats = append(stats, LayerStat{Players: server.ActiveConnections, Server: server})
	}
	sort.Slice(stats, func(i, j int) bool { return stats[i].Server.Alias < stats[j].Server.Alias })
	configured := config[mapID]
	if configured == 0 {
		configured = 1
	}
	return configured, stats, nil
}

func leastLoaded(servers []repo.GameServer) *repo.GameServer {
	index := 0
	for i := 1; i < len(servers); i++ {
		if servers[i].ActiveConnections < servers[index].ActiveConnections ||
			(servers[i].ActiveConnections == servers[index].ActiveConnections && servers[i].ID < servers[index].ID) {
			index = i
		}
	}
	server := servers[index]
	return &server
}

func serverByID(servers []repo.GameServer, id string) *repo.GameServer {
	for i := range servers {
		if servers[i].ID == id {
			server := servers[i]
			return &server
		}
	}
	return nil
}
