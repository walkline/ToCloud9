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
	updated map[uint32]uint32
}

func (s *layerGameServersStub) Register(context.Context, *repo.GameServer) error { return nil }
func (s *layerGameServersStub) AvailableForMapAndRealm(context.Context, uint32, uint32, bool) ([]repo.GameServer, error) {
	return append([]repo.GameServer(nil), s.servers...), nil
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
func (s *layerGameServersStub) UpdateMapLayerConfiguration(_ context.Context, _ uint32, config map[uint32]uint32) error {
	s.updated = cloneMapLayerCounts(config)
	return nil
}

func newLayerServiceForTest(maxPopulation, maxSwitches uint32, cooldown time.Duration) *layerService {
	return NewLayer(&layerGameServersStub{servers: []repo.GameServer{
		{ID: "layer-1", Address: "layer-1:8085", RealmID: 1, LayerID: 1, AssignedMapsToHandle: []uint32{0}},
		{ID: "layer-2", Address: "layer-2:8085", RealmID: 1, LayerID: 2, AssignedMapsToHandle: []uint32{0}},
	}}, LayerConfig{Enabled: true, MaxPopulation: maxPopulation, MaxSwitchesPerHour: maxSwitches, SwitchCooldown: cooldown}).(*layerService)
}

func TestLayerSelectActivatesNextLayerAtPopulationThreshold(t *testing.T) {
	layers := newLayerServiceForTest(1, 10, 0)

	first, err := layers.Select(context.Background(), 1, 0, 0, 0, 10, 0, LayerSelectLogin, "")
	require.NoError(t, err)
	require.Equal(t, uint32(1), first.LayerID)

	second, err := layers.Select(context.Background(), 1, 0, 0, 0, 20, 0, LayerSelectLogin, "")
	require.NoError(t, err)
	require.Equal(t, uint32(2), second.LayerID)
}

func TestMapLayerSelectionPreservesRegisteredLayerIDs(t *testing.T) {
	servers := &layerGameServersStub{servers: []repo.GameServer{
		{ID: "server-z", Address: "layer-42", RealmID: 1, LayerID: 42, ActiveConnections: 1},
		{ID: "server-a", Address: "layer-7", RealmID: 1, LayerID: 7, ActiveConnections: 0},
	}}
	layers := NewLayer(servers, LayerConfig{Enabled: true, MapLayers: map[uint32]uint32{1: 2}, RealmIDs: []uint32{1}}).(*layerService)

	selection, err := layers.Select(context.Background(), 1, 1, 0, 99, 100, 0, LayerSelectLogin, "")
	require.NoError(t, err)
	require.NotNil(t, selection.Server)
	require.Equal(t, uint32(7), selection.LayerID)

	bound, err := layers.Select(context.Background(), 1, 1, 0, 99, 101, 0, LayerSelectLogin, "")
	require.NoError(t, err)
	require.Equal(t, selection.Server.Address, bound.Server.Address)
	require.Equal(t, uint32(7), bound.LayerID)
}

func TestLayerSelectGroupJoinFollowsInviterAndEnforcesPolicy(t *testing.T) {
	layers := newLayerServiceForTest(2, 1, time.Minute)
	now := time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC)
	layers.now = func() time.Time { return now }

	inviter, err := layers.Select(context.Background(), 1, 0, 0, 0, 10, 0, LayerSelectLogin, "")
	require.NoError(t, err)
	layers.assignments[1][11] = &playerLayerAssignment{layerID: inviter.LayerID, serverAddress: inviter.Server.Address, online: true}
	invitee, err := layers.Select(context.Background(), 1, 0, 0, 0, 20, 0, LayerSelectLogin, "")
	require.NoError(t, err)
	require.NotEqual(t, inviter.LayerID, invitee.LayerID)
	delete(layers.assignments[1], 11)

	moved, err := layers.Select(context.Background(), 1, 0, 0, 0, 20, 10, LayerSelectGroupJoin, invitee.Server.Address)
	require.NoError(t, err)
	require.Equal(t, LayerSelectOK, moved.Status)
	require.Equal(t, inviter.LayerID, moved.LayerID)

	now = now.Add(30 * time.Second)
	throttled, err := layers.Select(context.Background(), 1, 0, 0, 0, 20, 0, LayerSelectManual, moved.Server.Address+"-other")
	require.NoError(t, err)
	require.Equal(t, LayerSelectThrottled, throttled.Status)

	layers.Release(1, 20)
	now = now.Add(31 * time.Second)
	limited, err := layers.Select(context.Background(), 1, 0, 0, 0, 20, 0, LayerSelectManual, "another-core")
	require.NoError(t, err)
	require.Equal(t, LayerSelectHourlyLimit, limited.Status, "logging out must not reset switch history")
}

