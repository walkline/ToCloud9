package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/walkline/ToCloud9/apps/servers-registry/repo"
)

type layerGameServersStub struct {
	servers []repo.GameServer
}

func (s *layerGameServersStub) Register(context.Context, *repo.GameServer) error { return nil }
func (s *layerGameServersStub) AvailableForMapAndRealm(context.Context, uint32, uint32, bool) ([]repo.GameServer, error) {
	return append([]repo.GameServer(nil), s.servers...), nil
}

type layerProvisionerStub struct{ ensured, deleted []uint32 }

func (p *layerProvisionerStub) EnsureLayer(_ context.Context, _, layerID uint32) error {
	p.ensured = append(p.ensured, layerID)
	return nil
}
func (p *layerProvisionerStub) DeleteLayer(_ context.Context, _, layerID uint32) error {
	p.deleted = append(p.deleted, layerID)
	return nil
}
func (s *layerGameServersStub) RandomServerForRealm(context.Context, uint32) (*repo.GameServer, error) {
	return nil, nil
}
func (s *layerGameServersStub) ListForRealm(context.Context, uint32) ([]repo.GameServer, error) {
	return s.servers, nil
}
func (s *layerGameServersStub) ListOfCrossRealms(context.Context) ([]repo.GameServer, error) {
	return nil, nil
}
func (s *layerGameServersStub) ListAll(context.Context) ([]repo.GameServer, error) {
	return s.servers, nil
}
func (s *layerGameServersStub) MapsLoadedForServer(context.Context, string, []uint32) (*repo.GameServer, error) {
	return nil, nil
}

func newLayerServiceForTest(maxPopulation, maxSwitches uint32, cooldown time.Duration) *layerService {
	return NewLayer(&layerGameServersStub{servers: []repo.GameServer{
		{ID: "layer-1", Address: "layer-1:8085", RealmID: 1, LayerID: 1, AssignedMapsToHandle: []uint32{0}},
		{ID: "layer-2", Address: "layer-2:8085", RealmID: 1, LayerID: 2, AssignedMapsToHandle: []uint32{0}},
	}}, LayerConfig{Enabled: true, MaxPopulation: maxPopulation, MaxSwitchesPerHour: maxSwitches, SwitchCooldown: cooldown}).(*layerService)
}

func TestLayerSelectActivatesNextLayerAtPopulationThreshold(t *testing.T) {
	layers := newLayerServiceForTest(1, 10, 0)

	first, err := layers.Select(context.Background(), 1, 0, 0, 10, 0, LayerSelectLogin, "")
	require.NoError(t, err)
	require.Equal(t, uint32(1), first.LayerID)

	second, err := layers.Select(context.Background(), 1, 0, 0, 20, 0, LayerSelectLogin, "")
	require.NoError(t, err)
	require.Equal(t, uint32(2), second.LayerID)
}

func TestLayerSelectGroupJoinFollowsInviterAndEnforcesPolicy(t *testing.T) {
	layers := newLayerServiceForTest(1, 1, time.Minute)
	now := time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC)
	layers.now = func() time.Time { return now }

	inviter, err := layers.Select(context.Background(), 1, 0, 0, 10, 0, LayerSelectLogin, "")
	require.NoError(t, err)
	invitee, err := layers.Select(context.Background(), 1, 0, 0, 20, 0, LayerSelectLogin, "")
	require.NoError(t, err)
	require.NotEqual(t, inviter.LayerID, invitee.LayerID)

	moved, err := layers.Select(context.Background(), 1, 0, 0, 20, 10, LayerSelectGroupJoin, invitee.Server.Address)
	require.NoError(t, err)
	require.Equal(t, LayerSelectOK, moved.Status)
	require.Equal(t, inviter.LayerID, moved.LayerID)

	now = now.Add(30 * time.Second)
	throttled, err := layers.Select(context.Background(), 1, 0, 0, 20, 0, LayerSelectManual, moved.Server.Address+"-other")
	require.NoError(t, err)
	require.Equal(t, LayerSelectThrottled, throttled.Status)

	layers.Release(1, 20)
	now = now.Add(31 * time.Second)
	limited, err := layers.Select(context.Background(), 1, 0, 0, 20, 0, LayerSelectManual, "another-core")
	require.NoError(t, err)
	require.Equal(t, LayerSelectHourlyLimit, limited.Status, "logging out must not reset switch history")
}

