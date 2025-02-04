package service

import (
	"context"
	"fmt"
	"sync"

	matchmaking "github.com/walkline/ToCloud9/apps/matchmakingserver"
	pbServ "github.com/walkline/ToCloud9/gen/servers-registry/pb"
	"github.com/walkline/ToCloud9/shared/events"
)

type CrossRealmNodesObserver interface {
	OnNoCrossRealmNodesAvailable()
	OnNoCrossRealmNodesUnAvailable()
}

type CrossRealmNode struct {
	ID            string
	AvailableMaps []uint32
}

type CrossRealmNodesTracker struct {
	activeCrossRealmNodes map[string]CrossRealmNode

	observer CrossRealmNodesObserver

	m sync.RWMutex
}

func NewCrossRealmNodesTracker(serversRegistry pbServ.ServersRegistryServiceClient) (*CrossRealmNodesTracker, error) {
	gameServers, err := serversRegistry.ListGameServersForRealm(context.Background(), &pbServ.ListGameServersForRealmRequest{
		Api:          matchmaking.SupportedServerRegistryVer,
		RealmID:      0,
		IsCrossRealm: true,
	})

	if err != nil {
		return nil, fmt.Errorf("cannot get cross-realm game servers: %w", err)
	}

	activeCrossRealmNodes := map[string]CrossRealmNode{}
	for _, server := range gameServers.GameServers {
		activeCrossRealmNodes[server.ID] = CrossRealmNode{
			ID:            server.ID,
			AvailableMaps: server.AvailableMaps,
		}
	}

	return &CrossRealmNodesTracker{
		activeCrossRealmNodes: activeCrossRealmNodes,
	}, nil
}

func (t *CrossRealmNodesTracker) SetObserver(observer CrossRealmNodesObserver) {
	t.observer = observer
}

func (t *CrossRealmNodesTracker) OnGameServerAdded(p *events.ServerRegistryEventGSAddedPayload) {
	if !p.GameServer.IsCrossRealm {
		return
	}

	t.m.Lock()

	lebBefore := len(t.activeCrossRealmNodes)

	t.activeCrossRealmNodes[p.GameServer.ID] = CrossRealmNode{
		ID:            p.GameServer.ID,
		AvailableMaps: p.GameServer.AvailableMaps,
	}

	t.m.Unlock()

	if lebBefore == 0 {
		t.observer.OnNoCrossRealmNodesAvailable()
	}
}

func (t *CrossRealmNodesTracker) OnGameServerRemoved(p *events.ServerRegistryEventGSRemovedPayload) {
	if !p.GameServer.IsCrossRealm {
		return
	}

	t.m.Lock()

	if len(t.activeCrossRealmNodes) == 0 {
		t.m.Unlock()
		return
	}

	delete(t.activeCrossRealmNodes, p.GameServer.ID)

	nodesUnavailable := len(t.activeCrossRealmNodes) == 0

	t.m.Unlock()

	if nodesUnavailable {
		t.observer.OnNoCrossRealmNodesUnAvailable()
	}
}

func (t *CrossRealmNodesTracker) IsCrossRealmNodeAvailable() bool {
	t.m.RLock()
	defer t.m.RUnlock()

	return len(t.activeCrossRealmNodes) > 0
}
