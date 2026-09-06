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

	eBroadcasterMocks "github.com/walkline/ToCloud9/apps/gateway/events-broadcaster/mocks"
	"github.com/walkline/ToCloud9/apps/gateway/packet"
	mocks "github.com/walkline/ToCloud9/apps/gateway/sockets/socketmock"
	pbChar "github.com/walkline/ToCloud9/gen/characters/pb"
	pbGuild "github.com/walkline/ToCloud9/gen/guilds/pb"
	pbWorld "github.com/walkline/ToCloud9/gen/worldserver/pb"
)

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

type charServicePetitionMock struct {
	pbChar.CharactersServiceClient

	validateResp *pbChar.ValidateGuildPetitionTurnInResponse
	validateErr  error
	validateReq  *pbChar.ValidateGuildPetitionTurnInRequest

	getPetitionResp *pbChar.GetGuildPetitionResponse
	getPetitionErr  error

	getSigsResp *pbChar.GetGuildPetitionSignaturesResponse
	getSigsErr  error

	addSigResp *pbChar.AddGuildPetitionSignatureResponse
	addSigErr  error
	addSigReq  *pbChar.AddGuildPetitionSignatureRequest

	deleteResp *pbChar.DeleteGuildPetitionResponse
	deleteErr  error

	onlineChars []*pbChar.ShortCharactersDataByGUIDsResponse_ShortCharData
	loginChar   *pbChar.LogInCharacter
}

func (m *charServicePetitionMock) ValidateGuildPetitionTurnIn(_ context.Context, in *pbChar.ValidateGuildPetitionTurnInRequest, _ ...grpc.CallOption) (*pbChar.ValidateGuildPetitionTurnInResponse, error) {
	m.validateReq = in
	return m.validateResp, m.validateErr
}

func (m *charServicePetitionMock) GetGuildPetition(_ context.Context, in *pbChar.GetGuildPetitionRequest, _ ...grpc.CallOption) (*pbChar.GetGuildPetitionResponse, error) {
	return m.getPetitionResp, m.getPetitionErr
}

func (m *charServicePetitionMock) GetGuildPetitionSignatures(_ context.Context, in *pbChar.GetGuildPetitionSignaturesRequest, _ ...grpc.CallOption) (*pbChar.GetGuildPetitionSignaturesResponse, error) {
	return m.getSigsResp, m.getSigsErr
}

func (m *charServicePetitionMock) AddGuildPetitionSignature(_ context.Context, in *pbChar.AddGuildPetitionSignatureRequest, _ ...grpc.CallOption) (*pbChar.AddGuildPetitionSignatureResponse, error) {
	m.addSigReq = in
	return m.addSigResp, m.addSigErr
}

func (m *charServicePetitionMock) DeleteGuildPetition(_ context.Context, _ *pbChar.DeleteGuildPetitionRequest, _ ...grpc.CallOption) (*pbChar.DeleteGuildPetitionResponse, error) {
	return m.deleteResp, m.deleteErr
}

func (m *charServicePetitionMock) ShortOnlineCharactersDataByGUIDs(_ context.Context, _ *pbChar.ShortCharactersDataByGUIDsRequest, _ ...grpc.CallOption) (*pbChar.ShortCharactersDataByGUIDsResponse, error) {
	return &pbChar.ShortCharactersDataByGUIDsResponse{Characters: m.onlineChars}, nil
}

func (m *charServicePetitionMock) CharactersToLoginByGUID(_ context.Context, _ *pbChar.CharactersToLoginByGUIDRequest, _ ...grpc.CallOption) (*pbChar.CharactersToLoginByGUIDResponse, error) {
	if m.loginChar != nil {
		return &pbChar.CharactersToLoginByGUIDResponse{Character: m.loginChar}, nil
	}
	return &pbChar.CharactersToLoginByGUIDResponse{
		Character: &pbChar.LogInCharacter{Race: 1},
	}, nil
}

func (m *charServicePetitionMock) GuildNameExists(_ context.Context, _ *pbChar.GuildNameExistsRequest, _ ...grpc.CallOption) (*pbChar.GuildNameExistsResponse, error) {
	return &pbChar.GuildNameExistsResponse{Exists: false}, nil
}

// worldClientTurnInMock returns the charter item for GetPlayerItemsByGuids by default.
type worldClientTurnInMock struct {
	pbWorld.WorldServerServiceClient
	items    []*pbWorld.GetPlayerItemsByGuidsResponse_Item
	getItems bool // if true, Items was configured (including empty)
}

