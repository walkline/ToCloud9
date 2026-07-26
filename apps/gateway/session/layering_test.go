package session

import (
	"context"
	"testing"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/walkline/ToCloud9/apps/gateway/packet"
	"github.com/walkline/ToCloud9/apps/gateway/sockets"
	socketMocks "github.com/walkline/ToCloud9/apps/gateway/sockets/socketmock"
)

func TestLayerPlayerRedirectSendsNewWorldBeforeInstallingDestinationSocket(t *testing.T) {
	previousWorldSocketCreator := WorldSocketCreator
	t.Cleanup(func() { WorldSocketCreator = previousWorldSocketCreator })

	sourceRead := make(chan *packet.Packet, 1)
	sourceRead <- packet.NewWriter(packet.TC9SMsgReadyForRedirect).Uint8(0).ToPacket()
	sourceSocket := socketMocks.NewSocket(t)
	sourceSocket.On("Send", mock.MatchedBy(func(writer *packet.Writer) bool {
		return writer.Opcode == packet.TC9CMsgPrepareForRedirect
	})).Return()
	sourceSocket.On("ReadChannel").Return((<-chan *packet.Packet)(sourceRead))
	sourceSocket.On("Close").Return()

	destinationRead := make(chan *packet.Packet, 2)
	destinationRead <- packet.NewWriter(packet.SMsgAuthChallenge).ToPacket()
	destinationRead <- packet.NewWriter(packet.SMsgLoginVerifyWorld).
		Uint32(1).
		Float32(10.5).
		Float32(20.25).
		Float32(30.75).
		Float32(1.5).
		ToPacket()
	destinationSocket := socketMocks.NewSocket(t)
	destinationSocket.On("ListenAndProcess", mock.Anything).Return(nil)
	destinationSocket.On("SendPacket", mock.Anything).Return()
	destinationSocket.On("Send", mock.MatchedBy(func(writer *packet.Writer) bool {
		return writer.Opcode == packet.CMsgPlayerLogin
	})).Return()
	destinationSocket.On("ReadChannel").Return((<-chan *packet.Packet)(destinationRead))

	gameSocket := socketMocks.NewSocket(t)
	var newWorldPacket *packet.Packet
	var session *GameSession
	gameSocket.On("Send", mock.MatchedBy(func(writer *packet.Writer) bool {
		return writer.Opcode == packet.SMsgTransferPending
	})).Return()
	gameSocket.On("Send", mock.MatchedBy(func(writer *packet.Writer) bool {
		return writer.Opcode == packet.SMsgNewWorld
	})).Run(func(arguments mock.Arguments) {
		assert.Nil(t, session.worldSocket, "destination socket must not be active before SMSG_NEW_WORLD")
		newWorldPacket = arguments.Get(0).(*packet.Writer).ToPacket()
	}).Return()

	session = NewGameSession(
		context.Background(),
		&log.Logger,
		gameSocket,
		7,
		packet.NewWriter(packet.CMsgAuthSession).ToPacket(),
		GameSessionParams{},
	)
	session.character = &LoggedInCharacter{
		GUID:      42,
		Map:       1,
		PositionX: 10.5,
		PositionY: 20.25,
		PositionZ: 30.75,
		PositionO: 1.5,
	}
	session.worldSocket = sourceSocket

	WorldSocketCreator = func(*zerolog.Logger, string) (sockets.Socket, error) {
		return destinationSocket, nil
	}

	require.NoError(t, session.layerPlayerRedirect(context.Background(), 42, "destination:8085", "illidan-vashj-z4"))
	require.Same(t, destinationSocket, session.worldSocket)
	require.True(t, session.worldEntryPending)
	require.NotNil(t, newWorldPacket)

	reader := newWorldPacket.Reader()
	assert.Equal(t, uint32(1), reader.Uint32())
	assert.Equal(t, float32(10.5), reader.Float32())
	assert.Equal(t, float32(20.25), reader.Float32())
	assert.Equal(t, float32(30.75), reader.Float32())
	assert.Equal(t, float32(1.5), reader.Float32())
	assert.NoError(t, reader.Error())
}

func TestLayerPlayerRedirectKeepsSourceSocketWhenPreparationFails(t *testing.T) {
	sourceRead := make(chan *packet.Packet, 1)
	sourceRead <- packet.NewWriter(packet.TC9SMsgReadyForRedirect).Uint8(1).ToPacket()
	sourceSocket := socketMocks.NewSocket(t)
	sourceSocket.On("Send", mock.Anything).Return()
	sourceSocket.On("ReadChannel").Return((<-chan *packet.Packet)(sourceRead))

	session := &GameSession{
		gameSocket:  socketMocks.NewSocket(t),
		worldSocket: sourceSocket,
		character:   &LoggedInCharacter{GUID: 42, Map: 1},
		accountID:   7,
	}

	err := session.layerPlayerRedirect(context.Background(), 42, "destination:8085", "illidan-vashj-z4")
	require.Error(t, err)
	assert.Same(t, sourceSocket, session.worldSocket)
}
