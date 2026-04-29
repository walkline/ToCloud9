package session

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	eBroadcaster "github.com/walkline/ToCloud9/apps/gateway/events-broadcaster"
	"github.com/walkline/ToCloud9/apps/gateway/packet"
	mocks "github.com/walkline/ToCloud9/apps/gateway/sockets/socketmock"
	pbChat "github.com/walkline/ToCloud9/gen/chat/pb"
	chatMocks "github.com/walkline/ToCloud9/gen/chat/pb/mocks"
)

func TestChannelMembershipAddChannel(t *testing.T) {
	broadcaster := eBroadcaster.NewChatChannelsService()
	cm := NewChannelMembership(12345, broadcaster)

	cm.AddChannel("Trade", 2, ChannelFlagTrade)

	assert.True(t, cm.IsMember("Trade"))
	ch := cm.GetChannel("Trade")
	assert.NotNil(t, ch)
	assert.Equal(t, "Trade", ch.Name)
	assert.Equal(t, uint32(2), ch.ChannelID)
	assert.Equal(t, ChannelFlagTrade, ch.Flags)
}

func TestChannelMembershipRemoveChannel(t *testing.T) {
	broadcaster := eBroadcaster.NewChatChannelsService()
	cm := NewChannelMembership(12345, broadcaster)

	cm.AddChannel("Trade", 2, ChannelFlagTrade)
	assert.True(t, cm.IsMember("Trade"))

	cm.RemoveChannel("Trade")
	assert.False(t, cm.IsMember("Trade"))
	assert.Nil(t, cm.GetChannel("Trade"))
}

func TestChannelMembershipGetAllChannels(t *testing.T) {
	broadcaster := eBroadcaster.NewChatChannelsService()
	cm := NewChannelMembership(12345, broadcaster)

	cm.AddChannel("Trade", 2, ChannelFlagTrade)
	cm.AddChannel("General", 1, ChannelFlagGeneral)
	cm.AddChannel("CustomChannel", 0, ChannelFlagCustom)

	channels := cm.GetAllChannels()
	assert.Len(t, channels, 3)
}

func TestChannelMembershipInitialChannels(t *testing.T) {
	broadcaster := eBroadcaster.NewChatChannelsService()
	cm := NewChannelMembership(12345, broadcaster)

	// First channels added quickly should be tracked as initial
	cm.AddChannel("General", 1, ChannelFlagGeneral)
	time.Sleep(10 * time.Millisecond)
	cm.AddChannel("Trade", 2, ChannelFlagTrade)

	assert.Len(t, cm.initialChannels, 2)
	assert.False(t, cm.initialChannelsLoaded)

	// Wait for timeout
	time.Sleep(6 * time.Second)
	cm.AddChannel("LateChannel", 3, ChannelFlagCustom)

	assert.True(t, cm.initialChannelsLoaded)
	assert.Len(t, cm.initialChannels, 2) // Late channel not added
}

func TestChannelNotifyBuilderJoined(t *testing.T) {
	gameSocket := &mocks.Socket{}
	gameSocket.On("Send", mock.MatchedBy(func(w *packet.Writer) bool {
		return w.Opcode == packet.SMsgChannelNotify
	})).Return()

	session := &GameSession{
		gameSocket: gameSocket,
	}

	ch := &ChannelInfo{Name: "TestChannel", ChannelID: 1, Flags: ChannelFlagCustom}
	session.ChannelNotify(ch).Joined(12345)

	gameSocket.AssertCalled(t, "Send", mock.Anything)
}

func TestChannelNotifyBuilderYouJoined(t *testing.T) {
	gameSocket := &mocks.Socket{}
	gameSocket.On("Send", mock.MatchedBy(func(w *packet.Writer) bool {
		return w.Opcode == packet.SMsgChannelNotify
	})).Return()

	session := &GameSession{
		gameSocket: gameSocket,
	}

	ch := &ChannelInfo{Name: "TestChannel", ChannelID: 1, Flags: ChannelFlagCustom}
	session.ChannelNotify(ch).YouJoined()

	gameSocket.AssertCalled(t, "Send", mock.Anything)
}