func TestLayerLifecycleProvisionsThenDrainsAndDeletesExcessLayer(t *testing.T) {
	serverRepo := &layerGameServersStub{servers: []repo.GameServer{{ID: "l1", Address: "l1", RealmID: 1, LayerID: 1}}}
	provisioner := &layerProvisionerStub{}
	now := time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC)
	layers := NewLayer(serverRepo, LayerConfig{
		Enabled: true, MaxPopulation: 200, MinLayers: 1, MaxLayers: 10,
		RealmIDs: []uint32{1}, Scopes: []LayerScope{{Name: "human-start", ZoneIDs: []uint32{12}, MaxPopulation: 200}},
		Provisioner: provisioner,
	}).(*layerService)
	layers.now = func() time.Time { return now }
	layers.assignments[1] = make(map[uint64]*playerLayerAssignment)
	for i := uint64(1); i <= 500; i++ {
		layerID := uint32(1)
		if i > 200 {
			layerID = 2
		}
		if i > 400 {
			layerID = 3
		}
		layers.assignments[1][i] = &playerLayerAssignment{layerID: layerID, serverAddress: "old", online: true, mapID: 0, zoneID: 12, lastSeen: now}
	}

	layers.reconcile(context.Background())
	require.ElementsMatch(t, []uint32{2, 3}, provisioner.ensured)

	serverRepo.servers = []repo.GameServer{
		{ID: "l1", Address: "l1", RealmID: 1, LayerID: 1},
		{ID: "l2", Address: "l2", RealmID: 1, LayerID: 2},
		{ID: "l3", Address: "l3", RealmID: 1, LayerID: 3},
	}
	// Leave 300 players online, including all 100 players currently on layer 3.
	for i := uint64(101); i <= 300; i++ {
		layers.assignments[1][i].online = false
	}
	layers.reconcile(context.Background())
	layers.reconcile(context.Background())
	require.False(t, layers.draining[1][3].IsZero())

	for i := uint64(401); i <= 500; i++ {
		action, err := layers.Poll(context.Background(), 1, 0, 12, i, "l3")
		require.NoError(t, err)
		require.NotNil(t, action.Server)
		require.Less(t, action.LayerID, uint32(3))
		layers.CompleteSwitch(1, i, true)
	}
	layers.reconcile(context.Background())
	require.Contains(t, provisioner.deleted, uint32(3))
}

func TestLayerLifecycleMovesOverflowPlayerWhenNewLayerBecomesReady(t *testing.T) {
	serverRepo := &layerGameServersStub{servers: []repo.GameServer{
		{ID: "l1", Address: "l1", RealmID: 1, LayerID: 1},
		{ID: "l2", Address: "l2", RealmID: 1, LayerID: 2},
	}}
	layers := NewLayer(serverRepo, LayerConfig{Enabled: true, MaxPopulation: 200, MinLayers: 1, MaxLayers: 10}).(*layerService)
	now := time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC)
	layers.now = func() time.Time { return now }
	layers.assignments[1] = make(map[uint64]*playerLayerAssignment)
	for i := uint64(1); i <= 201; i++ {
		layers.assignments[1][i] = &playerLayerAssignment{layerID: 1, serverAddress: "l1", online: true, lastSeen: now}
	}

	action, err := layers.Poll(context.Background(), 1, 0, 0, 201, "l1")
	require.NoError(t, err)
	require.NotNil(t, action.Server)
	require.Equal(t, uint32(2), action.LayerID)
	layers.CompleteSwitch(1, 201, true)
	require.Equal(t, uint32(2), layers.assignments[1][201].layerID)
}