func TestLayerLifecycleOnlyMovesOverCapacityPlayerAtSafeTransition(t *testing.T) {
	serverRepo := &layerGameServersStub{servers: []repo.GameServer{
		{ID: "l1", Address: "l1", RealmID: 1, LayerID: 1},
		{ID: "l2", Address: "l2", RealmID: 1, LayerID: 2},
	}}
	layers := NewLayer(serverRepo, LayerConfig{Enabled: true, MaxPopulation: 200}).(*layerService)
	now := time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC)
	layers.now = func() time.Time { return now }
	layers.assignments[1] = make(map[uint64]*playerLayerAssignment)
	for i := uint64(1); i <= 201; i++ {
		layers.assignments[1][i] = &playerLayerAssignment{layerID: 1, serverAddress: "l1", online: true, lastSeen: now}
	}

	action, err := layers.Poll(context.Background(), 1, 0, 0, 0, 201, "l1")
	require.NoError(t, err)
	require.Nil(t, action.Server, "population polling must never redirect an active player")

	action, err = layers.Select(context.Background(), 1, 0, 0, 0, 201, 0, LayerSelectMapChange, "l1")
	require.NoError(t, err)
	require.NotNil(t, action.Server)
	require.Equal(t, uint32(2), action.LayerID)
	layers.CompleteSwitch(1, 201, true)
	require.Equal(t, uint32(2), layers.assignments[1][201].layerID)
}

func TestLayerPollReconstructsAssignmentAfterRegistryRestart(t *testing.T) {
	layers := newLayerServiceForTest(10, 10, 0)

	action, err := layers.Poll(context.Background(), 1, 0, 12, 0, 77, "layer-2:8085")
	require.NoError(t, err)
	require.Nil(t, action.Server)

	assignment := layers.assignments[1][77]
	require.NotNil(t, assignment)
	require.True(t, assignment.online)
	require.Equal(t, uint32(2), assignment.layerID)
	require.Equal(t, "layer-2:8085", assignment.serverAddress)
	require.Equal(t, uint32(12), assignment.zoneID)

	require.Equal(t, LayerForceOK, layers.Force(context.Background(), 1, 77, 1, 0))
}

func TestLayerGroupJoinDoesNotExceedHardCap(t *testing.T) {
	servers := &layerGameServersStub{servers: []repo.GameServer{
		{ID: "l1", Address: "l1", RealmID: 1, LayerID: 1},
		{ID: "l2", Address: "l2", RealmID: 1, LayerID: 2},
	}}
	layers := NewLayer(servers, LayerConfig{
		Enabled: true, MaxPopulation: 200,
	}).(*layerService)
	layers.assignments[1] = make(map[uint64]*playerLayerAssignment)
	for i := uint64(1); i <= 220; i++ {
		layers.assignments[1][i] = &playerLayerAssignment{layerID: 1, serverAddress: "l1", online: true}
	}
	layers.assignments[1][221] = &playerLayerAssignment{layerID: 2, serverAddress: "l2", online: true}

	selection, err := layers.Select(context.Background(), 1, 0, 0, 0, 221, 1, LayerSelectGroupJoin, "l2")
	require.NoError(t, err)
	require.Equal(t, LayerSelectNoServer, selection.Status)
}