func TestChannelNotifyBuilderModeChange(t *testing.T) {
	gameSocket := &mocks.Socket{}
	gameSocket.On("Send", mock.MatchedBy(func(w *packet.Writer) bool {
		return w.Opcode == packet.SMsgChannelNotify
	})).Return()

	session := &GameSession{
		gameSocket: gameSocket,
	}

	ch := &ChannelInfo{Name: "TestChannel", ChannelID: 0, Flags: ChannelFlagCustom}
	session.ChannelNotify(ch).ModeChange(12345, MemberFlagModerator, MemberFlagModerator|MemberFlagOwner)

	gameSocket.AssertCalled(t, "Send", mock.Anything)
}

func TestHandleJoinChannelCustomSuccess(t *testing.T) {
	chatClient := &chatMocks.ChatServiceClient{}
	chatClient.On("JoinChannel", mock.Anything, mock.MatchedBy(func(req *pbChat.JoinChannelRequest) bool {
		return req.ChannelName == "MyCustomChannel" && req.Password == "secret"
	})).Return(&pbChat.JoinChannelResponse{
		Status: pbChat.JoinChannelResponse_Ok,
		Channel: &pbChat.ChannelInfo{
			ChannelID:   0,
			Flags:       uint32(ChannelFlagCustom),
			MemberFlags: uint32(MemberFlagOwner),
		},
	}, nil)

	gameSocket := &mocks.Socket{}
	gameSocket.On("Send", mock.Anything).Return()

	broadcaster := eBroadcaster.NewChatChannelsService()
	session := &GameSession{
		gameSocket:        gameSocket,
		chatServiceClient: chatClient,
		channelMembership: NewChannelMembership(12345, broadcaster),
		character:         &LoggedInCharacter{GUID: 12345, Name: "TestPlayer", Race: 1},
	}

	p := packet.NewWriter(packet.CMsgJoinChannel).
		Uint32(0). // channelID (0 = custom)
		Uint8(0).  // unknown1
		Uint8(0).  // unknown2
		String("MyCustomChannel").
		String("secret").
		ToPacket()

	err := session.HandleJoinChannel(context.Background(), p)
	assert.Nil(t, err)
	assert.True(t, session.channelMembership.IsMember("MyCustomChannel"))
	chatClient.AssertExpectations(t)
}

func TestHandleJoinChannelWrongPassword(t *testing.T) {
	chatClient := &chatMocks.ChatServiceClient{}
	chatClient.On("JoinChannel", mock.Anything, mock.Anything).Return(&pbChat.JoinChannelResponse{
		Status: pbChat.JoinChannelResponse_WrongPassword,
	}, nil)

	gameSocket := &mocks.Socket{}
	gameSocket.On("Send", mock.Anything).Return()

	broadcaster := eBroadcaster.NewChatChannelsService()
	session := &GameSession{
		gameSocket:        gameSocket,
		chatServiceClient: chatClient,
		channelMembership: NewChannelMembership(12345, broadcaster),
		character:         &LoggedInCharacter{GUID: 12345, Name: "TestPlayer", Race: 1},
	}

	p := packet.NewWriter(packet.CMsgJoinChannel).
		Uint32(0).
		Uint8(0).
		Uint8(0).
		String("SecretChannel").
		String("wrongpass").
		ToPacket()

	err := session.HandleJoinChannel(context.Background(), p)
	assert.Nil(t, err)
	assert.False(t, session.channelMembership.IsMember("SecretChannel"))
	gameSocket.AssertCalled(t, "Send", mock.Anything)
}