func (m *worldClientTurnInMock) GetPlayerItemsByGuids(_ context.Context, in *pbWorld.GetPlayerItemsByGuidsRequest, _ ...grpc.CallOption) (*pbWorld.GetPlayerItemsByGuidsResponse, error) {
	if m.getItems {
		return &pbWorld.GetPlayerItemsByGuidsResponse{Items: m.items}, nil
	}
	// Default: item present for each requested GUID (happy path).
	out := make([]*pbWorld.GetPlayerItemsByGuidsResponse_Item, 0, len(in.Guids))
	for _, g := range in.Guids {
		out = append(out, &pbWorld.GetPlayerItemsByGuidsResponse_Item{Guid: g, Entry: 5863})
	}
	return &pbWorld.GetPlayerItemsByGuidsResponse{Items: out}, nil
}

func (m *worldClientTurnInMock) DestroyItemsWithGuidsFromPlayer(_ context.Context, _ *pbWorld.DestroyItemsWithGuidsFromPlayerRequest, _ ...grpc.CallOption) (*pbWorld.DestroyItemsWithGuidsFromPlayerResponse, error) {
	return &pbWorld.DestroyItemsWithGuidsFromPlayerResponse{}, nil
}

func turnInPetitionSession(t *testing.T, charClient pbChar.CharactersServiceClient, guildClient pbGuild.GuildServiceClient) (*GameSession, *[]*packet.Writer, *[]*packet.Packet) {
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
		logger:               &log.Logger,
		gameSocket:           gameSocket,
		worldSocket:          worldSocket,
		character:            &LoggedInCharacter{GUID: 42},
		guildServiceClient:   guildClient,
		charServiceClient:    charClient,
		gameServerGRPCClient: &worldClientTurnInMock{},
	}

	return session, sentToClient, forwardedToWorld
}

func turnInPetitionPacket() *packet.Packet {
	return packet.NewWriter(packet.CMsgTurnInPetition).Uint64(0x4000000000000001).ToPacket()
}

func TestHandleTurnInPetitionCreatesGuild(t *testing.T) {
	charClient := &charServicePetitionMock{
		validateResp: &pbChar.ValidateGuildPetitionTurnInResponse{
			Status:         pbChar.ValidateGuildPetitionTurnInResponse_Ok,
			GuildName:      "MyGuild",
			SignatoryGUIDs: []uint64{100, 101},
		},
		deleteResp: &pbChar.DeleteGuildPetitionResponse{Status: pbChar.DeleteGuildPetitionResponse_Ok},
	}
	guildClient := &guildServiceClientCreateGuildMock{
		resp: &pbGuild.CreateGuildResponse{GuildID: 7},
	}

	session, sentToClient, _ := turnInPetitionSession(t, charClient, guildClient)

	err := session.HandleTurnInPetition(context.Background(), turnInPetitionPacket())
	assert.NoError(t, err)

	assert.Equal(t, uint64(0x4000000000000001), charClient.validateReq.PetitionItemGUID)
	assert.Equal(t, "MyGuild", guildClient.params.Name)
	assert.Equal(t, []uint64{100, 101}, guildClient.params.SignatoryGUIDs)
	assert.Equal(t, uint32(7), session.character.GuildID)

	if assert.GreaterOrEqual(t, len(*sentToClient), 2) {
		assert.Equal(t, packet.SMsgTurnInPetitionResults, (*sentToClient)[1].Opcode)
		assert.Equal(t, uint32(petitionTurnOK), (*sentToClient)[1].ToPacket().Reader().Uint32())
	}
}

func TestHandleTurnInPetitionForwardsArenaCharter(t *testing.T) {
	charClient := &charServicePetitionMock{
		validateResp: &pbChar.ValidateGuildPetitionTurnInResponse{
			Status: pbChar.ValidateGuildPetitionTurnInResponse_NotGuildPetition,
		},
	}
	guildClient := &guildServiceClientCreateGuildMock{}

	session, sentToClient, forwardedToWorld := turnInPetitionSession(t, charClient, guildClient)

	p := turnInPetitionPacket()
	err := session.HandleTurnInPetition(context.Background(), p)
	assert.NoError(t, err)
	assert.Empty(t, *sentToClient)
	if assert.Len(t, *forwardedToWorld, 1) {
		assert.Equal(t, p, (*forwardedToWorld)[0])
	}
	assert.Nil(t, guildClient.params)
}

