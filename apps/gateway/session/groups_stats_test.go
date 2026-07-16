package session

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/walkline/ToCloud9/apps/gateway/packet"
)

func TestHandleRequestPartyMemberStatsDuringLayerHandoff(t *testing.T) {
	tests := map[string]*GameSession{
		"character not initialized": {},
		"gateway group member not cached": {
			character: &LoggedInCharacter{},
		},
		"game server managed group": {
			character: &LoggedInCharacter{GroupMangedByGameServer: true},
		},
	}

	for name, session := range tests {
		t.Run(name, func(t *testing.T) {
			request := packet.NewWriter(packet.CMsgRequestPartyMemberStats).
				Uint64(42).
				ToPacket()

			require.NotPanics(t, func() {
				require.NoError(t, session.HandleRequestPartyMemberStats(context.Background(), request))
			})
		})
	}
}
