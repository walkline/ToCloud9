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

	eBroadcaster "github.com/walkline/ToCloud9/apps/gateway/events-broadcaster"
	"github.com/walkline/ToCloud9/apps/gateway/packet"
	mocks "github.com/walkline/ToCloud9/apps/gateway/sockets/socketmock"
	pbChar "github.com/walkline/ToCloud9/gen/characters/pb"
	pbGuild "github.com/walkline/ToCloud9/gen/guilds/pb"
	pbWorld "github.com/walkline/ToCloud9/gen/worldserver/pb"
	"github.com/walkline/ToCloud9/shared/events"
	"github.com/walkline/ToCloud9/shared/wow"
)

type charServiceClientOnlineByNameMock struct {
	pbChar.CharactersServiceClient
	resp *pbChar.CharacterOnlineByNameResponse
	err  error
}

func (m *charServiceClientOnlineByNameMock) CharacterOnlineByName(_ context.Context, _ *pbChar.CharacterOnlineByNameRequest, _ ...grpc.CallOption) (*pbChar.CharacterOnlineByNameResponse, error) {
	return m.resp, m.err
}

type guildServiceClientInviteMock struct {
	pbGuild.GuildServiceClient
	inviteErr    error
	inviteCalled bool
	rosterResp   *pbGuild.GetRosterInfoResponse
}

func (m *guildServiceClientInviteMock) InviteMember(_ context.Context, _ *pbGuild.InviteMemberParams, _ ...grpc.CallOption) (*pbGuild.InviteMemberResponse, error) {
	m.inviteCalled = true
	if m.inviteErr != nil {
		return nil, m.inviteErr
	}
	return &pbGuild.InviteMemberResponse{}, nil
}

func (m *guildServiceClientInviteMock) GetRosterInfo(_ context.Context, _ *pbGuild.GetRosterInfoParams, _ ...grpc.CallOption) (*pbGuild.GetRosterInfoResponse, error) {
	return m.rosterResp, nil
}

func (m *guildServiceClientInviteMock) GetBankState(_ context.Context, _ *pbGuild.GetBankStateParams, _ ...grpc.CallOption) (*pbGuild.GetBankStateResponse, error) {
	// Mirrors the rank of promoteRoster so the permissions assertions hold.
	return &pbGuild.GetBankStateResponse{RankID: 1, RankRights: 0xFFF, MoneyPerDay: 100}, nil
}

func guildTestSession(t *testing.T, guildClient pbGuild.GuildServiceClient, charClient pbChar.CharactersServiceClient) (*GameSession, *[]*packet.Writer, *worldServerGuildFieldsMock) {
	t.Helper()

	sentToClient := &[]*packet.Writer{}
	worldClient := &worldServerGuildFieldsMock{}
	gameSocket := &mocks.Socket{}
	gameSocket.On("Send", mock.Anything).Run(func(args mock.Arguments) {
		*sentToClient = append(*sentToClient, args.Get(0).(*packet.Writer))
	}).Return()

	session := &GameSession{
		logger:             &log.Logger,
		gameSocket:         gameSocket,
		character:          &LoggedInCharacter{GUID: 42, GuildID: 7},
		guildServiceClient: guildClient,
		charServiceClient:  charClient,

		gameServerGRPCClient: worldClient,
	}

	return session, sentToClient, worldClient
}

// worldServerGuildFieldsMock answers SetPlayerGuildFields, which the rank
// refresh calls before touching the client UI.
type worldServerGuildFieldsMock struct {
	pbWorld.WorldServerServiceClient
	lastRequest *pbWorld.SetPlayerGuildFieldsRequest
}

func (m *worldServerGuildFieldsMock) SetPlayerGuildFields(_ context.Context, in *pbWorld.SetPlayerGuildFieldsRequest, _ ...grpc.CallOption) (*pbWorld.SetPlayerGuildFieldsResponse, error) {
	m.lastRequest = in
	return &pbWorld.SetPlayerGuildFieldsResponse{Applied: true}, nil
}

func promoteRoster() *pbGuild.GetRosterInfoResponse {
	return &pbGuild.GetRosterInfoResponse{
		Guild: &pbGuild.GetRosterInfoResponse_Guild{
			Members: []*pbGuild.GetRosterInfoResponse_Member{
				{Guid: 42, RankID: 1},
			},
			Ranks: []*pbGuild.GetRosterInfoResponse_Rank{
				{Id: 1, Flags: 0xFFF, GoldLimit: 100},
			},
		},
	}
}