func TestHandleTurnInPetitionBusinessStatuses(t *testing.T) {
	tests := []struct {
		name           string
		status         pbChar.ValidateGuildPetitionTurnInResponse_Status
		expectedResult uint32
	}{
		{"already in guild", pbChar.ValidateGuildPetitionTurnInResponse_AlreadyInGuild, petitionTurnAlreadyInGuild},
		{"need more signatures", pbChar.ValidateGuildPetitionTurnInResponse_NeedMoreSignatures, petitionTurnNeedMoreSignatures},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			charClient := &charServicePetitionMock{
				validateResp: &pbChar.ValidateGuildPetitionTurnInResponse{Status: tt.status},
			}
			session, sentToClient, _ := turnInPetitionSession(t, charClient, &guildServiceClientCreateGuildMock{})

			err := session.HandleTurnInPetition(context.Background(), turnInPetitionPacket())
			assert.NoError(t, err)
			if assert.Len(t, *sentToClient, 1) {
				assert.Equal(t, packet.SMsgTurnInPetitionResults, (*sentToClient)[0].Opcode)
				assert.Equal(t, tt.expectedResult, (*sentToClient)[0].ToPacket().Reader().Uint32())
			}
		})
	}
}

func TestHandleTurnInPetitionSilentStatuses(t *testing.T) {
	for _, s := range []pbChar.ValidateGuildPetitionTurnInResponse_Status{
		pbChar.ValidateGuildPetitionTurnInResponse_NotOwner,
	} {
		charClient := &charServicePetitionMock{
			validateResp: &pbChar.ValidateGuildPetitionTurnInResponse{Status: s},
		}
		session, sentToClient, forwardedToWorld := turnInPetitionSession(t, charClient, &guildServiceClientCreateGuildMock{})

		err := session.HandleTurnInPetition(context.Background(), turnInPetitionPacket())
		assert.NoError(t, err)
		assert.Empty(t, *sentToClient)
		assert.Empty(t, *forwardedToWorld)
	}
}

func TestHandleTurnInPetitionMissingCharterItem(t *testing.T) {
	charClient := &charServicePetitionMock{
		validateResp: &pbChar.ValidateGuildPetitionTurnInResponse{
			Status:         pbChar.ValidateGuildPetitionTurnInResponse_Ok,
			GuildName:      "MyGuild",
			SignatoryGUIDs: []uint64{100},
		},
	}
	guildClient := &guildServiceClientCreateGuildMock{}
	session, sentToClient, forwardedToWorld := turnInPetitionSession(t, charClient, guildClient)
	session.gameServerGRPCClient = &worldClientTurnInMock{getItems: true, items: nil}

	err := session.HandleTurnInPetition(context.Background(), turnInPetitionPacket())
	assert.NoError(t, err)
	assert.Empty(t, *sentToClient)
	assert.Empty(t, *forwardedToWorld)
	assert.Nil(t, guildClient.params)
	assert.Equal(t, uint32(0), session.character.GuildID)
}

func TestHandleTurnInPetitionNameTaken(t *testing.T) {
	charClient := &charServicePetitionMock{
		validateResp: &pbChar.ValidateGuildPetitionTurnInResponse{
			Status:    pbChar.ValidateGuildPetitionTurnInResponse_Ok,
			GuildName: "Taken",
		},
		deleteResp: &pbChar.DeleteGuildPetitionResponse{Status: pbChar.DeleteGuildPetitionResponse_Ok},
	}
	guildClient := &guildServiceClientCreateGuildMock{
		err: status.Error(codes.AlreadyExists, "name taken"),
	}

	session, sentToClient, _ := turnInPetitionSession(t, charClient, guildClient)

	err := session.HandleTurnInPetition(context.Background(), turnInPetitionPacket())
	assert.NoError(t, err)
	if assert.Len(t, *sentToClient, 1) {
		reader := (*sentToClient)[0].ToPacket().Reader()
		assert.Equal(t, uint32(guildCommandCreate), reader.Uint32())
		_ = reader.String()
		assert.Equal(t, uint32(guildErrNameExistsS), reader.Uint32())
	}
}