func TestPerMapLayersUseLeastLoadedCoreAndGroupAffinity(t *testing.T) {
	servers := &layerGameServersStub{servers: []repo.GameServer{
		{ID: "a", Address: "a", RealmID: 1, LayerID: 1, ActiveConnections: 8, AssignedMapsToHandle: []uint32{1}},
		{ID: "b", Address: "b", RealmID: 1, LayerID: 2, ActiveConnections: 2, AssignedMapsToHandle: []uint32{1}},
		{ID: "c", Address: "c", RealmID: 1, LayerID: 3, ActiveConnections: 5, AssignedMapsToHandle: []uint32{1}},
	}}
	layers := NewLayer(servers, LayerConfig{Enabled: true, RealmIDs: []uint32{1}, MapLayers: map[uint32]uint32{1: 3}}).(*layerService)
	first, err := layers.Select(context.Background(), 1, 1, 0, 77, 10, 0, LayerSelectLogin, "")
	require.NoError(t, err)
	require.Equal(t, "b", first.Server.Address)

	servers.servers[1].ActiveConnections = 20
	member, err := layers.Select(context.Background(), 1, 1, 0, 77, 11, 0, LayerSelectLogin, "")
	require.NoError(t, err)
	require.Equal(t, "b", member.Server.Address, "group binding wins over current load")

	ungrouped, err := layers.Select(context.Background(), 1, 1, 0, 0, 12, 0, LayerSelectLogin, "")
	require.NoError(t, err)
	require.Equal(t, "c", ungrouped.Server.Address)
}

func TestPerMapLayerStatsUseMapLayerIDsAndMapPopulation(t *testing.T) {
	servers := &layerGameServersStub{servers: []repo.GameServer{
		{ID: "a", Address: "a", RealmID: 1, LayerID: 2, AssignedMapsToHandle: []uint32{1}},
		{ID: "b", Address: "b", RealmID: 1, LayerID: 1, AssignedMapsToHandle: []uint32{1}},
	}}
	layers := NewLayer(servers, LayerConfig{Enabled: true, MaxPopulation: 1, RealmIDs: []uint32{1}, MapLayers: map[uint32]uint32{1: 2}}).(*layerService)
	layers.assignments[1] = map[uint64]*playerLayerAssignment{
		10: {layerID: 1, mapID: 1, online: true},
		11: {layerID: 1, mapID: 1, online: true},
		12: {layerID: 2, mapID: 1, online: true},
		13: {layerID: 1, mapID: 571, online: true},
	}

	stats, err := layers.Stats(context.Background(), 1, 1, 10)
	require.NoError(t, err)
	require.Equal(t, uint32(1), stats.CurrentLayerID)
	require.Equal(t, []LayerStat{
		{LayerID: 1, CurrentPlayers: 2, ReadyGameServers: 1},
		{LayerID: 2, CurrentPlayers: 1, ReadyGameServers: 1},
	}, stats.Layers)
}

func TestPerMapPollRevivesReleasedPlayerForForceSwitch(t *testing.T) {
	servers := &layerGameServersStub{servers: []repo.GameServer{
		{ID: "a", Address: "a", RealmID: 1, LayerID: 1, AssignedMapsToHandle: []uint32{1}},
		{ID: "b", Address: "b", RealmID: 1, LayerID: 2, AssignedMapsToHandle: []uint32{1}},
	}}
	layers := NewLayer(servers, LayerConfig{Enabled: true, RealmIDs: []uint32{1}, MapLayers: map[uint32]uint32{1: 2}}).(*layerService)
	layers.assignments[1] = map[uint64]*playerLayerAssignment{10: {layerID: 2, serverAddress: "b", online: true}}
	layers.Release(1, 10)

	selection, err := layers.Poll(context.Background(), 1, 1, 0, 0, 10, "a")
	require.NoError(t, err)
	require.Nil(t, selection.Server)
	require.True(t, layers.assignments[1][10].online)
	require.Equal(t, uint32(1), layers.assignments[1][10].layerID)
	require.Equal(t, LayerForceOK, layers.Force(context.Background(), 1, 10, 2, 1))
}