func TestPromoteEventPushesPermissionsToPromotedMember(t *testing.T) {
	guildClient := &guildServiceClientInviteMock{rosterResp: promoteRoster()}
	session, sentToClient, worldClient := guildTestSession(t, guildClient, nil)

	err := session.HandleEventGuildMemberPromoted(context.Background(), &eBroadcaster.Event{
		Payload: &events.GuildEventMemberPromotePayload{
			RankName:     "Officer",
			PromoterName: "Leader",
			MemberName:   "Member",
			MemberGUID:   42,
			RankID:       1,
		},
	})
	assert.Nil(t, err)

	// The unit field carries the rank the client gates its buttons on.
	if assert.NotNil(t, worldClient.lastRequest) {
		assert.Equal(t, uint64(42), worldClient.lastRequest.PlayerGuid)
		assert.Equal(t, uint32(1), worldClient.lastRequest.Rank)
	}

	if assert.Len(t, *sentToClient, 3) {
		assert.Equal(t, packet.SMsgGuildEvent, (*sentToClient)[0].Opcode)
		assert.Equal(t, packet.MsgGuildPermissions, (*sentToClient)[1].Opcode)
		assert.Equal(t, packet.SMsgGuildRoster, (*sentToClient)[2].Opcode)
		r := (*sentToClient)[1].ToPacket().Reader()
		assert.Equal(t, uint32(1), r.Uint32())     // rank id
		assert.Equal(t, uint32(0xFFF), r.Uint32()) // rank flags
	}
}

func TestPromoteEventOtherMemberNoPermissionsPush(t *testing.T) {
	guildClient := &guildServiceClientInviteMock{rosterResp: promoteRoster()}
	session, sentToClient, worldClient := guildTestSession(t, guildClient, nil)

	err := session.HandleEventGuildMemberPromoted(context.Background(), &eBroadcaster.Event{
		Payload: &events.GuildEventMemberPromotePayload{
			RankName:   "Officer",
			MemberName: "Member",
			MemberGUID: 51,
		},
	})
	assert.Nil(t, err)

	assert.Nil(t, worldClient.lastRequest, "another member's rank change must not touch our unit fields")

	if assert.Len(t, *sentToClient, 1) {
		assert.Equal(t, packet.SMsgGuildEvent, (*sentToClient)[0].Opcode)
	}
}

func TestDemoteEventPushesPermissionsToDemotedMember(t *testing.T) {
	guildClient := &guildServiceClientInviteMock{rosterResp: promoteRoster()}
	session, sentToClient, worldClient := guildTestSession(t, guildClient, nil)

	err := session.HandleEventGuildMemberDemoted(context.Background(), &eBroadcaster.Event{
		Payload: &events.GuildEventMemberDemotePayload{
			RankName:    "Initiate",
			DemoterName: "Leader",
			MemberName:  "Member",
			MemberGUID:  42,
			RankID:      1,
		},
	})
	assert.Nil(t, err)

	if assert.NotNil(t, worldClient.lastRequest) {
		assert.Equal(t, uint32(1), worldClient.lastRequest.Rank)
	}

	if assert.Len(t, *sentToClient, 3) {
		assert.Equal(t, packet.MsgGuildPermissions, (*sentToClient)[1].Opcode)
		assert.Equal(t, packet.SMsgGuildRoster, (*sentToClient)[2].Opcode)
	}
}
func guildInvitePacket(name string) *packet.Packet {
	return packet.NewWriter(packet.CMsgGuildInvite).String(name).ToPacket()
}

func TestHandleGuildInviteMapsBusinessErrors(t *testing.T) {
	tests := []struct {
		name       string
		inviteErr  error
		wantResult uint32
		wantParam  string
	}{
		{"invitee already in guild", status.Error(codes.FailedPrecondition, "already in guild"), guildErrAlreadyInGuildS, "Thrall"},
		{"no invite permission", status.Error(codes.PermissionDenied, "not enough rights"), guildErrPermissions, ""},
		{"inviter not in a guild", status.Error(codes.NotFound, "guild not found"), guildErrPlayerNotInGuild, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			charClient := &charServiceClientOnlineByNameMock{
				resp: &pbChar.CharacterOnlineByNameResponse{
					Character: &pbChar.CharacterOnlineByNameResponse_Char{
						CharGUID: 51,
						CharName: "Thrall",
					},
				},
			}
			guildClient := &guildServiceClientInviteMock{inviteErr: tt.inviteErr}
			session, sentToClient, _ := guildTestSession(t, guildClient, charClient)

			err := session.HandleGuildInvite(context.Background(), guildInvitePacket("Thrall"))
			assert.Nil(t, err)

			if assert.Len(t, *sentToClient, 1) {
				assert.Equal(t, packet.SMsgGuildCommandResult, (*sentToClient)[0].Opcode)
				r := (*sentToClient)[0].ToPacket().Reader()
				assert.Equal(t, uint32(guildCommandInvite), r.Uint32())
				assert.Equal(t, tt.wantParam, r.String())
				assert.Equal(t, tt.wantResult, r.Uint32())
			}
		})
	}
}

