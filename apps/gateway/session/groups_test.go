package session

import (
	"context"
	"testing"

	"github.com/rs/zerolog/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"google.golang.org/grpc"

	root "github.com/walkline/ToCloud9/apps/gateway"
	"github.com/walkline/ToCloud9/apps/gateway/packet"
	mocks "github.com/walkline/ToCloud9/apps/gateway/sockets/socketmock"
	pbGroup "github.com/walkline/ToCloud9/gen/group/pb"
	"github.com/walkline/ToCloud9/shared/events"
)

type groupServiceClientByMemberMock struct {
	pbGroup.GroupServiceClient
	resp *pbGroup.GetGroupResponse
	err  error

	calls int
}

func (m *groupServiceClientByMemberMock) GetGroupByMember(_ context.Context, _ *pbGroup.GetGroupByMemberRequest, _ ...grpc.CallOption) (*pbGroup.GetGroupResponse, error) {
	m.calls++
	return m.resp, m.err
}

func partyMemberStatsSession(t *testing.T, groupClient pbGroup.GroupServiceClient) (*GameSession, *[]*packet.Packet, *[]*packet.Packet) {
	t.Helper()

	sentToClient := &[]*packet.Packet{}
	gameSocket := &mocks.Socket{}
	gameSocket.On("SendPacket", mock.Anything).Run(func(args mock.Arguments) {
		*sentToClient = append(*sentToClient, args.Get(0).(*packet.Packet))
	}).Return()

	forwardedToWorld := &[]*packet.Packet{}
	worldSocket := &mocks.Socket{}
	worldSocket.On("SendPacket", mock.Anything).Run(func(args mock.Arguments) {
		*forwardedToWorld = append(*forwardedToWorld, args.Get(0).(*packet.Packet))
	}).Return()

	session := &GameSession{
		logger:             &log.Logger,
		gameSocket:         gameSocket,
		worldSocket:        worldSocket,
		character:          &LoggedInCharacter{GUID: 42},
		groupServiceClient: groupClient,
	}

	return session, sentToClient, forwardedToWorld
}

func requestPartyMemberStatsPacket(guid uint64) *packet.Packet {
	return packet.NewWriter(packet.CMsgRequestPartyMemberStats).Uint64(guid).ToPacket()
}

func uint8Ptr(v uint8) *uint8    { return &v }
func uint32Ptr(v uint32) *uint32 { return &v }

func completeStats(guid uint64) events.GroupMemberStatsUpdate {
	return events.GroupMemberStatsUpdate{
		MemberGUID: guid,
		Level:      uint8Ptr(10),
		Zone:       uint32Ptr(440),
		CurHP:      uint32Ptr(230),
		MaxHP:      uint32Ptr(260),
		PowerType:  uint8Ptr(1),
		CurPower:   uint32Ptr(80),
		MaxPower:   uint32Ptr(100),
	}
}

func TestHandleRequestPartyMemberStatsFullWhenCacheComplete(t *testing.T) {
	groupClient := &groupServiceClientByMemberMock{}
	session, sentToClient, forwardedToWorld := partyMemberStatsSession(t, groupClient)

	const memberGUID = uint64(60553)
	session.storeGroupMemberStats(&events.GroupMemberStatsUpdate{MemberGUID: memberGUID})
	stats := completeStats(memberGUID)
	session.storeGroupMemberStats(&stats)

	err := session.HandleRequestPartyMemberStats(context.Background(), requestPartyMemberStatsPacket(memberGUID))
	assert.Nil(t, err)

	assert.Empty(t, *forwardedToWorld)
	assert.Zero(t, groupClient.calls)
	if assert.Len(t, *sentToClient, 1) {
		assert.Equal(t, packet.SMsgPartyMemberStatsFull, (*sentToClient)[0].Opcode)
	}
}

func TestHandleRequestPartyMemberStatsIncrementalWhenCachePartial(t *testing.T) {
	groupClient := &groupServiceClientByMemberMock{}
	session, sentToClient, forwardedToWorld := partyMemberStatsSession(t, groupClient)

	// A member that only ever published its zone (typical for a bot that
	// walked without taking damage) must not get a FULL answer: the missing
	// HP fields would be reset to 0 client side.
	const memberGUID = uint64(60553)
	session.storeGroupMemberStats(&events.GroupMemberStatsUpdate{
		MemberGUID: memberGUID,
		Zone:       uint32Ptr(440),
	})

	err := session.HandleRequestPartyMemberStats(context.Background(), requestPartyMemberStatsPacket(memberGUID))
	assert.Nil(t, err)

	assert.Empty(t, *forwardedToWorld)
	if assert.Len(t, *sentToClient, 1) {
		assert.Equal(t, packet.SMsgPartyMemberStats, (*sentToClient)[0].Opcode)
	}
}