func TestHandleJoinChannelBanned(t *testing.T) {
	chatClient := &chatMocks.ChatServiceClient{}
	chatClient.On("JoinChannel", mock.Anything, mock.Anything).Return(&pbChat.JoinChannelResponse{
		Status: pbChat.JoinChannelResponse_Banned,
	}, nil)

	gameSocket := &mocks.Socket{}
	gameSocket.On("Send", mock.Anything).Return()

	broadcaster := eBroadcaster.NewChatChannelsService()
	session := &GameSession{
		gameSocket:        gameSocket,
		chatServiceClient: chatClient,
		channelMembership: NewChannelMembership(12345, broadcaster),
		character:         &LoggedInCharacter{GUID: 12345, Name: "TestPlayer", Race: 1},
	}

	p := packet.NewWriter(packet.CMsgJoinChannel).
		Uint32(0).
		Uint8(0).
		Uint8(0).
		String("BannedChannel").
		String("").
		ToPacket()

	err := session.HandleJoinChannel(context.Background(), p)
	assert.Nil(t, err)
	assert.False(t, session.channelMembership.IsMember("BannedChannel"))
}

func TestHandleJoinChannelSystemForwardToWorldserver(t *testing.T) {
	worldSocket := &mocks.Socket{}
	worldSocket.On("SendPacket", mock.Anything).Return()

	session := &GameSession{
		worldSocket: worldSocket,
	}

	// System channel (channelID != 0)
	p := packet.NewWriter(packet.CMsgJoinChannel).
		Uint32(1). // System channel ID
		Uint8(0).
		Uint8(0).
		String("General").
		String("").
		ToPacket()

	err := session.HandleJoinChannel(context.Background(), p)
	assert.Nil(t, err)
	worldSocket.AssertCalled(t, "SendPacket", mock.Anything)
}

func TestHandleLeaveChannelSuccess(t *testing.T) {
	chatClient := &chatMocks.ChatServiceClient{}
	chatClient.On("LeaveChannel", mock.Anything, mock.MatchedBy(func(req *pbChat.LeaveChannelRequest) bool {
		return req.ChannelName == "TestChannel"
	})).Return(&pbChat.LeaveChannelResponse{
		Status: pbChat.LeaveChannelResponse_Ok,
	}, nil)

	gameSocket := &mocks.Socket{}
	gameSocket.On("Send", mock.Anything).Return()

	broadcaster := eBroadcaster.NewChatChannelsService()
	cm := NewChannelMembership(12345, broadcaster)
	cm.AddChannel("TestChannel", 0, ChannelFlagCustom)

	session := &GameSession{
		gameSocket:        gameSocket,
		chatServiceClient: chatClient,
		channelMembership: cm,
		character:         &LoggedInCharacter{GUID: 12345, Name: "TestPlayer", Race: 1},
	}

	p := packet.NewWriter(packet.CMsgLeaveChannel).
		Uint32(0).
		String("TestChannel").
		ToPacket()

	err := session.HandleLeaveChannel(context.Background(), p)
	assert.Nil(t, err)
	assert.False(t, session.channelMembership.IsMember("TestChannel"))
}

func TestHandleLeaveChannelNotMember(t *testing.T) {
	chatClient := &chatMocks.ChatServiceClient{}
	chatClient.On("LeaveChannel", mock.Anything, mock.Anything).Return(&pbChat.LeaveChannelResponse{
		Status: pbChat.LeaveChannelResponse_NotMember,
	}, nil)

	gameSocket := &mocks.Socket{}
	gameSocket.On("Send", mock.Anything).Return()

	broadcaster := eBroadcaster.NewChatChannelsService()
	session := &GameSession{
		gameSocket:        gameSocket,
		chatServiceClient: chatClient,
		channelMembership: NewChannelMembership(12345, broadcaster),
		character:         &LoggedInCharacter{GUID: 12345, Name: "TestPlayer", Race: 1},
	}

	p := packet.NewWriter(packet.CMsgLeaveChannel).
		Uint32(0).
		String("NonExistentChannel").
		ToPacket()

	err := session.HandleLeaveChannel(context.Background(), p)
	assert.Nil(t, err)
	gameSocket.AssertCalled(t, "Send", mock.Anything)
}

