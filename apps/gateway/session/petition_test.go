package session

import (
	"context"
	"testing"

	"github.com/rs/zerolog/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/walkline/ToCloud9/apps/gateway/packet"
	mocks "github.com/walkline/ToCloud9/apps/gateway/sockets/socketmock"
	pbGuild "github.com/walkline/ToCloud9/gen/guilds/pb"
	pbWorld "github.com/walkline/ToCloud9/gen/worldserver/pb"
)

type worldServerClientCanTurnInMock struct {
	pbWorld.WorldServerServiceClient
	resp *pbWorld.CanTurnInGuildPetitionResponse
	err  error

	request *pbWorld.CanTurnInGuildPetitionRequest
}

func (m *worldServerClientCanTurnInMock) CanTurnInGuildPetition(_ context.Context, in *pbWorld.CanTurnInGuildPetitionRequest, _ ...grpc.CallOption) (*pbWorld.CanTurnInGuildPetitionResponse, error) {
	m.request = in
	return m.resp, m.err
}

type guildServiceClientCreateGuildMock struct {
	pbGuild.GuildServiceClient
	resp *pbGuild.CreateGuildResponse
	err  error

	params *pbGuild.CreateGuildParams
}

func (m *guildServiceClientCreateGuildMock) CreateGuild(_ context.Context, in *pbGuild.CreateGuildParams, _ ...grpc.CallOption) (*pbGuild.CreateGuildResponse, error) {
	m.params = in
	return m.resp, m.err
}

func turnInPetitionSession(t *testing.T, worldClient pbWorld.WorldServerServiceClient, guildClient pbGuild.GuildServiceClient) (*GameSession, *[]*packet.Writer, *[]*packet.Packet) {
	t.Helper()

	sentToClient := &[]*packet.Writer{}
	gameSocket := &mocks.Socket{}
	gameSocket.On("Send", mock.Anything).Run(func(args mock.Arguments) {
		*sentToClient = append(*sentToClient, args.Get(0).(*packet.Writer))
	}).Return()

	forwardedToWorld := &[]*packet.Packet{}
	worldSocket := &mocks.Socket{}
	worldSocket.On("Address").Return("gameserver:9601")
	worldSocket.On("SendPacket", mock.Anything).Run(func(args mock.Arguments) {
		*forwardedToWorld = append(*forwardedToWorld, args.Get(0).(*packet.Packet))
	}).Return()

	session := &GameSession{
		logger:      &log.Logger,
		gameSocket:  gameSocket,
		worldSocket: worldSocket,
		character:   &LoggedInCharacter{GUID: 42},
		gameServerGRPCConnMgr: &GameGRPCConnMgrMock{
			connToReturn: worldClient,
		},
		guildServiceClient: guildClient,
	}

	return session, sentToClient, forwardedToWorld
}

func turnInPetitionPacket() *packet.Packet {
	return packet.NewWriter(packet.CMsgTurnInPetition).Uint64(0xF110000000000001).ToPacket()
}

func TestHandleTurnInPetitionCreatesGuild(t *testing.T) {
	worldClient := &worldServerClientCanTurnInMock{
		resp: &pbWorld.CanTurnInGuildPetitionResponse{
			Status:         pbWorld.CanTurnInGuildPetitionResponse_Ok,
			GuildName:      "Stormwind Watch",
			SignatoryGUIDs: []uint64{10, 11, 12},
		},
	}
	guildClient := &guildServiceClientCreateGuildMock{
		resp: &pbGuild.CreateGuildResponse{GuildID: 7},
	}
	session, sentToClient, _ := turnInPetitionSession(t, worldClient, guildClient)

	err := session.HandleTurnInPetition(context.Background(), turnInPetitionPacket())
	assert.Nil(t, err)

	assert.Equal(t, uint64(42), worldClient.request.PlayerGUID)
	assert.Equal(t, uint64(0xF110000000000001), worldClient.request.PetitionItemGUID)

	assert.Equal(t, uint64(42), guildClient.params.LeaderGUID)
	assert.Equal(t, "Stormwind Watch", guildClient.params.Name)
	assert.Equal(t, []uint64{10, 11, 12}, guildClient.params.SignatoryGUIDs)

	assert.Equal(t, uint32(7), session.character.GuildID)

	if assert.Len(t, *sentToClient, 2) {
		assert.Equal(t, packet.SMsgGuildCommandResult, (*sentToClient)[0].Opcode)
		assert.Equal(t, packet.SMsgTurnInPetitionResults, (*sentToClient)[1].Opcode)
		result := (*sentToClient)[1].ToPacket().Reader().Uint32()
		assert.Equal(t, uint32(petitionTurnOK), result)
	}
}

