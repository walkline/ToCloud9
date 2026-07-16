package session

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestLayerSwitchSafe(t *testing.T) {
	now := time.Now()
	s := &GameSession{character: &LoggedInCharacter{Map: 0, CurHP: 100}}
	require.True(t, s.layerSwitchSafe(now))

	s.layerSafety.inCombat = true
	require.False(t, s.layerSwitchSafe(now))
	s.layerSafety.inCombat = false

	s.layerSafety.lastDamagedAt = now.Add(-29 * time.Second)
	require.False(t, s.layerSwitchSafe(now))
	s.layerSafety.lastDamagedAt = now.Add(-30 * time.Second)
	require.True(t, s.layerSwitchSafe(now))

	s.character.Map = 33 // Shadowfang Keep (instance map)
	require.False(t, s.layerSwitchSafe(now))
	s.character.Map, s.character.CurHP = 571, 0
	require.False(t, s.layerSwitchSafe(now))
}

func TestLayerSwitchSafeTransientInteractions(t *testing.T) {
	for name, apply := range map[string]func(*layerSafetyState){
		"falling":   func(v *layerSafetyState) { v.falling = true },
		"looting":   func(v *layerSafetyState) { v.looting = true },
		"trading":   func(v *layerSafetyState) { v.trading = true },
		"casting":   func(v *layerSafetyState) { v.casting = true },
		"releasing": func(v *layerSafetyState) { v.releasing = true },
	} {
		t.Run(name, func(t *testing.T) {
			s := &GameSession{character: &LoggedInCharacter{Map: 1, CurHP: 1}}
			apply(&s.layerSafety)
			require.False(t, s.layerSwitchSafe(time.Now()))
		})
	}
}
