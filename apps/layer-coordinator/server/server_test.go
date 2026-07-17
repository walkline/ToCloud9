package server

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	pbRegistry "github.com/walkline/ToCloud9/gen/servers-registry/pb"
)

type registryStub struct {
	pbRegistry.ServersRegistryServiceClient
	selected *pbRegistry.Server
}

func (s *registryStub) SelectGameServerForPlayer(context.Context, *pbRegistry.SelectGameServerForPlayerRequest, ...grpc.CallOption) (*pbRegistry.SelectGameServerForPlayerResponse, error) {
	return &pbRegistry.SelectGameServerForPlayerResponse{Status: pbRegistry.SelectGameServerForPlayerResponse_OK, GameServer: s.selected, LayerID: s.selected.LayerID}, nil
}
func (s *registryStub) ListGameServersForRealm(context.Context, *pbRegistry.ListGameServersForRealmRequest, ...grpc.CallOption) (*pbRegistry.ListGameServersResponse, error) {
	return &pbRegistry.ListGameServersResponse{GameServers: []*pbRegistry.GameServerDetailed{{Address: s.selected.Address, LayerID: s.selected.LayerID, AssignedMaps: []uint32{1}}}}, nil
}
func (s *registryStub) GetMapLayerConfiguration(context.Context, *pbRegistry.GetMapLayerConfigurationRequest, ...grpc.CallOption) (*pbRegistry.GetMapLayerConfigurationResponse, error) {
	return &pbRegistry.GetMapLayerConfigurationResponse{Maps: []*pbRegistry.MapLayerConfiguration{{MapID: 1, LayerCount: 2}}}, nil
}
func (s *registryStub) AvailableGameServersForMapAndRealm(context.Context, *pbRegistry.AvailableGameServersForMapAndRealmRequest, ...grpc.CallOption) (*pbRegistry.AvailableGameServersForMapAndRealmResponse, error) {
	return &pbRegistry.AvailableGameServersForMapAndRealmResponse{GameServers: []*pbRegistry.Server{s.selected}}, nil
}

func TestCoordinatorOwnsSwitchLifecycle(t *testing.T) {
	registry := &registryStub{selected: &pbRegistry.Server{Address: "layer-42", LayerID: 42}}
	coordinator := New(registry)

	selection, err := coordinator.SelectGameServerForPlayer(context.Background(), &pbRegistry.SelectGameServerForPlayerRequest{RealmID: 1, MapID: 1, PlayerGUID: 7, CurrentGameServerAddress: "layer-7"})
	require.NoError(t, err)
	require.Equal(t, uint32(42), selection.LayerID)
	require.Equal(t, "layer-42", coordinator.players[1][7].pendingServerAddress)

	_, err = coordinator.CompletePlayerLayerSwitch(context.Background(), &pbRegistry.CompletePlayerLayerSwitchRequest{RealmID: 1, PlayerGUID: 7, Success: true})
	require.NoError(t, err)
	require.Equal(t, uint32(42), coordinator.players[1][7].layerID)
	require.Empty(t, coordinator.players[1][7].pendingServerAddress)

	_, err = coordinator.ReleasePlayerLayer(context.Background(), &pbRegistry.ReleasePlayerLayerRequest{RealmID: 1, PlayerGUID: 7})
	require.NoError(t, err)
	require.False(t, coordinator.players[1][7].online)
}

func TestCoordinatorForceUsesStableRegisteredLayerID(t *testing.T) {
	registry := &registryStub{selected: &pbRegistry.Server{Address: "layer-42", LayerID: 42}}
	coordinator := New(registry)
	coordinator.players[1] = map[uint64]*playerState{7: {online: true, mapID: 1, layerID: 7}}

	response, err := coordinator.ForcePlayerLayer(context.Background(), &pbRegistry.ForcePlayerLayerRequest{RealmID: 1, MapID: 1, PlayerGUID: 7, LayerID: 42})
	require.NoError(t, err)
	require.Equal(t, pbRegistry.ForcePlayerLayerResponse_OK, response.Status)
	require.Equal(t, uint32(42), coordinator.players[1][7].pendingLayerID)
}
