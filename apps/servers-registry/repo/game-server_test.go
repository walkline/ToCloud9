package repo

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGameServerCopyDoesNotShareMapSlices(t *testing.T) {
	original := GameServer{AvailableMaps: []uint32{1}, AssignedMapsToHandle: []uint32{2}, AssignedButPendingMaps: []uint32{3}}
	copy := original.Copy()
	copy.AvailableMaps[0], copy.AssignedMapsToHandle[0], copy.AssignedButPendingMaps[0] = 10, 20, 30

	require.Equal(t, []uint32{1}, original.AvailableMaps)
	require.Equal(t, []uint32{2}, original.AssignedMapsToHandle)
	require.Equal(t, []uint32{3}, original.AssignedButPendingMaps)
}

func TestGameServerAliasIsDeterministicAndReadable(t *testing.T) {
	repository := &gameServerRedisRepo{}
	first := repository.generateAlias("10.0.0.1:9601")

	require.Equal(t, first, repository.generateAlias("10.0.0.1:9601"))
	require.NotEqual(t, first, repository.generateAlias("10.0.0.2:9601"))
	require.Len(t, strings.Split(first, "-"), 3)
}