func TestHandleTurnInPetitionCreateFailure(t *testing.T) {
	charClient := &charServicePetitionMock{
		validateResp: &pbChar.ValidateGuildPetitionTurnInResponse{
			Status:    pbChar.ValidateGuildPetitionTurnInResponse_Ok,
			GuildName: "X",
		},
	}
	guildClient := &guildServiceClientCreateGuildMock{
		err: status.Error(codes.Internal, "boom"),
	}

	session, _, _ := turnInPetitionSession(t, charClient, guildClient)
	err := session.HandleTurnInPetition(context.Background(), turnInPetitionPacket())
	assert.Error(t, err)
}

func TestHandlePetitionSignForwardsArena(t *testing.T) {
	charClient := &charServicePetitionMock{
		getPetitionResp: &pbChar.GetGuildPetitionResponse{
			Status: pbChar.GetGuildPetitionResponse_NotGuildPetition,
		},
	}
	session, _, forwardedToWorld := turnInPetitionSession(t, charClient, &guildServiceClientCreateGuildMock{})

	p := packet.NewWriter(packet.CMsgPetitionSign).Uint64(0x4000000000000002).Uint8(0).ToPacket()
	err := session.HandlePetitionSign(context.Background(), p)
	assert.NoError(t, err)
	if assert.Len(t, *forwardedToWorld, 1) {
		assert.Equal(t, p, (*forwardedToWorld)[0])
	}
}

func TestHandlePetitionSignOK(t *testing.T) {
	charClient := &charServicePetitionMock{
		getPetitionResp: &pbChar.GetGuildPetitionResponse{
			Status: pbChar.GetGuildPetitionResponse_Ok,
			Petition: &pbChar.GuildPetition{
				PetitionItemGUID: 0x4000000000000001,
				OwnerGUID:        1,
				PetitionID:       55,
				Type:             9,
			},
		},
		addSigResp: &pbChar.AddGuildPetitionSignatureResponse{
			Result:           pbChar.AddGuildPetitionSignatureResponse_Ok,
			OwnerGUID:        1,
			PetitionItemGUID: 0x4000000000000001,
			SignerGUID:       42,
		},
	}
	session, sentToClient, forwardedToWorld := turnInPetitionSession(t, charClient, &guildServiceClientCreateGuildMock{})
	session.accountID = 7

	p := packet.NewWriter(packet.CMsgPetitionSign).Uint64(0x4000000000000001).Uint8(0).ToPacket()
	err := session.HandlePetitionSign(context.Background(), p)
	assert.NoError(t, err)
	assert.Empty(t, *forwardedToWorld)
	assert.Equal(t, uint32(7), charClient.addSigReq.SignerAccountID)
	if assert.Len(t, *sentToClient, 1) {
		assert.Equal(t, packet.SMsgPetitionSignResults, (*sentToClient)[0].Opcode)
		r := (*sentToClient)[0].ToPacket().Reader()
		assert.Equal(t, uint64(0x4000000000000001), r.Uint64())
		assert.Equal(t, uint64(42), r.Uint64())
		assert.Equal(t, uint32(petitionSignOK), r.Uint32())
	}
}

func TestHandlePetitionShowSignaturesGuild(t *testing.T) {
	charClient := &charServicePetitionMock{
		getSigsResp: &pbChar.GetGuildPetitionSignaturesResponse{
			Status: pbChar.GetGuildPetitionSignaturesResponse_Ok,
			Petition: &pbChar.GuildPetition{
				PetitionID: 12,
			},
			Signatures: []*pbChar.GuildPetitionSignature{
				{PlayerGUID: 100},
				{PlayerGUID: 101},
			},
		},
	}
	session, sentToClient, forwardedToWorld := turnInPetitionSession(t, charClient, &guildServiceClientCreateGuildMock{})

	p := packet.NewWriter(packet.CMsgPetitionShowSignatures).Uint64(0x4000000000000001).ToPacket()
	err := session.HandlePetitionShowSignatures(context.Background(), p)
	assert.NoError(t, err)
	assert.Empty(t, *forwardedToWorld)
	if assert.Len(t, *sentToClient, 1) {
		assert.Equal(t, packet.SMsgPetitionShowSignatures, (*sentToClient)[0].Opcode)
		r := (*sentToClient)[0].ToPacket().Reader()
		assert.Equal(t, uint64(0x4000000000000001), r.Uint64())
		assert.Equal(t, uint64(42), r.Uint64()) // requester as owner field
		assert.Equal(t, uint32(12), r.Uint32())
		assert.Equal(t, uint8(2), r.Uint8())
	}
}