func TestHandleChannelList(t *testing.T) {
	chatClient := &chatMocks.ChatServiceClient{}
	chatClient.On("GetChannelList", mock.Anything, mock.MatchedBy(func(req *pbChat.GetChannelListRequest) bool {
		return req.ChannelName == "TestChannel"
	})).Return(&pbChat.GetChannelListResponse{
		Status: pbChat.GetChannelListResponse_Ok,
		Members: []*pbChat.ChannelMember{
			{Guid: 111, Flags: uint32(MemberFlagOwner)},
			{Guid: 222, Flags: uint32(MemberFlagModerator)},
			{Guid: 333, Flags: uint32(MemberFlagNone)},
		},
	}, nil)

	gameSocket := &mocks.Socket{}
	gameSocket.On("Send", mock.MatchedBy(func(w *packet.Writer) bool {
		return w.Opcode == packet.SMsgChannelList
	})).Return()

	broadcaster := eBroadcaster.NewChatChannelsService()
	cm := NewChannelMembership(12345, broadcaster)
	cm.AddChannel("TestChannel", 0, ChannelFlagCustom)

	session := &GameSession{
		gameSocket:        gameSocket,
		chatServiceClient: chatClient,
		channelMembership: cm,
		character:         &LoggedInCharacter{GUID: 12345},
	}

	p := packet.NewWriter(packet.CMsgChannelList).
		String("TestChannel").
		ToPacket()

	err := session.HandleChannelList(context.Background(), p)
	assert.Nil(t, err)
	gameSocket.AssertCalled(t, "Send", mock.Anything)
}

func TestSendChannelMessageToChatSuccess(t *testing.T) {
	chatClient := &chatMocks.ChatServiceClient{}
	chatClient.On("SendChannelMessage", mock.Anything, mock.MatchedBy(func(req *pbChat.SendChannelMessageRequest) bool {
		return req.ChannelName == "TestChannel" && req.Message == "Hello world"
	})).Return(&pbChat.SendChannelMessageResponse{
		Status: pbChat.SendChannelMessageResponse_Ok,
	}, nil)

	gameSocket := &mocks.Socket{}
	gameSocket.On("Send", mock.Anything).Return()

	session := &GameSession{
		gameSocket:        gameSocket,
		chatServiceClient: chatClient,
		character:         &LoggedInCharacter{GUID: 12345, Name: "TestPlayer", Race: 1},
	}

	err := session.SendChannelMessageToChat(context.Background(), "TestChannel", "Hello world", 0)
	assert.Nil(t, err)
	gameSocket.AssertCalled(t, "Send", mock.Anything) // Echo back
}

func TestSendChannelMessageToChatMuted(t *testing.T) {
	chatClient := &chatMocks.ChatServiceClient{}
	chatClient.On("SendChannelMessage", mock.Anything, mock.Anything).Return(&pbChat.SendChannelMessageResponse{
		Status: pbChat.SendChannelMessageResponse_Muted,
	}, nil)

	gameSocket := &mocks.Socket{}
	gameSocket.On("Send", mock.Anything).Return()

	session := &GameSession{
		gameSocket:        gameSocket,
		chatServiceClient: chatClient,
		character:         &LoggedInCharacter{GUID: 12345, Name: "TestPlayer", Race: 1},
	}

	err := session.SendChannelMessageToChat(context.Background(), "TestChannel", "Hello", 0)
	assert.Nil(t, err)
	gameSocket.AssertCalled(t, "Send", mock.Anything) // Muted notification
}

func TestHandleEventChannelMessageDelivery(t *testing.T) {
	gameSocket := &mocks.Socket{}
	gameSocket.On("Send", mock.MatchedBy(func(w *packet.Writer) bool {
		return w.Opcode == packet.SMsgMessageChat
	})).Return()

	broadcaster := eBroadcaster.NewChatChannelsService()
	cm := NewChannelMembership(12345, broadcaster)
	cm.AddChannel("TestChannel", 0, ChannelFlagCustom)

	session := &GameSession{
		gameSocket:        gameSocket,
		channelMembership: cm,
		character:         &LoggedInCharacter{GUID: 12345},
	}

	event := &eBroadcaster.Event{
		Type: eBroadcaster.EventTypeChannelMessage,
		Payload: &eBroadcaster.ChannelMessagePayload{
			ChannelName: "TestChannel",
			SenderGUID:  99999,
			SenderName:  "OtherPlayer",
			Language:    0,
			Message:     "Hello from other player",
		},
	}

	err := session.HandleEventChannelMessage(context.Background(), event)
	assert.Nil(t, err)
	gameSocket.AssertCalled(t, "Send", mock.Anything)
}

