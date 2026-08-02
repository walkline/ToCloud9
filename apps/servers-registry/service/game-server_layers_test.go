package service

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/walkline/ToCloud9/apps/servers-registry/repo"
)

func TestApplyLayerAssignmentsUsesDistinctCompatibleGameServersPerMap(t *testing.T) {
	servers := []repo.GameServer{
		{ID: "a", AvailableMaps: []uint32{1}, AssignedMapsToHandle: []uint32{1}},
		{ID: "b", AvailableMaps: []uint32{1}, AssignedMapsToHandle: []uint32{1}},
		{ID: "c", AvailableMaps: []uint32{1}},
		{ID: "incompatible", AvailableMaps: []uint32{530}, AssignedMapsToHandle: []uint32{1}},
	}

	applyLayerAssignments(servers, map[uint32]uint32{1: 2})

	var hosts []string
	for _, server := range servers {
		if containsMap(server.AssignedMapsToHandle, 1) {
			hosts = append(hosts, server.ID)
		}
	}
	require.Equal(t, []string{"a", "b"}, hosts)
}

func TestApplyLayerAssignmentsAreIndependentForEachMap(t *testing.T) {
	servers := []repo.GameServer{{ID: "a"}, {ID: "b"}, {ID: "c"}}

	applyLayerAssignments(servers, map[uint32]uint32{1: 2, 530: 2})

	require.True(t, containsMap(servers[0].AssignedMapsToHandle, 1))
	require.True(t, containsMap(servers[0].AssignedMapsToHandle, 530))
	require.Equal(t, 2, countMapHosts(servers, 1))
	require.Equal(t, 2, countMapHosts(servers, 530))
}

func countMapHosts(servers []repo.GameServer, mapID uint32) int {
	count := 0
	for _, server := range servers {
		if containsMap(server.AssignedMapsToHandle, mapID) {
			count++
		}
	}
	return count
}
