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

	first, err := layers.Select(context.Background(), 1, 0, 10, 0, LayerSelectLogin, "")
	require.NoError(t, err)
	require.Equal(t, uint32(1), first.LayerID)

	second, err := layers.Select(context.Background(), 1, 0, 20, 0, LayerSelectLogin, "")
	require.NoError(t, err)
	require.Equal(t, uint32(2), second.LayerID)
}

func TestLayerSelectGroupJoinFollowsInviterAndEnforcesPolicy(t *testing.T) {
	layers := newLayerServiceForTest(1, 1, time.Minute)
	now := time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC)
	layers.now = func() time.Time { return now }

	inviter, err := layers.Select(context.Background(), 1, 0, 10, 0, LayerSelectLogin, "")
	require.NoError(t, err)
	invitee, err := layers.Select(context.Background(), 1, 0, 20, 0, LayerSelectLogin, "")
	require.NoError(t, err)
	require.NotEqual(t, inviter.LayerID, invitee.LayerID)

	moved, err := layers.Select(context.Background(), 1, 0, 20, 10, LayerSelectGroupJoin, invitee.Server.Address)
	require.NoError(t, err)
	require.Equal(t, LayerSelectOK, moved.Status)
	require.Equal(t, inviter.LayerID, moved.LayerID)

	now = now.Add(30 * time.Second)
	throttled, err := layers.Select(context.Background(), 1, 0, 20, 0, LayerSelectManual, moved.Server.Address+"-other")
	require.NoError(t, err)
	require.Equal(t, LayerSelectThrottled, throttled.Status)

	layers.Release(1, 20)
	now = now.Add(31 * time.Second)
	limited, err := layers.Select(context.Background(), 1, 0, 20, 0, LayerSelectManual, "another-core")
	require.NoError(t, err)
	require.Equal(t, LayerSelectHourlyLimit, limited.Status, "logging out must not reset switch history")
}
