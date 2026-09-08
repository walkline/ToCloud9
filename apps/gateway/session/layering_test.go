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
	pbServ "github.com/walkline/ToCloud9/gen/servers-registry/pb"
	regMocks "github.com/walkline/ToCloud9/gen/servers-registry/pb/mocks"
)

func TestLayerPlayerRedirectFallsBackToNewWorldWithLegacyCore(t *testing.T) {
	previousWorldSocketCreator := WorldSocketCreator
	t.Cleanup(func() { WorldSocketCreator = previousWorldSocketCreator })

	sourceRead := make(chan *packet.Packet, 1)
	sourceRead <- packet.NewWriter(packet.TC9SMsgReadyForRedirect).Uint8(0).ToPacket()
	sourceSocket := socketMocks.NewSocket(t)
	sourceSocket.On("Send", mock.MatchedBy(func(writer *packet.Writer) bool {
		return writer.Opcode == packet.TC9CMsgPrepareForRedirect &&
			assert.Equal(t, []byte{packet.TC9RedirectVersionedRequest, packet.TC9RedirectOptionSeamless}, writer.Payload.Bytes())
	})).Return()
	sourceSocket.On("ReadChannel").Return((<-chan *packet.Packet)(sourceRead))
	sourceSocket.On("Close").Return()

	firstLoginPacket := packet.NewWriter(packet.SMsgLoginVerifyWorld).
		Uint32(1).Float32(10.5).Float32(20.25).Float32(30.75).Float32(1.5).ToPacket()
	destinationRead := make(chan *packet.Packet, 2)
	destinationRead <- packet.NewWriter(packet.SMsgAuthChallenge).ToPacket()
	destinationRead <- firstLoginPacket
	destinationSocket := socketMocks.NewSocket(t)
	destinationSocket.On("ListenAndProcess", mock.Anything).Return(nil)
	destinationSocket.On("SendPacket", mock.Anything).Return()
	destinationSocket.On("Send", mock.MatchedBy(func(writer *packet.Writer) bool {
		return writer.Opcode == packet.CMsgPlayerLogin
	})).Return()
	destinationSocket.On("ReadChannel").Return((<-chan *packet.Packet)(destinationRead))

	gameWrite := make(chan *packet.Packet, 1)
	gameSocket := socketMocks.NewSocket(t)
	var newWorldPacket *packet.Packet
	var session *GameSession
	var sawFirstPacketAfterNewWorld bool
	gameSocket.On("Send", mock.MatchedBy(func(writer *packet.Writer) bool {
		return writer.Opcode == packet.SMsgNewWorld
	})).Run(func(arguments mock.Arguments) {
		assert.Nil(t, session.worldSocket, "destination socket must not be active before SMSG_NEW_WORLD")
		newWorldPacket = arguments.Get(0).(*packet.Writer).ToPacket()
	}).Return()
	gameSocket.On("WriteChannel").Return((chan<- *packet.Packet)(gameWrite)).Run(func(mock.Arguments) {
		assert.NotNil(t, session.worldSocket, "first destination packet is dispatched only after install")
		assert.NotNil(t, newWorldPacket, "SMSG_NEW_WORLD must precede first destination packet")
		sawFirstPacketAfterNewWorld = true
	})

	session = NewGameSession(
		context.Background(),
		&log.Logger,
		gameSocket,
		7,
		packet.NewWriter(packet.CMsgAuthSession).ToPacket(),
		GameSessionParams{SeamlessLayerSwitch: true},
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

	require.NoError(t, session.layerPlayerRedirect(context.Background(), 42, "destination:8085", "red-onyxia-7k"))
	require.Same(t, destinationSocket, session.worldSocket)
	require.True(t, session.worldEntryPending)
	require.NotNil(t, newWorldPacket)
	require.True(t, sawFirstPacketAfterNewWorld)
	require.Same(t, firstLoginPacket, <-gameWrite)

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
	sourceSocket.On("Send", mock.MatchedBy(func(writer *packet.Writer) bool {
		return writer.Opcode == packet.TC9CMsgPrepareForRedirect && writer.Payload.Len() == 0
	})).Return()
	sourceSocket.On("ReadChannel").Return((<-chan *packet.Packet)(sourceRead))

	session := &GameSession{
		gameSocket:  socketMocks.NewSocket(t),
		worldSocket: sourceSocket,
		character:   &LoggedInCharacter{GUID: 42, Map: 1},
		accountID:   7,
	}

	err := session.layerPlayerRedirect(context.Background(), 42, "destination:8085", "red-onyxia-7k")
	require.Error(t, err)
	assert.Same(t, sourceSocket, session.worldSocket)
}

func TestLayerPlayerRedirectSeamlessForwardsSourceVisibilityAndKeepsWorldLoaded(t *testing.T) {
	previousWorldSocketCreator := WorldSocketCreator
	t.Cleanup(func() { WorldSocketCreator = previousWorldSocketCreator })

	visibilityPacket := packet.NewWriter(packet.SMsgUpdateObject).Uint32(0).ToPacket()
	sourceRead := make(chan *packet.Packet, 2)
	sourceRead <- visibilityPacket
	sourceRead <- packet.NewWriter(packet.TC9SMsgReadyForRedirect).
		Uint8(0).
		Uint8(packet.TC9RedirectVersionedRequest).
		Uint8(packet.TC9RedirectOptionSeamless).
		ToPacket()
	sourceSocket := socketMocks.NewSocket(t)
	sourceSocket.On("Send", mock.MatchedBy(func(writer *packet.Writer) bool {
		return writer.Opcode == packet.TC9CMsgPrepareForRedirect &&
			assert.Equal(t, []byte{packet.TC9RedirectVersionedRequest, packet.TC9RedirectOptionSeamless}, writer.Payload.Bytes())
	})).Return()
	sourceSocket.On("ReadChannel").Return((<-chan *packet.Packet)(sourceRead))
	sourceSocket.On("Close").Return()

	firstLoginPacket := packet.NewWriter(packet.SMsgLoginVerifyWorld).
		Uint32(1).Float32(10.5).Float32(20.25).Float32(30.75).Float32(1.5).ToPacket()
	destinationRead := make(chan *packet.Packet, 2)
	destinationRead <- packet.NewWriter(packet.SMsgAuthChallenge).ToPacket()
	destinationRead <- firstLoginPacket
	destinationSocket := socketMocks.NewSocket(t)
	destinationSocket.On("ListenAndProcess", mock.Anything).Return(nil)
	destinationSocket.On("SendPacket", mock.Anything).Return()
	destinationSocket.On("Send", mock.MatchedBy(func(writer *packet.Writer) bool {
		return writer.Opcode == packet.CMsgPlayerLogin
	})).Return()
	destinationSocket.On("ReadChannel").Return((<-chan *packet.Packet)(destinationRead))

	gameWrite := make(chan *packet.Packet, 2)
	gameSocket := socketMocks.NewSocket(t)
	gameSocket.On("WriteChannel").Return((chan<- *packet.Packet)(gameWrite))

	session := NewGameSession(
		context.Background(),
		&log.Logger,
		gameSocket,
		7,
		packet.NewWriter(packet.CMsgAuthSession).ToPacket(),
		GameSessionParams{},
	)
	session.character = &LoggedInCharacter{GUID: 42, Map: 1}
	session.worldSocket = sourceSocket
	session.seamlessLayerSwitch = true

	WorldSocketCreator = func(*zerolog.Logger, string) (sockets.Socket, error) {
		return destinationSocket, nil
	}

	require.NoError(t, session.layerPlayerRedirect(context.Background(), 42, "destination:8085", "red-onyxia-7k"))
	require.Same(t, destinationSocket, session.worldSocket)
	require.True(t, session.worldEntryPending)
	assert.Same(t, visibilityPacket, <-gameWrite, "source visibility teardown must reach the client first")
	assert.Same(t, firstLoginPacket, <-gameWrite, "destination login stream must follow without SMSG_NEW_WORLD")
	assert.Empty(t, gameWrite)
}

func TestApplyGroupLayerUsesSeamlessRedirect(t *testing.T) {
	previousWorldSocketCreator := WorldSocketCreator
	t.Cleanup(func() { WorldSocketCreator = previousWorldSocketCreator })

	const groupID = uint32(77)
	registry := &regMocks.ServersRegistryServiceClient{}
	registry.On("AvailableGameServersForMapAndRealm", mock.Anything, mock.MatchedBy(func(req *pbServ.AvailableGameServersForMapAndRealmRequest) bool {
		return req.MapID == 1 && req.GroupID == groupID
	})).Return(&pbServ.AvailableGameServersForMapAndRealmResponse{
		GameServers: []*pbServ.Server{{
			ID:          "destination-id",
			Address:     "destination:8085",
			GrpcAddress: "destination:9509",
			Alias:       "red-onyxia-7k",
		}},
	}, nil)

	visibilityPacket := packet.NewWriter(packet.SMsgUpdateObject).Uint32(0).ToPacket()
	sourceRead := make(chan *packet.Packet, 2)
	sourceRead <- visibilityPacket
	sourceRead <- packet.NewWriter(packet.TC9SMsgReadyForRedirect).
		Uint8(0).
		Uint8(packet.TC9RedirectVersionedRequest).
		Uint8(packet.TC9RedirectOptionSeamless).
		ToPacket()
	sourceSocket := socketMocks.NewSocket(t)
	sourceSocket.On("Send", mock.MatchedBy(func(writer *packet.Writer) bool {
		return writer.Opcode == packet.TC9CMsgPrepareForRedirect &&
			assert.Equal(t, []byte{packet.TC9RedirectVersionedRequest, packet.TC9RedirectOptionSeamless}, writer.Payload.Bytes())
	})).Return()
	sourceSocket.On("ReadChannel").Return((<-chan *packet.Packet)(sourceRead))
	sourceSocket.On("Close").Return()

	firstLoginPacket := packet.NewWriter(packet.SMsgLoginVerifyWorld).Uint32(1).ToPacket()
	destinationRead := make(chan *packet.Packet, 2)
	destinationRead <- packet.NewWriter(packet.SMsgAuthChallenge).ToPacket()
	destinationRead <- firstLoginPacket
	destinationSocket := socketMocks.NewSocket(t)
	destinationSocket.On("ListenAndProcess", mock.Anything).Return(nil)
	destinationSocket.On("SendPacket", mock.Anything).Return()
	destinationSocket.On("Send", mock.MatchedBy(func(writer *packet.Writer) bool {
		return writer.Opcode == packet.CMsgPlayerLogin
	})).Return()
	destinationSocket.On("ReadChannel").Return((<-chan *packet.Packet)(destinationRead))

	gameWrite := make(chan *packet.Packet, 2)
	gameSocket := socketMocks.NewSocket(t)
	gameSocket.On("Send", mock.MatchedBy(func(writer *packet.Writer) bool {
		return writer.Opcode == packet.SMsgMessageChat
	})).Return()
	gameSocket.On("WriteChannel").Return((chan<- *packet.Packet)(gameWrite))

	session := NewGameSession(
		context.Background(),
		&log.Logger,
		gameSocket,
		7,
		packet.NewWriter(packet.CMsgAuthSession).ToPacket(),
		GameSessionParams{
			ServersRegistryClient: registry,
			GameServerGRPCConnMgr: &GameGRPCConnMgrMock{},
			SeamlessLayerSwitch:   true,
		},
	)
	session.character = &LoggedInCharacter{GUID: 42, Map: 1}
	session.worldSocket = sourceSocket
	session.currentGameServerID = "source-id"

	WorldSocketCreator = func(*zerolog.Logger, string) (sockets.Socket, error) {
		return destinationSocket, nil
	}

	require.NoError(t, session.applyGroupLayer(context.Background(), groupID))
	assert.Equal(t, "destination-id", session.currentGameServerID)
	assert.Equal(t, "red-onyxia-7k", session.currentGameServerAlias)
	assert.Same(t, visibilityPacket, <-gameWrite)
	assert.Same(t, firstLoginPacket, <-gameWrite)
	assert.Empty(t, gameWrite)
}

func TestWaitForWorldServerRedirectNegotiatesOptionsAndFallsBack(t *testing.T) {
	tests := []struct {
		name            string
		acknowledgement *packet.Packet
		wantOptions     uint8
		wantError       bool
	}{
		{
			name:            "legacy core",
			acknowledgement: packet.NewWriter(packet.TC9SMsgReadyForRedirect).Uint8(0).ToPacket(),
		},
		{
			name: "seamless accepted",
			acknowledgement: packet.NewWriter(packet.TC9SMsgReadyForRedirect).
				Uint8(0).
				Uint8(packet.TC9RedirectVersionedRequest).
				Uint8(packet.TC9RedirectOptionSeamless).
				ToPacket(),
			wantOptions: packet.TC9RedirectOptionSeamless,
		},
		{
			name: "seamless disabled by core",
			acknowledgement: packet.NewWriter(packet.TC9SMsgReadyForRedirect).
				Uint8(0).
				Uint8(packet.TC9RedirectVersionedRequest).
				Uint8(0).
				ToPacket(),
		},
		{
			name: "unknown response version",
			acknowledgement: packet.NewWriter(packet.TC9SMsgReadyForRedirect).
				Uint8(0).
				Uint8(packet.TC9RedirectVersionedRequest + 1).
				Uint8(packet.TC9RedirectOptionSeamless).
				ToPacket(),
		},
		{
			name: "redirect rejected",
			acknowledgement: packet.NewWriter(packet.TC9SMsgReadyForRedirect).
				Uint8(1).
				ToPacket(),
			wantError: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			read := make(chan *packet.Packet, 1)
			read <- test.acknowledgement
			socket := socketMocks.NewSocket(t)
			socket.On("ReadChannel").Return((<-chan *packet.Packet)(read))

			options, err := waitForWorldServerRedirect(context.Background(), socket, nil)
			if test.wantError {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, test.wantOptions, options)
		})
	}
}
