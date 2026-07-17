package session

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestClusterRecoverySuppressesLayerOrchestration(t *testing.T) {
	s := &GameSession{
		layeringEnabled:         true,
		worldRecoveryInProgress: true,
		character:               &LoggedInCharacter{GUID: 1, Map: 1, CurHP: 1},
	}

	require.NoError(t, s.processNextLayerSwitch(context.Background()))
	require.False(t, s.layerSwitchInProgress)
	require.False(t, s.seamlessLayerSwitch)
}
