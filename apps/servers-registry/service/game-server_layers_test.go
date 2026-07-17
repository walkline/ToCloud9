package service

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/walkline/ToCloud9/apps/servers-registry/repo"
)

func TestAssignLayeredMapUsesDistinctEligibleCores(t *testing.T) {
	servers := []repo.GameServer{
		{ID: "a", ActiveConnections: 5, AssignedMapsToHandle: []uint32{0, 1}},
		{ID: "b", ActiveConnections: 1, AssignedMapsToHandle: []uint32{2}},
		{ID: "c", ActiveConnections: 3, AssignedMapsToHandle: []uint32{3}},
	}
	previous := []repo.GameServer{servers[0].Copy(), servers[1].Copy(), servers[2].Copy()}
	result := assignLayeredMap(servers, previous, 1, 2)
	require.True(t, containsMap(result[0].AssignedMapsToHandle, 1))
	require.True(t, containsMap(result[1].AssignedMapsToHandle, 1))
	require.False(t, containsMap(result[2].AssignedMapsToHandle, 1))
}
