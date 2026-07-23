package service

import (
	"context"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/walkline/ToCloud9/apps/servers-registry/repo"
)

type layerStoreStub struct {
	mu       sync.Mutex
	config   map[uint32]map[uint32]uint32
	bindings map[[3]uint32]string
}

func newLayerStoreStub() *layerStoreStub {
	return &layerStoreStub{config: map[uint32]map[uint32]uint32{}, bindings: map[[3]uint32]string{}}
}
func (s *layerStoreStub) Configuration(_ context.Context, realm uint32) (map[uint32]uint32, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	result := map[uint32]uint32{}
	for k, v := range s.config[realm] {
		result[k] = v
	}
	return result, nil
}
func (s *layerStoreStub) SetConfiguration(_ context.Context, realm uint32, value map[uint32]uint32) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.config[realm] = value
	return nil
}
func (s *layerStoreStub) GroupBinding(_ context.Context, realm, group, mapID uint32) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.bindings[[3]uint32{realm, group, mapID}], nil
}
func (s *layerStoreStub) BindGroup(_ context.Context, realm, group, mapID uint32, server string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := [3]uint32{realm, group, mapID}
	if s.bindings[key] == "" {
		s.bindings[key] = server
	}
	return s.bindings[key], nil
}
func (s *layerStoreStub) SetGroupBinding(_ context.Context, realm, group, mapID uint32, server string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.bindings[[3]uint32{realm, group, mapID}] = server
	return nil
}
func (s *layerStoreStub) ReplaceGroupBinding(_ context.Context, realm, group, mapID uint32, stale, server string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := [3]uint32{realm, group, mapID}
	if s.bindings[key] == stale {
		s.bindings[key] = server
	}
	return s.bindings[key], nil
}
func (s *layerStoreStub) LockRealm(context.Context, uint32) (func(), error) {
	return func() {}, nil
}

type layerServersStub struct{ servers []repo.GameServer }

func (s *layerServersStub) Register(context.Context, *repo.GameServer) error { return nil }
func (s *layerServersStub) AvailableForMapAndRealm(context.Context, uint32, uint32, bool) ([]repo.GameServer, error) {
	return append([]repo.GameServer(nil), s.servers...), nil
}
func (s *layerServersStub) RandomServerForRealm(context.Context, uint32) (*repo.GameServer, error) {
	return nil, nil
}
func (s *layerServersStub) ListForRealm(context.Context, uint32) ([]repo.GameServer, error) {
	return append([]repo.GameServer(nil), s.servers...), nil
}
func (s *layerServersStub) ListOfCrossRealms(context.Context) ([]repo.GameServer, error) {
	return nil, nil
}
func (s *layerServersStub) ListAll(context.Context) ([]repo.GameServer, error) { return nil, nil }
func (s *layerServersStub) MapsLoadedForServer(context.Context, string, []uint32) (*repo.GameServer, error) {
	return nil, nil
}
func (s *layerServersStub) RedistributeRealm(context.Context, uint32) error {
	return nil
}

func TestRegistryReplicasShareAtomicGroupBinding(t *testing.T) {
	store := newLayerStoreStub()
	require.NoError(t, store.SetConfiguration(context.Background(), 1, map[uint32]uint32{1: 2}))
	servers := &layerServersStub{servers: []repo.GameServer{
		{ID: "layer-1", Alias: "thrall-onyxia-a", ActiveConnections: 2},
		{ID: "layer-2", Alias: "jaina-arthas-b", ActiveConnections: 1},
	}}
	replicaA, replicaB := NewLayer(servers, store), NewLayer(servers, store)

	var first, second LayerSelection
	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); first, _ = replicaA.Select(context.Background(), 1, 1, 77, "") }()
	go func() { defer wg.Done(); second, _ = replicaB.Select(context.Background(), 1, 1, 77, "") }()
	wg.Wait()

	require.Equal(t, LayerSelectionOK, first.Status)
	require.Equal(t, first.Server.ID, second.Server.ID)
	require.Equal(t, "layer-2", first.Server.ID)
}

func TestStaleGroupBindingMovesToAvailableServer(t *testing.T) {
	store := newLayerStoreStub()
	require.NoError(t, store.SetConfiguration(context.Background(), 1, map[uint32]uint32{1: 2}))
	require.NoError(t, store.SetGroupBinding(context.Background(), 1, 77, 1, "gone"))
	layers := NewLayer(&layerServersStub{servers: []repo.GameServer{{ID: "ready", Alias: "ready-alias"}}}, store)

	selection, err := layers.Select(context.Background(), 1, 1, 77, "")
	require.NoError(t, err)
	require.Equal(t, "ready", selection.Server.ID)
	bound, err := store.GroupBinding(context.Background(), 1, 77, 1)
	require.NoError(t, err)
	require.Equal(t, "ready", bound)
}

func TestConfigurationIsSharedAcrossRegistryReplicas(t *testing.T) {
	store := newLayerStoreStub()
	servers := &layerServersStub{}
	replicaA, replicaB := NewLayer(servers, store), NewLayer(servers, store)

	require.NoError(t, replicaA.UpdateConfiguration(context.Background(), 1, map[uint32]uint32{1: 2, 571: 3}))
	config, err := replicaB.Configuration(context.Background(), 1)
	require.NoError(t, err)
	require.Equal(t, map[uint32]uint32{1: 2, 571: 3}, config)
}

func TestPreferredGameServerSelectionDoesNotChangeGroupBinding(t *testing.T) {
	store := newLayerStoreStub()
	require.NoError(t, store.SetConfiguration(context.Background(), 1, map[uint32]uint32{1: 2}))
	require.NoError(t, store.SetGroupBinding(context.Background(), 1, 77, 1, "layer-1"))
	layers := NewLayer(&layerServersStub{servers: []repo.GameServer{{ID: "layer-1", Alias: "thrall-onyxia-a"}, {ID: "layer-2", Alias: "jaina-arthas-b"}}}, store)

	selection, err := layers.Select(context.Background(), 1, 1, 77, "jaina-arthas-b")
	require.NoError(t, err)
	require.Equal(t, "layer-2", selection.Server.ID)
	bound, err := store.GroupBinding(context.Background(), 1, 77, 1)
	require.NoError(t, err)
	require.Equal(t, "layer-1", bound)
}
