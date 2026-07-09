package session

import (
	"context"
	"encoding/binary"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/walkline/ToCloud9/apps/gateway/packet"
	mocks "github.com/walkline/ToCloud9/apps/gateway/sockets/socketmock"
	pbChar "github.com/walkline/ToCloud9/gen/characters/pb"
	charMocks "github.com/walkline/ToCloud9/gen/characters/pb/mocks"
)

func nameQueryPacket(charGUID uint64) *packet.Packet {
	payload := make([]byte, 8)
	binary.LittleEndian.PutUint64(payload, charGUID)
	return &packet.Packet{Opcode: packet.CMsgNameQuery, Data: payload}
}

// The game server silently drops STATUS_LOGGEDIN opcodes while the player is
// logging in, so the gateway must answer name queries itself instead of
// forwarding them.
func TestHandleNameQueryAnswersAtGatewayForOnlineCharacter(t *testing.T) {
	charClient := &charMocks.CharactersServiceClient{}
	charClient.On("ShortOnlineCharactersDataByGUIDs", mock.Anything, mock.Anything).
		Return(&pbChar.ShortCharactersDataByGUIDsResponse{
			Characters: []*pbChar.ShortCharactersDataByGUIDsResponse_ShortCharData{{
				CharGUID:   40554,
				CharName:   "Ixy",
				CharRace:   1,
				CharGender: 1,
				CharClass:  5,
			}},
		}, nil)

	var sent *packet.Writer
	gameSocket := &mocks.Socket{}
	gameSocket.On("Send", mock.MatchedBy(func(w *packet.Writer) bool {
		sent = w
		return w.Opcode == packet.SMsgNameQueryResponse
	})).Return()

	worldSocket := &mocks.Socket{}

	session := &GameSession{
		charServiceClient: charClient,
		gameSocket:        gameSocket,
		worldSocket:       worldSocket,
	}

	err := session.HandleNameQuery(context.Background(), nameQueryPacket(40554))

	assert.NoError(t, err)
	worldSocket.AssertNotCalled(t, "SendPacket", mock.Anything)
	if assert.NotNil(t, sent) {
		reader := sent.ToPacket().Reader()
		assert.Equal(t, uint64(40554), reader.ReadGUID())
		assert.Equal(t, uint8(0), reader.Uint8(), "name should be marked as known")
		assert.Equal(t, "Ixy", reader.String())
		assert.Equal(t, uint8(0), reader.Uint8(), "realm name should be empty")
		assert.Equal(t, uint8(1), reader.Uint8(), "race")
		assert.Equal(t, uint8(1), reader.Uint8(), "gender")
		assert.Equal(t, uint8(5), reader.Uint8(), "class")
	}
}

func TestHandleNameQueryFallsBackToStorageForOfflineCharacter(t *testing.T) {
	charClient := &charMocks.CharactersServiceClient{}
	charClient.On("ShortOnlineCharactersDataByGUIDs", mock.Anything, mock.Anything).
		Return(&pbChar.ShortCharactersDataByGUIDsResponse{}, nil)
	charClient.On("CharactersToLoginByGUID", mock.Anything, mock.Anything).
		Return(&pbChar.CharactersToLoginByGUIDResponse{
			Character: &pbChar.LogInCharacter{
				GUID:   40554,
				Name:   "Ixy",
				Race:   1,
				Gender: 1,
				Class:  5,
			},
		}, nil)

	var sent *packet.Writer
	gameSocket := &mocks.Socket{}
	gameSocket.On("Send", mock.MatchedBy(func(w *packet.Writer) bool {
		sent = w
		return w.Opcode == packet.SMsgNameQueryResponse
	})).Return()

	worldSocket := &mocks.Socket{}

	session := &GameSession{
		charServiceClient: charClient,
		gameSocket:        gameSocket,
		worldSocket:       worldSocket,
	}

	err := session.HandleNameQuery(context.Background(), nameQueryPacket(40554))

	assert.NoError(t, err)
	worldSocket.AssertNotCalled(t, "SendPacket", mock.Anything)
	if assert.NotNil(t, sent) {
		reader := sent.ToPacket().Reader()
		assert.Equal(t, uint64(40554), reader.ReadGUID())
		assert.Equal(t, uint8(0), reader.Uint8())
		assert.Equal(t, "Ixy", reader.String())
	}
}

func TestHandleNameQueryForwardsUnknownCharacterToGameServer(t *testing.T) {
	charClient := &charMocks.CharactersServiceClient{}
	charClient.On("ShortOnlineCharactersDataByGUIDs", mock.Anything, mock.Anything).
		Return(&pbChar.ShortCharactersDataByGUIDsResponse{}, nil)
	charClient.On("CharactersToLoginByGUID", mock.Anything, mock.Anything).
		Return(&pbChar.CharactersToLoginByGUIDResponse{}, nil)

	gameSocket := &mocks.Socket{}

	worldSocket := &mocks.Socket{}
	worldSocket.On("SendPacket", mock.Anything).Return()

	session := &GameSession{
		charServiceClient: charClient,
		gameSocket:        gameSocket,
		worldSocket:       worldSocket,
	}

	p := nameQueryPacket(99999)
	err := session.HandleNameQuery(context.Background(), p)

	assert.NoError(t, err)
	gameSocket.AssertNotCalled(t, "Send", mock.Anything)
	worldSocket.AssertCalled(t, "SendPacket", p)
}
