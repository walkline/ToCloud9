package session

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/walkline/ToCloud9/apps/gateway/packet"
	socketMock "github.com/walkline/ToCloud9/apps/gateway/sockets/socketmock"
)

func TestSeamlessHandoffBuffersLatestMovementUntilWorldEntry(t *testing.T) {
	s := &GameSession{
		character:           &LoggedInCharacter{GUID: 1},
		seamlessLayerSwitch: true,
		worldEntryPending:   true,
	}
	first := movementPacket(1, 10)
	latest := movementPacket(1, 20)

	require.NoError(t, s.HandleMovement(context.Background(), first))
	require.NoError(t, s.HandleMovement(context.Background(), latest))
	require.Same(t, latest, s.pendingLayerMovement)

	world := &socketMock.Socket{}
	world.On("SendPacket", latest).Once()
	s.worldSocket = world
	s.flushPendingLayerMovement()

	require.Nil(t, s.pendingLayerMovement)
	world.AssertExpectations(t)
}

func movementPacket(guid uint64, x float32) *packet.Packet {
	p := packet.NewWriter(packet.MsgMoveHeartbeat).
		GUID(guid).
		Uint32(0).
		Uint16(0).
		Uint32(1).
		Float32(x).
		Float32(2).
		Float32(3).
		Float32(4).
		ToPacket()
	p.Source = packet.SourceGameClient
	return p
}