func TestHandleEventChannelMessageNotMember(t *testing.T) {
	gameSocket := &mocks.Socket{}

	broadcaster := eBroadcaster.NewChatChannelsService()
	cm := NewChannelMembership(12345, broadcaster)
	// Not a member of any channels

	session := &GameSession{
		gameSocket:        gameSocket,
		channelMembership: cm,
		character:         &LoggedInCharacter{GUID: 12345},
	}

	event := &eBroadcaster.Event{
		Type: eBroadcaster.EventTypeChannelMessage,
		Payload: &eBroadcaster.ChannelMessagePayload{
			ChannelName: "TestChannel",
			SenderGUID:  99999,
			SenderName:  "OtherPlayer",
			Message:     "Should not receive",
		},
	}

	err := session.HandleEventChannelMessage(context.Background(), event)
	assert.Nil(t, err)
	gameSocket.AssertNotCalled(t, "Send", mock.Anything)
}

func TestHandleEventChannelMessageSelfEcho(t *testing.T) {
	gameSocket := &mocks.Socket{}

	broadcaster := eBroadcaster.NewChatChannelsService()
	cm := NewChannelMembership(12345, broadcaster)
	cm.AddChannel("TestChannel", 0, ChannelFlagCustom)

	session := &GameSession{
		gameSocket:        gameSocket,
		channelMembership: cm,
		character:         &LoggedInCharacter{GUID: 12345},
	}

	// Event from self - should not echo
	event := &eBroadcaster.Event{
		Type: eBroadcaster.EventTypeChannelMessage,
		Payload: &eBroadcaster.ChannelMessagePayload{
			ChannelName: "TestChannel",
			SenderGUID:  12345, // Same as session character
			SenderName:  "TestPlayer",
			Message:     "My own message",
		},
	}

	err := session.HandleEventChannelMessage(context.Background(), event)
	assert.Nil(t, err)
	gameSocket.AssertNotCalled(t, "Send", mock.Anything)
}

func TestHandleEventChannelJoined(t *testing.T) {
	gameSocket := &mocks.Socket{}
	gameSocket.On("Send", mock.MatchedBy(func(w *packet.Writer) bool {
		return w.Opcode == packet.SMsgChannelNotify
	})).Return()

	broadcaster := eBroadcaster.NewChatChannelsService()
	cm := NewChannelMembership(12345, broadcaster)
	cm.AddChannel("TestChannel", 0, ChannelFlagCustom)

	session := &GameSession{
		gameSocket:        gameSocket,
		channelMembership: cm,
		character:         &LoggedInCharacter{GUID: 12345},
	}

	event := &eBroadcaster.Event{
		Type: eBroadcaster.EventTypeChannelJoined,
		Payload: &eBroadcaster.ChannelJoinedPayload{
			ChannelName: "TestChannel",
			PlayerGUID:  99999,
			PlayerName:  "OtherPlayer",
		},
	}

	err := session.HandleEventChannelJoined(context.Background(), event)
	assert.Nil(t, err)
	gameSocket.AssertCalled(t, "Send", mock.Anything)
}

func TestHandleEventChannelJoinedSelf(t *testing.T) {
	gameSocket := &mocks.Socket{}

	broadcaster := eBroadcaster.NewChatChannelsService()
	cm := NewChannelMembership(12345, broadcaster)
	cm.AddChannel("TestChannel", 0, ChannelFlagCustom)

	session := &GameSession{
		gameSocket:        gameSocket,
		channelMembership: cm,
		character:         &LoggedInCharacter{GUID: 12345},
	}

	// Event about self joining - should not send
	event := &eBroadcaster.Event{
		Type: eBroadcaster.EventTypeChannelJoined,
		Payload: &eBroadcaster.ChannelJoinedPayload{
			ChannelName: "TestChannel",
			PlayerGUID:  12345,
			PlayerName:  "TestPlayer",
		},
	}

	err := session.HandleEventChannelJoined(context.Background(), event)
	assert.Nil(t, err)
	gameSocket.AssertNotCalled(t, "Send", mock.Anything)
}