func TestHandleGuildInviteUnknownErrorStaysAnError(t *testing.T) {
	charClient := &charServiceClientOnlineByNameMock{
		resp: &pbChar.CharacterOnlineByNameResponse{
			Character: &pbChar.CharacterOnlineByNameResponse_Char{CharGUID: 51, CharName: "Thrall"},
		},
	}
	guildClient := &guildServiceClientInviteMock{inviteErr: status.Error(codes.Internal, "boom")}
	session, sentToClient, _ := guildTestSession(t, guildClient, charClient)

	err := session.HandleGuildInvite(context.Background(), guildInvitePacket("Thrall"))
	assert.NotNil(t, err)
	assert.Empty(t, *sentToClient)
}

func TestHandleGuildInviteRefusesTheOtherFaction(t *testing.T) {
	tests := []struct {
		name        string
		inviterRace uint8
		inviteeRace uint32
		allowCross  bool
		wantRefused bool
	}{
		{"alliance invites horde", uint8(wow.RaceIDNightElf), uint32(wow.RaceIDBloodElf), false, true},
		{"horde invites alliance", uint8(wow.RaceIDOrc), uint32(wow.RaceIDDwarf), false, true},
		{"same faction goes through", uint8(wow.RaceIDOrc), uint32(wow.RaceIDTauren), false, false},
		{"cross faction allowed by config", uint8(wow.RaceIDNightElf), uint32(wow.RaceIDBloodElf), true, false},
		{"unknown race is not refused", 0, 0, false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			charClient := &charServiceClientOnlineByNameMock{
				resp: &pbChar.CharacterOnlineByNameResponse{
					Character: &pbChar.CharacterOnlineByNameResponse_Char{
						CharGUID: 51,
						CharName: "Thrall",
						CharRace: tt.inviteeRace,
					},
				},
			}
			guildClient := &guildServiceClientInviteMock{}
			session, sentToClient, _ := guildTestSession(t, guildClient, charClient)
			session.character.Race = tt.inviterRace
			session.allowCrossFactionGuilds = tt.allowCross

			err := session.HandleGuildInvite(context.Background(), guildInvitePacket("Thrall"))
			assert.Nil(t, err)

			if !tt.wantRefused {
				assert.True(t, guildClient.inviteCalled, "the invite must reach the guild service")
				return
			}

			assert.False(t, guildClient.inviteCalled, "no invite must reach the guild service")
			if assert.Len(t, *sentToClient, 1) {
				assert.Equal(t, packet.SMsgGuildCommandResult, (*sentToClient)[0].Opcode)
				r := (*sentToClient)[0].ToPacket().Reader()
				assert.Equal(t, uint32(guildCommandInvite), r.Uint32())
				assert.Equal(t, "Thrall", r.String())
				assert.Equal(t, uint32(guildErrNotAllied), r.Uint32())
			}
		})
	}
}
func TestGuildDisbandIsRefusedAndNotForwarded(t *testing.T) {
	session, sentToClient, _ := guildTestSession(t, nil, nil)

	err := session.HandleGuildDisband(context.Background(), nil)
	assert.Nil(t, err)

	// A system message goes back to the client and nothing reaches the
	// worldserver, which would otherwise disband the guild on stale state.
	if assert.Len(t, *sentToClient, 1) {
		assert.Equal(t, packet.SMsgMessageChat, (*sentToClient)[0].Opcode)
	}
}

func TestGuildLeaderChangeIsRefusedAndNotForwarded(t *testing.T) {
	session, sentToClient, _ := guildTestSession(t, nil, nil)

	err := session.HandleGuildLeaderChange(context.Background(), nil)
	assert.Nil(t, err)

	if assert.Len(t, *sentToClient, 1) {
		assert.Equal(t, packet.SMsgMessageChat, (*sentToClient)[0].Opcode)
	}
}

func TestGuildlessPlayerGetsNoRefusalMessage(t *testing.T) {
	session, sentToClient, _ := guildTestSession(t, nil, nil)
	session.character.GuildID = 0

	assert.Nil(t, session.HandleGuildDisband(context.Background(), nil))
	assert.Nil(t, session.HandleGuildLeaderChange(context.Background(), nil))
	assert.Empty(t, *sentToClient)
}
