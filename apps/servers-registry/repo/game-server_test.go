package repo

import (
	"fmt"
	"regexp"
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
	// modifier-name-code (2 base36 chars)
	require.Equal(t, 3, len(strings.Split(first, "-")))
	require.Regexp(t, regexp.MustCompile(`^[a-z]+-[a-z]+-[0-9a-z]{2}$`), first)
	require.LessOrEqual(t, len(first), 18)

	seen := map[string]string{}
	for i := 0; i < 40; i++ {
		addr := fmt.Sprintf("10.0.0.%d:8085", i+1)
		alias := repository.generateAlias(addr)
		if other, ok := seen[alias]; ok {
			t.Fatalf("alias collision %q for %s and %s", alias, other, addr)
		}
		seen[alias] = addr
	}
}

func TestAliasPartsAreShortAndFamiliar(t *testing.T) {
	require.Equal(t, 12, len(aliasModifiers))
	require.GreaterOrEqual(t, len(aliasRaidBosses), 30)
	require.LessOrEqual(t, len(aliasRaidBosses), 50)

	for _, modifier := range aliasModifiers {
		require.Regexp(t, regexp.MustCompile(`^[a-z]+$`), modifier)
		require.LessOrEqual(t, len(modifier), 5)
	}
	for _, name := range aliasRaidBosses {
		require.Regexp(t, regexp.MustCompile(`^[a-z]+$`), name)
		require.LessOrEqual(t, len(name), 8)
	}
}