func TestHandlePetitionShowSignaturesForwardsArena(t *testing.T) {
	charClient := &charServicePetitionMock{
		getSigsResp: &pbChar.GetGuildPetitionSignaturesResponse{
			Status: pbChar.GetGuildPetitionSignaturesResponse_NotGuildPetition,
		},
	}
	session, sentToClient, forwardedToWorld := turnInPetitionSession(t, charClient, &guildServiceClientCreateGuildMock{})

	p := packet.NewWriter(packet.CMsgPetitionShowSignatures).Uint64(0x4000000000000009).ToPacket()
	err := session.HandlePetitionShowSignatures(context.Background(), p)
	assert.NoError(t, err)
	assert.Empty(t, *sentToClient)
	if assert.Len(t, *forwardedToWorld, 1) {
		assert.Equal(t, p, (*forwardedToWorld)[0])
	}
}

func TestHandleOfferPetitionAllowsTargetWhoLeftGuild(t *testing.T) {
	charClient := &charServicePetitionMock{
		getSigsResp: &pbChar.GetGuildPetitionSignaturesResponse{
			Status:   pbChar.GetGuildPetitionSignaturesResponse_Ok,
			Petition: &pbChar.GuildPetition{PetitionID: 77, OwnerGUID: 42},
		},
		onlineChars: []*pbChar.ShortCharactersDataByGUIDsResponse_ShortCharData{
			{CharGUID: 99, CharName: "Jaina", CharRace: 1, CharGuildID: 5},
		},
		loginChar: &pbChar.LogInCharacter{GUID: 99, Name: "Jaina", Race: 1, GuildID: 0},
	}
	broadcaster := eBroadcasterMocks.NewBroadcaster(t)
	broadcaster.On("NewPetitionOfferedEvent", mock.Anything).Return()
	session, sentToClient, _ := turnInPetitionSession(t, charClient, &guildServiceClientCreateGuildMock{})
	session.character = &LoggedInCharacter{GUID: 42, Name: "Owner", Race: 1}
	session.eventsBroadcaster = broadcaster

	p := packet.NewWriter(packet.CMsgOfferPetition).
		Uint32(0).
		Uint64(0x4000000000000001).
		Uint64(99).
		ToPacket()
	err := session.HandleOfferPetition(context.Background(), p)
	assert.NoError(t, err)
	for _, w := range *sentToClient {
		assert.NotEqual(t, packet.SMsgGuildCommandResult, w.Opcode)
	}
	broadcaster.AssertCalled(t, "NewPetitionOfferedEvent", mock.Anything)
}

func TestHandleOfferPetitionNamesTargetWhenAlreadyInGuild(t *testing.T) {
	charClient := &charServicePetitionMock{
		getSigsResp: &pbChar.GetGuildPetitionSignaturesResponse{
			Status:   pbChar.GetGuildPetitionSignaturesResponse_Ok,
			Petition: &pbChar.GuildPetition{PetitionID: 77, OwnerGUID: 42},
		},
		onlineChars: []*pbChar.ShortCharactersDataByGUIDsResponse_ShortCharData{
			{CharGUID: 99, CharName: "Jaina", CharRace: 1, CharGuildID: 5},
		},
		loginChar: &pbChar.LogInCharacter{GUID: 99, Name: "Jaina", Race: 1, GuildID: 5},
	}
	session, sentToClient, _ := turnInPetitionSession(t, charClient, &guildServiceClientCreateGuildMock{})
	session.character = &LoggedInCharacter{GUID: 42, Name: "Owner", Race: 1}

	p := packet.NewWriter(packet.CMsgOfferPetition).
		Uint32(0).
		Uint64(0x4000000000000001).
		Uint64(99).
		ToPacket()
	err := session.HandleOfferPetition(context.Background(), p)
	assert.NoError(t, err)
	if assert.Len(t, *sentToClient, 1) {
		assert.Equal(t, packet.SMsgGuildCommandResult, (*sentToClient)[0].Opcode)
		r := (*sentToClient)[0].ToPacket().Reader()
		assert.Equal(t, uint32(guildCommandInvite), r.Uint32())
		assert.Equal(t, "Jaina", r.String())
		assert.Equal(t, uint32(guildErrAlreadyInGuildS), r.Uint32())
	}
}