func TestPerMapPollDoesNotSwitchLayersWhenCurrentCoreIsMissing(t *testing.T) {
	servers := &layerGameServersStub{servers: []repo.GameServer{
		{ID: "b", Address: "layer-2", RealmID: 1, LayerID: 2, AssignedMapsToHandle: []uint32{1}},
	}}
	layers := NewLayer(servers, LayerConfig{Enabled: true, RealmIDs: []uint32{1}, MapLayers: map[uint32]uint32{1: 2}}).(*layerService)
	layers.assignments[1] = map[uint64]*playerLayerAssignment{
		10: {layerID: 1, serverAddress: "layer-1", mapID: 1, online: true},
	}

	selection, err := layers.Poll(context.Background(), 1, 1, 0, 0, 10, "layer-1")
	require.NoError(t, err)
	require.Nil(t, selection.Server)
	require.Equal(t, uint32(1), layers.assignments[1][10].layerID)
}

func TestWholeGroupReturningToMapGetsFreshLeastPopulatedLayer(t *testing.T) {
	servers := &layerGameServersStub{servers: []repo.GameServer{
		{ID: "a", Address: "layer-1", RealmID: 1, LayerID: 1, AssignedMapsToHandle: []uint32{1}},
		{ID: "b", Address: "layer-2", RealmID: 1, LayerID: 2, AssignedMapsToHandle: []uint32{1}},
	}}
	layers := NewLayer(servers, LayerConfig{Enabled: true, RealmIDs: []uint32{1}, MapLayers: map[uint32]uint32{1: 2}}).(*layerService)
	layers.assignments[1] = map[uint64]*playerLayerAssignment{
		10: {layerID: 1, mapID: 33, groupID: 77, online: true},
		11: {layerID: 1, mapID: 33, groupID: 77, online: true},
		12: {layerID: 1, mapID: 1, online: true},
	}
	layers.groupBindings[groupMapKey{1, 77, 1}] = groupMapBinding{"layer-1", 1}

	first, err := layers.Select(context.Background(), 1, 1, 0, 77, 10, 0, LayerSelectMapChange, "instance")
	require.NoError(t, err)
	require.Equal(t, uint32(2), first.LayerID)
	require.Equal(t, "layer-2", first.Server.Address)

	second, err := layers.Select(context.Background(), 1, 1, 0, 77, 11, 0, LayerSelectMapChange, "instance")
	require.NoError(t, err)
	require.Equal(t, uint32(2), second.LayerID)
	require.Equal(t, "layer-2", second.Server.Address)
}

func TestReturningGroupKeepsBindingWhenMemberRemainedOnMap(t *testing.T) {
	servers := &layerGameServersStub{servers: []repo.GameServer{
		{ID: "a", Address: "layer-1", RealmID: 1, LayerID: 1, AssignedMapsToHandle: []uint32{1}},
		{ID: "b", Address: "layer-2", RealmID: 1, LayerID: 2, AssignedMapsToHandle: []uint32{1}},
	}}
	layers := NewLayer(servers, LayerConfig{Enabled: true, RealmIDs: []uint32{1}, MapLayers: map[uint32]uint32{1: 2}}).(*layerService)
	layers.assignments[1] = map[uint64]*playerLayerAssignment{
		10: {layerID: 1, mapID: 33, groupID: 77, online: true},
		11: {layerID: 1, mapID: 1, groupID: 77, online: true},
	}
	layers.groupBindings[groupMapKey{1, 77, 1}] = groupMapBinding{"layer-1", 1}

	selection, err := layers.Select(context.Background(), 1, 1, 0, 77, 10, 0, LayerSelectMapChange, "instance")
	require.NoError(t, err)
	require.Equal(t, uint32(1), selection.LayerID)
}

func TestRuntimeMapLayerConfigurationRedistributesAndClearsBindings(t *testing.T) {
	servers := &layerGameServersStub{servers: []repo.GameServer{{ID: "a", Address: "a", RealmID: 1, AssignedMapsToHandle: []uint32{1}}}}
	layers := NewLayer(servers, LayerConfig{Enabled: true, RealmIDs: []uint32{1}, MapLayers: map[uint32]uint32{1: 2}}).(*layerService)
	layers.groupBindings[groupMapKey{1, 9, 1}] = groupMapBinding{"a", 1}
	require.NoError(t, layers.UpdateMapConfiguration(context.Background(), 1, map[uint32]uint32{1: 3, 571: 2}))
	require.Equal(t, map[uint32]uint32{1: 3, 571: 2}, servers.updated)
	require.Empty(t, layers.groupBindings)
}