func TestHandleEventChannelLeft(t *testing.T) {
	gameSocket := &mocks.Socket{}
	gameSocket.On("Send", mock.MatchedBy(func(w *packet.Writer) bool {
		return w.Opcode == packet.SMsgChannelNotify
	})).Return()

	broadcaster := eBroadcaster.NewChatChannelsService()
	cm := NewChannelMembership(12345, broadcaster)
	cm.AddChannel("TestChannel", 0, ChannelFlagCustom)

	session := &GameSession{
		gameSocket:        gameSocket,
		channelMembership: cm,
		character:         &LoggedInCharacter{GUID: 12345},
	}

	event := &eBroadcaster.Event{
		Type: eBroadcaster.EventTypeChannelLeft,
		Payload: &eBroadcaster.ChannelLeftPayload{
			ChannelName: "TestChannel",
			PlayerGUID:  99999,
			PlayerName:  "OtherPlayer",
		},
	}

	err := session.HandleEventChannelLeft(context.Background(), event)
	assert.Nil(t, err)
	gameSocket.AssertCalled(t, "Send", mock.Anything)
}

func TestHandleEventChannelLeftSelf(t *testing.T) {
	gameSocket := &mocks.Socket{}

	broadcaster := eBroadcaster.NewChatChannelsService()
	cm := NewChannelMembership(12345, broadcaster)
	cm.AddChannel("TestChannel", 0, ChannelFlagCustom)

	session := &GameSession{
		gameSocket:        gameSocket,
		channelMembership: cm,
		character:         &LoggedInCharacter{GUID: 12345},
	}

	// Event about self leaving - should not send
	event := &eBroadcaster.Event{
		Type: eBroadcaster.EventTypeChannelLeft,
		Payload: &eBroadcaster.ChannelLeftPayload{
			ChannelName: "TestChannel",
			PlayerGUID:  12345,
			PlayerName:  "TestPlayer",
		},
	}

	err := session.HandleEventChannelLeft(context.Background(), event)
	assert.Nil(t, err)
	gameSocket.AssertNotCalled(t, "Send", mock.Anything)
}

func TestGetTeamID(t *testing.T) {
	tests := []struct {
		race     uint8
		expected pbChat.TeamID
	}{
		{1, pbChat.TeamID_TEAM_ALLIANCE},  // Human - Alliance
		{3, pbChat.TeamID_TEAM_ALLIANCE},  // Dwarf - Alliance
		{4, pbChat.TeamID_TEAM_ALLIANCE},  // Night Elf - Alliance
		{7, pbChat.TeamID_TEAM_ALLIANCE},  // Gnome - Alliance
		{11, pbChat.TeamID_TEAM_ALLIANCE}, // Draenei - Alliance
		{2, pbChat.TeamID_TEAM_HORDE},     // Orc - Horde
		{5, pbChat.TeamID_TEAM_HORDE},     // Undead - Horde
		{6, pbChat.TeamID_TEAM_HORDE},     // Tauren - Horde
		{8, pbChat.TeamID_TEAM_HORDE},     // Troll - Horde
		{10, pbChat.TeamID_TEAM_HORDE},    // Blood Elf - Horde
		{9, pbChat.TeamID_TEAM_ALLIANCE},  // Goblin - Alliance (per wow.DefaultRaces)
		{99, pbChat.TeamID_TEAM_NEUTRAL},  // Unknown - Neutral
	}

	for _, tt := range tests {
		session := &GameSession{
			character: &LoggedInCharacter{Race: tt.race},
		}
		teamID := session.getTeamID()
		assert.Equal(t, tt.expected, teamID, "Race %d should return teamID %d", tt.race, tt.expected)
	}
}