func TestHandleTurnInPetitionForwardsArenaCharter(t *testing.T) {
	worldClient := &worldServerClientCanTurnInMock{
		resp: &pbWorld.CanTurnInGuildPetitionResponse{
			Status: pbWorld.CanTurnInGuildPetitionResponse_NotGuildPetition,
		},
	}
	guildClient := &guildServiceClientCreateGuildMock{}
	session, sentToClient, forwardedToWorld := turnInPetitionSession(t, worldClient, guildClient)

	p := turnInPetitionPacket()
	err := session.HandleTurnInPetition(context.Background(), p)
	assert.Nil(t, err)

	if assert.Len(t, *forwardedToWorld, 1) {
		assert.Equal(t, p, (*forwardedToWorld)[0])
	}
	assert.Empty(t, *sentToClient)
	assert.Nil(t, guildClient.params)
}

func TestHandleTurnInPetitionBusinessStatuses(t *testing.T) {
	tests := []struct {
		name           string
		status         pbWorld.CanTurnInGuildPetitionResponse_Status
		expectedResult uint32
	}{
		{"already in guild", pbWorld.CanTurnInGuildPetitionResponse_AlreadyInGuild, petitionTurnAlreadyInGuild},
		{"need more signatures", pbWorld.CanTurnInGuildPetitionResponse_NeedMoreSignatures, petitionTurnNeedMoreSignatures},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			worldClient := &worldServerClientCanTurnInMock{
				resp: &pbWorld.CanTurnInGuildPetitionResponse{Status: tt.status},
			}
			guildClient := &guildServiceClientCreateGuildMock{}
			session, sentToClient, _ := turnInPetitionSession(t, worldClient, guildClient)

			err := session.HandleTurnInPetition(context.Background(), turnInPetitionPacket())
			assert.Nil(t, err)

			if assert.Len(t, *sentToClient, 1) {
				assert.Equal(t, packet.SMsgTurnInPetitionResults, (*sentToClient)[0].Opcode)
				result := (*sentToClient)[0].ToPacket().Reader().Uint32()
				assert.Equal(t, tt.expectedResult, result)
			}
			assert.Nil(t, guildClient.params)
		})
	}
}

func TestHandleTurnInPetitionSilentStatuses(t *testing.T) {
	for _, s := range []pbWorld.CanTurnInGuildPetitionResponse_Status{
		pbWorld.CanTurnInGuildPetitionResponse_PlayerNotFound,
		pbWorld.CanTurnInGuildPetitionResponse_PetitionNotFound,
		pbWorld.CanTurnInGuildPetitionResponse_NotPetitionOwner,
	} {
		worldClient := &worldServerClientCanTurnInMock{
			resp: &pbWorld.CanTurnInGuildPetitionResponse{Status: s},
		}
		guildClient := &guildServiceClientCreateGuildMock{}
		session, sentToClient, forwardedToWorld := turnInPetitionSession(t, worldClient, guildClient)

		err := session.HandleTurnInPetition(context.Background(), turnInPetitionPacket())
		assert.Nil(t, err)
		assert.Empty(t, *sentToClient)
		assert.Empty(t, *forwardedToWorld)
		assert.Nil(t, guildClient.params)
	}
}

func TestHandleTurnInPetitionNameTaken(t *testing.T) {
	worldClient := &worldServerClientCanTurnInMock{
		resp: &pbWorld.CanTurnInGuildPetitionResponse{
			Status:    pbWorld.CanTurnInGuildPetitionResponse_Ok,
			GuildName: "Stormwind Watch",
		},
	}
	guildClient := &guildServiceClientCreateGuildMock{
		err: status.Error(codes.AlreadyExists, "guild name already taken"),
	}
	session, sentToClient, _ := turnInPetitionSession(t, worldClient, guildClient)

	err := session.HandleTurnInPetition(context.Background(), turnInPetitionPacket())
	assert.Nil(t, err)

	assert.Equal(t, uint32(0), session.character.GuildID)

	if assert.Len(t, *sentToClient, 1) {
		assert.Equal(t, packet.SMsgGuildCommandResult, (*sentToClient)[0].Opcode)
		reader := (*sentToClient)[0].ToPacket().Reader()
		assert.Equal(t, uint32(guildCommandCreate), reader.Uint32())
		assert.Equal(t, "Stormwind Watch", reader.String())
		assert.Equal(t, uint32(guildErrNameExistsS), reader.Uint32())
	}
}

func TestHandleTurnInPetitionCreateFailure(t *testing.T) {
	worldClient := &worldServerClientCanTurnInMock{
		resp: &pbWorld.CanTurnInGuildPetitionResponse{
			Status:    pbWorld.CanTurnInGuildPetitionResponse_Ok,
			GuildName: "Stormwind Watch",
		},
	}
	guildClient := &guildServiceClientCreateGuildMock{
		err: status.Error(codes.Unavailable, "guild service down"),
	}
	session, sentToClient, _ := turnInPetitionSession(t, worldClient, guildClient)

	err := session.HandleTurnInPetition(context.Background(), turnInPetitionPacket())
	assert.NotNil(t, err)
	assert.Empty(t, *sentToClient)
	assert.Equal(t, uint32(0), session.character.GuildID)
}