func TestHandleRequestPartyMemberStatsCacheMissKnownMember(t *testing.T) {
	const memberGUID = uint64(60553)
	groupClient := &groupServiceClientByMemberMock{
		resp: &pbGroup.GetGroupResponse{
			Api: root.SupportedGroupServiceVer,
			Group: &pbGroup.GetGroupResponse_Group{
				Members: []*pbGroup.GetGroupResponse_GroupMember{
					{Guid: 42, IsOnline: true},
					{Guid: memberGUID, IsOnline: true},
				},
			},
		},
	}
	session, sentToClient, forwardedToWorld := partyMemberStatsSession(t, groupClient)

	err := session.HandleRequestPartyMemberStats(context.Background(), requestPartyMemberStatsPacket(memberGUID))
	assert.Nil(t, err)

	assert.Empty(t, *forwardedToWorld)
	assert.Equal(t, 1, groupClient.calls)
	if assert.Len(t, *sentToClient, 1) {
		assert.Equal(t, packet.SMsgPartyMemberStats, (*sentToClient)[0].Opcode)
	}
}

func TestHandleRequestPartyMemberStatsCacheMissOfflineMemberFallsThrough(t *testing.T) {
	// A group member that is not connected must keep the game server "offline"
	// stub path: answering with a fabricated status would show a disconnected
	// member as online with a full health bar.
	const memberGUID = uint64(60553)
	groupClient := &groupServiceClientByMemberMock{
		resp: &pbGroup.GetGroupResponse{
			Api: root.SupportedGroupServiceVer,
			Group: &pbGroup.GetGroupResponse_Group{
				Members: []*pbGroup.GetGroupResponse_GroupMember{
					{Guid: 42, IsOnline: true},
					{Guid: memberGUID, IsOnline: false},
				},
			},
		},
	}
	session, sentToClient, forwardedToWorld := partyMemberStatsSession(t, groupClient)

	p := requestPartyMemberStatsPacket(memberGUID)
	err := session.HandleRequestPartyMemberStats(context.Background(), p)
	assert.Nil(t, err)

	assert.Empty(t, *sentToClient)
	assert.Equal(t, 1, groupClient.calls)
	if assert.Len(t, *forwardedToWorld, 1) {
		assert.Equal(t, p, (*forwardedToWorld)[0])
	}
}

func TestHandleRequestPartyMemberStatsCacheMissStrangerFallsThrough(t *testing.T) {
	groupClient := &groupServiceClientByMemberMock{
		resp: &pbGroup.GetGroupResponse{
			Api: root.SupportedGroupServiceVer,
			Group: &pbGroup.GetGroupResponse_Group{
				Members: []*pbGroup.GetGroupResponse_GroupMember{{Guid: 42}},
			},
		},
	}
	session, sentToClient, forwardedToWorld := partyMemberStatsSession(t, groupClient)

	p := requestPartyMemberStatsPacket(60553)
	err := session.HandleRequestPartyMemberStats(context.Background(), p)
	assert.Nil(t, err)

	assert.Empty(t, *sentToClient)
	if assert.Len(t, *forwardedToWorld, 1) {
		assert.Equal(t, p, (*forwardedToWorld)[0])
	}
}

func TestHandleRequestPartyMemberStatsPetGUIDFallsThrough(t *testing.T) {
	groupClient := &groupServiceClientByMemberMock{}
	session, sentToClient, forwardedToWorld := partyMemberStatsSession(t, groupClient)

	// 3.3.5 pet full GUIDs carry the 0xF140 high type: no group lookup for those.
	p := requestPartyMemberStatsPacket(0xF140000000001234)
	err := session.HandleRequestPartyMemberStats(context.Background(), p)
	assert.Nil(t, err)

	assert.Empty(t, *sentToClient)
	assert.Zero(t, groupClient.calls)
	if assert.Len(t, *forwardedToWorld, 1) {
		assert.Equal(t, p, (*forwardedToWorld)[0])
	}
}

func TestHandleRequestPartyMemberStatsGameServerManagedGroup(t *testing.T) {
	groupClient := &groupServiceClientByMemberMock{}
	session, sentToClient, forwardedToWorld := partyMemberStatsSession(t, groupClient)
	session.character.GroupMangedByGameServer = true

	p := requestPartyMemberStatsPacket(60553)
	err := session.HandleRequestPartyMemberStats(context.Background(), p)
	assert.Nil(t, err)

	assert.Empty(t, *sentToClient)
	assert.Zero(t, groupClient.calls)
	if assert.Len(t, *forwardedToWorld, 1) {
		assert.Equal(t, p, (*forwardedToWorld)[0])
	}
}
