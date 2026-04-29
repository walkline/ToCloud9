package session

import (
	"context"
	"fmt"
	"strings"
	"time"

	root "github.com/walkline/ToCloud9/apps/gateway"
	eBroadcaster "github.com/walkline/ToCloud9/apps/gateway/events-broadcaster"
	"github.com/walkline/ToCloud9/apps/gateway/packet"
	pbChat "github.com/walkline/ToCloud9/gen/chat/pb"
	"github.com/walkline/ToCloud9/shared/wow"
)

// Channel IDs from ChatChannels.dbc
const (
	ChannelIDCustom          uint32 = 0
	ChannelIDGeneral         uint32 = 1
	ChannelIDTrade           uint32 = 2
	ChannelIDLocalDefense    uint32 = 22
	ChannelIDWorldDefense    uint32 = 23
	ChannelIDGuildRecrument  uint32 = 25
	ChannelIDLookingForGroup uint32 = 26
)

// Channel flags from worldserver
const (
	ChannelFlagNone    uint8 = 0x00
	ChannelFlagCustom  uint8 = 0x01
	ChannelFlagTrade   uint8 = 0x04
	ChannelFlagNotLFG  uint8 = 0x08
	ChannelFlagGeneral uint8 = 0x10
	ChannelFlagCity    uint8 = 0x20
	ChannelFlagLFG     uint8 = 0x40
	ChannelFlagVoice   uint8 = 0x80
)

// Member flags
const (
	MemberFlagNone      uint8 = 0x00
	MemberFlagOwner     uint8 = 0x01
	MemberFlagModerator uint8 = 0x02
)

// WorldserverChannelInfo holds channel data from worldserver
type WorldserverChannelInfo struct {
	Name      string
	ChannelID uint32
	Flags     uint8
}

// ChannelMembership tracks which channels a player is subscribed to
type ChannelMembership struct {
	channels map[string]*ChannelInfo // key: channelName (lowercase for case-insensitive lookup)
	events   <-chan eBroadcaster.Event

	// initialChannels used to send the same packets that client is sending when player logs in into the game
	// to send them to worldserver when "redirecting".
	// We are doing so because at this point only worldserver knows location names (and some system channels have location names)
	// and I don't want to add dbc reading especially into gateway at this point.
	initialChannels                   map[string]*ChannelInfo
	lastJoinDateToTrackInitialJoining time.Time
	initialChannelsLoaded             bool

	playerGUID        uint64
	eventsBroadcaster *eBroadcaster.ChatChannelsService
}

// ChannelInfo holds information about a channel the player is a member of
type ChannelInfo struct {
	Name      string
	ChannelID uint32
	Flags     uint8
}

func NewChannelMembership(playerGUID uint64, eventsBroadcaster *eBroadcaster.ChatChannelsService) *ChannelMembership {
	return &ChannelMembership{
		channels:                          make(map[string]*ChannelInfo),
		playerGUID:                        playerGUID,
		eventsBroadcaster:                 eventsBroadcaster,
		initialChannels:                   make(map[string]*ChannelInfo),
		lastJoinDateToTrackInitialJoining: time.Now(),
	}
}

func (cm *ChannelMembership) AddChannel(name string, channelID uint32, flags uint8) {
	// Normalize channel name to lowercase for case-insensitive lookups
	// This matches the behavior in chatserver's channelKey() function
	normalizedName := strings.ToLower(name)

	cm.channels[normalizedName] = &ChannelInfo{
		Name:      name, // Store original case for display
		ChannelID: channelID,
		Flags:     flags,
	}

	cm.events = cm.eventsBroadcaster.AddPlayerToChannel(cm.playerGUID, normalizedName)

	if !cm.initialChannelsLoaded {
		const initialJoiningWaitingTime = 5 * time.Second
		if time.Since(cm.lastJoinDateToTrackInitialJoining) > initialJoiningWaitingTime {
			cm.initialChannelsLoaded = true
		} else {
			cm.initialChannels[normalizedName] = &ChannelInfo{
				Name:      name,
				ChannelID: channelID,
				Flags:     flags,
			}
			cm.lastJoinDateToTrackInitialJoining = time.Now()
		}
	}
}

func (cm *ChannelMembership) RemoveChannel(name string) {
	normalizedName := strings.ToLower(name)
	delete(cm.channels, normalizedName)
	cm.eventsBroadcaster.RemovePlayerFromChannel(cm.playerGUID, normalizedName)
}

func (cm *ChannelMembership) GetChannel(name string) *ChannelInfo {
	return cm.channels[strings.ToLower(name)]
}

func (cm *ChannelMembership) GetEventsStream() <-chan eBroadcaster.Event {
	return cm.events
}

func (cm *ChannelMembership) IsMember(name string) bool {
	_, exists := cm.channels[strings.ToLower(name)]
	return exists
}

func (cm *ChannelMembership) GetAllChannels() []*ChannelInfo {
	channels := make([]*ChannelInfo, 0, len(cm.channels))
	for _, ch := range cm.channels {
		channels = append(channels, ch)
	}
	return channels
}

// Chat notify types (from Channel.h)
const (
	ChatJoinedNotice              uint8 = 0x00
	ChatLeftNotice                uint8 = 0x01
	ChatYouJoinedNotice           uint8 = 0x02
	ChatYouLeftNotice             uint8 = 0x03
	ChatWrongPasswordNotice       uint8 = 0x04
	ChatNotMemberNotice           uint8 = 0x05
	ChatNotModeratorNotice        uint8 = 0x06
	ChatPasswordChangedNotice     uint8 = 0x07
	ChatOwnerChangedNotice        uint8 = 0x08
	ChatPlayerNotFoundNotice      uint8 = 0x09
	ChatNotOwnerNotice            uint8 = 0x0A
	ChatChannelOwnerNotice        uint8 = 0x0B
	ChatModeChangeNotice          uint8 = 0x0C
	ChatAnnouncementsOnNotice     uint8 = 0x0D
	ChatAnnouncementsOffNotice    uint8 = 0x0E
	ChatModerationOnNotice        uint8 = 0x0F
	ChatModerationOffNotice       uint8 = 0x10
	ChatMutedNotice               uint8 = 0x11
	ChatPlayerKickedNotice        uint8 = 0x12
	ChatBannedNotice              uint8 = 0x13
	ChatPlayerBannedNotice        uint8 = 0x14
	ChatPlayerUnbannedNotice      uint8 = 0x15
	ChatPlayerNotBannedNotice     uint8 = 0x16
	ChatPlayerAlreadyMemberNotice uint8 = 0x17
	ChatInviteNotice              uint8 = 0x18
	ChatInviteWrongFactionNotice  uint8 = 0x19
	ChatWrongFactionNotice        uint8 = 0x1A
	ChatInvalidNameNotice         uint8 = 0x1B
	ChatNotModeratedNotice        uint8 = 0x1C
	ChatPlayerInvitedNotice       uint8 = 0x1D
	ChatPlayerInviteBannedNotice  uint8 = 0x1E
	ChatThrottledNotice           uint8 = 0x1F
	ChatNotInAreaNotice           uint8 = 0x20
	ChatNotInLFGNotice            uint8 = 0x21
	ChatVoiceOnNotice             uint8 = 0x22
	ChatVoiceOffNotice            uint8 = 0x23
)

// HandleJoinChannel handles CMSG_JOIN_CHANNEL packet
func (s *GameSession) HandleJoinChannel(ctx context.Context, p *packet.Packet) error {

	r := p.Reader()
	channelID := r.Uint32()
	_ = r.Uint8() // unknown1
	_ = r.Uint8() // unknown2
	channelName := r.String()
	password := r.String()
	fmt.Println("Join Channel", channelName, channelID)
	// If it's a system channel, just forward it to worldserver since it has all required DBC data
	// and we will hook to worldserver response.
	if channelID != 0 && s.worldSocket != nil {
		s.worldSocket.SendPacket(p)
		return nil
	}

	// Channel flags from ChatChannels.dbc
	flagsMap := map[uint32]uint32{
		ChannelIDCustom:          0x1,
		ChannelIDGeneral:         524291,
		ChannelIDTrade:           59,
		ChannelIDLocalDefense:    65539,
		ChannelIDWorldDefense:    65540,
		ChannelIDGuildRecrument:  131122,
		ChannelIDLookingForGroup: 262201,
	}

	// Call chat service to join the channel
	resp, err := s.chatServiceClient.JoinChannel(ctx, &pbChat.JoinChannelRequest{
		Api:          root.Ver,
		RealmID:      root.RealmID,
		PlayerGUID:   s.character.GUID,
		PlayerName:   s.character.Name,
		TeamID:       s.getTeamID(),
		ChannelName:  channelName,
		ChannelID:    channelID,
		Password:     password,
		ChannelFlags: flagsMap[channelID],
	})
	if err != nil {
		s.logger.Error().Err(err).Str("channelName", channelName).Msg("Failed to join channel")
		return err
	}

	// Handle response
	switch resp.Status {
	case pbChat.JoinChannelResponse_Ok:
		// Add to local membership - use channel Flags, not memberFlags!
		s.channelMembership.AddChannel(channelName, resp.Channel.ChannelID, uint8(resp.Channel.Flags))
		ch := s.channelMembership.GetChannel(channelName)
		notify := s.ChannelNotify(ch)

		notify.Joined(s.character.GUID)
		notify.YouJoined()

		// For custom channels, if the player became owner, send mode change notification
		memberFlags := uint8(resp.Channel.MemberFlags)
		if ch.Flags&ChannelFlagCustom != 0 && memberFlags&MemberFlagOwner != 0 {
			notify.ModeChange(s.character.GUID, MemberFlagModerator, MemberFlagModerator|MemberFlagOwner)
		}

	case pbChat.JoinChannelResponse_WrongPassword:
		s.ChannelNotify(&ChannelInfo{Name: channelName}).Simple(ChatWrongPasswordNotice)

	case pbChat.JoinChannelResponse_Banned:
		s.ChannelNotify(&ChannelInfo{Name: channelName}).Simple(ChatBannedNotice)

	case pbChat.JoinChannelResponse_WrongFaction:
		s.ChannelNotify(&ChannelInfo{Name: channelName}).Simple(ChatWrongFactionNotice)

	case pbChat.JoinChannelResponse_NotInArea:
		s.ChannelNotify(&ChannelInfo{Name: channelName}).Simple(ChatNotInAreaNotice)

	case pbChat.JoinChannelResponse_Throttled:
		s.ChannelNotify(&ChannelInfo{Name: channelName}).Simple(ChatThrottledNotice)
	}

	return nil
}

// HandleLeaveChannel handles CMSG_LEAVE_CHANNEL packet
func (s *GameSession) HandleLeaveChannel(ctx context.Context, p *packet.Packet) error {
	r := p.Reader()
	_ = r.Uint32() // unknown
	channelName := r.String()

	// Call chat service to leave the channel
	resp, err := s.chatServiceClient.LeaveChannel(ctx, &pbChat.LeaveChannelRequest{
		Api:         root.Ver,
		RealmID:     root.RealmID,
		PlayerGUID:  s.character.GUID,
		ChannelName: channelName,
		TeamID:      s.getTeamID(),
	})
	if err != nil {
		return err
	}

	// Handle response
	switch resp.Status {
	case pbChat.LeaveChannelResponse_Ok:
		notify := s.ChannelNotify(&ChannelInfo{Name: channelName})
		// Remove from local membership
		s.channelMembership.RemoveChannel(channelName)

		notify.YouLeft()

	case pbChat.LeaveChannelResponse_NotMember:
		s.ChannelNotify(&ChannelInfo{Name: channelName}).Simple(ChatNotMemberNotice)
	}

	return nil
}

// HandleChannelList handles CMSG_CHANNEL_LIST packet
func (s *GameSession) HandleChannelList(ctx context.Context, p *packet.Packet) error {
	r := p.Reader()
	channelName := r.String()

	// Call chat service to get channel members
	resp, err := s.chatServiceClient.GetChannelList(ctx, &pbChat.GetChannelListRequest{
		Api:         root.Ver,
		RealmID:     root.RealmID,
		PlayerGUID:  s.character.GUID,
		ChannelName: channelName,
		TeamID:      s.getTeamID(),
	})
	if err != nil {
		s.logger.Error().Err(err).Str("channelName", channelName).Msg("Failed to get channel list from chat service")
		return err
	}

	if resp.Status != pbChat.GetChannelListResponse_Ok {
		return nil
	}

	// Send SMSG_CHANNEL_LIST
	channelInfo := s.channelMembership.GetChannel(channelName)
	if channelInfo == nil {
		return nil
	}

	w := packet.NewWriterWithSize(packet.SMsgChannelList, 0)
	w.Uint8(1) // Display type: 1 = list
	w.String(channelName)
	w.Uint8(channelInfo.Flags)
	w.Uint32(uint32(len(resp.Members)))

	for _, member := range resp.Members {
		w.Uint64(member.Guid)
		w.Uint8(uint8(member.Flags))
		// Note: Some implementations include player name here, but AC 3.3.5a doesn't
		// The client looks up names by GUID
	}

	s.gameSocket.Send(w)
	return nil
}

// channelNotify is a builder for SMSG_CHANNEL_NOTIFY packets.
type channelNotify struct {
	s  *GameSession
	ch *ChannelInfo
}

// ChannelNotify creates a notification builder for the given channel.
func (s *GameSession) ChannelNotify(ch *ChannelInfo) channelNotify {
	return channelNotify{s: s, ch: ch}
}

func (n channelNotify) header(notifyType uint8) *packet.Writer {
	w := packet.NewWriterWithSize(packet.SMsgChannelNotify, 0)
	w.Uint8(notifyType)
	w.String(n.ch.Name)
	return w
}

func (n channelNotify) send(w *packet.Writer) {
	n.s.gameSocket.Send(w)
}

func (n channelNotify) Joined(playerGUID uint64) {
	w := n.header(ChatJoinedNotice)
	w.Uint64(playerGUID)
	n.send(w)
}

func (n channelNotify) Left(playerGUID uint64) {
	w := n.header(ChatLeftNotice)
	w.Uint64(playerGUID)
	n.send(w)
}

func (n channelNotify) YouJoined() {
	w := n.header(ChatYouJoinedNotice)
	w.Uint8(n.ch.Flags)
	if n.ch.Flags&ChannelFlagCustom != 0 {
		w.Uint32(0)
	} else {
		w.Uint32(n.ch.ChannelID)
	}
	w.Uint32(0)
	n.send(w)
}

func (n channelNotify) YouLeft() {
	n.send(n.header(ChatYouLeftNotice))
}

func (n channelNotify) ModeChange(playerGUID uint64, oldFlags, newFlags uint8) {
	w := n.header(ChatModeChangeNotice)
	w.Uint64(playerGUID)
	w.Uint8(oldFlags)
	w.Uint8(newFlags)
	n.send(w)
}

func (n channelNotify) PlayerAction(notifyType uint8, playerGUID uint64) {
	w := n.header(notifyType)
	w.Uint64(playerGUID)
	n.send(w)
}

func (n channelNotify) PlayerName(notifyType uint8, name string) {
	w := n.header(notifyType)
	w.String(name)
	n.send(w)
}

func (n channelNotify) Simple(notifyType uint8) {
	n.send(n.header(notifyType))
}

// SendChannelMessage sends a channel message to the client
func (s *GameSession) SendChannelMessage(channelName string, senderGUID uint64, senderName string, language uint32, message string) {
	w := packet.NewWriterWithSize(packet.SMsgMessageChat, 0)
	w.Uint8(uint8(ChatTypeChannel)) // CHAT_MSG_CHANNEL = 0x11 = 17
	w.Uint32(language)              // int32 language
	w.Uint64(senderGUID)            // ObjectGuid senderGUID
	w.Uint32(0)                     // uint32 flags
	w.String(channelName)           // string channelName (comes AFTER flags!)
	w.Uint64(s.character.GUID)
	msgLen := uint32(len(message) + 1)
	w.Uint32(msgLen)  // uint32 message length
	w.String(message) // string message
	w.Uint8(0)        // uint8 chatTag

	s.gameSocket.Send(w)
}

func (s *GameSession) getTeamID() pbChat.TeamID {
	raceID := wow.RaceID(s.character.Race)
	if int(raceID) < len(wow.DefaultRaces) {
		return wow.DefaultRaces[raceID].Team.TeamID()
	}
	return pbChat.TeamID_TEAM_NEUTRAL
}

// SendChannelMessageToChat sends a channel message via the chat service
func (s *GameSession) SendChannelMessageToChat(ctx context.Context, channelName string, message string, language uint32) error {
	resp, err := s.chatServiceClient.SendChannelMessage(ctx, &pbChat.SendChannelMessageRequest{
		Api:         root.Ver,
		RealmID:     root.RealmID,
		SenderGUID:  s.character.GUID,
		SenderName:  s.character.Name,
		ChannelName: channelName,
		Language:    language,
		Message:     message,
		TeamID:      s.getTeamID(),
	})
	if err != nil {
		s.logger.Error().Err(err).Str("channelName", channelName).Msg("Failed to send channel message")
		return err
	}

	switch resp.Status {
	case pbChat.SendChannelMessageResponse_Ok:
		// Echo the message back to the sender (like guild/party messages)
		s.SendChannelMessage(channelName, s.character.GUID, s.character.Name, language, message)
	case pbChat.SendChannelMessageResponse_NotMember:
		s.ChannelNotify(&ChannelInfo{Name: channelName}).Simple(ChatNotMemberNotice)
	case pbChat.SendChannelMessageResponse_Muted:
		s.ChannelNotify(&ChannelInfo{Name: channelName}).Simple(ChatMutedNotice)
	case pbChat.SendChannelMessageResponse_Throttled:
		s.ChannelNotify(&ChannelInfo{Name: channelName}).Simple(ChatThrottledNotice)
	}

	return nil
}

func (s *GameSession) HandleEventChannelMessage(ctx context.Context, e *eBroadcaster.Event) error {
	eventData := e.Payload.(*eBroadcaster.ChannelMessagePayload)

	fmt.Println("eventData:", eventData.Message, s.character.Name, "from", eventData.SenderName)

	// Only send if we're a member of this channel
	if !s.channelMembership.IsMember(eventData.ChannelName) {
		return nil
	}

	// Don't send to the sender (they already got the echo)
	if s.character != nil && s.character.GUID == eventData.SenderGUID {
		return nil
	}

	s.SendChannelMessage(eventData.ChannelName, eventData.SenderGUID, eventData.SenderName, eventData.Language, eventData.Message)
	return nil
}

func (s *GameSession) HandleEventChannelJoined(ctx context.Context, e *eBroadcaster.Event) error {
	eventData := e.Payload.(*eBroadcaster.ChannelJoinedPayload)

	// Only send if we're a member of this channel
	if !s.channelMembership.IsMember(eventData.ChannelName) {
		return nil
	}

	// Don't send to the player who just joined (they got YOU_JOINED notification)
	if s.character != nil && s.character.GUID == eventData.PlayerGUID {
		return nil
	}

	if ch := s.channelMembership.GetChannel(eventData.ChannelName); ch != nil {
		// Only notify in custom channels
		if ch.Flags&ChannelFlagCustom == 0 {
			return nil
		}
	}

	s.ChannelNotify(&ChannelInfo{Name: eventData.ChannelName}).Joined(eventData.PlayerGUID)
	return nil
}

func (s *GameSession) HandleEventChannelLeft(ctx context.Context, e *eBroadcaster.Event) error {
	eventData := e.Payload.(*eBroadcaster.ChannelLeftPayload)

	// Only send if we're a member of this channel
	if !s.channelMembership.IsMember(eventData.ChannelName) {
		return nil
	}

	// Don't send to the player who just left (they got YOU_LEFT notification)
	if s.character != nil && s.character.GUID == eventData.PlayerGUID {
		return nil
	}

	s.ChannelNotify(&ChannelInfo{Name: eventData.ChannelName}).Left(eventData.PlayerGUID)
	return nil
}

func (s *GameSession) InterceptWorldserverChannelNotify(ctx context.Context, p *packet.Packet) error {
	r := p.Reader()
	notifyType := r.Uint8()
	channelName := r.String()

	// Read flags and channelID
	flags := r.Uint8()
	channelID := r.Uint32()
	_ = r.Uint32() // unknown field

	// Ignore custom channels - we handle those separately
	if flags&ChannelFlagCustom != 0 {
		return nil
	}

	switch notifyType {
	case ChatYouLeftNotice:
		_, err := s.chatServiceClient.LeaveChannel(ctx, &pbChat.LeaveChannelRequest{
			Api:         root.Ver,
			RealmID:     root.RealmID,
			PlayerGUID:  s.character.GUID,
			ChannelName: channelName,
			TeamID:      s.getTeamID(),
		})
		if err != nil {
			return fmt.Errorf("failed to leave channel: %w", err)
		}

		// Remove from local membership
		s.channelMembership.RemoveChannel(channelName)
	case ChatYouJoinedNotice:
		if channelID != 0 {
			for _, ch := range s.channelMembership.channels {
				if ch.ChannelID == channelID {
					_, err := s.chatServiceClient.LeaveChannel(ctx, &pbChat.LeaveChannelRequest{
						Api:         root.Ver,
						RealmID:     root.RealmID,
						PlayerGUID:  s.character.GUID,
						ChannelName: ch.Name,
						TeamID:      s.getTeamID(),
					})
					if err != nil {
						return fmt.Errorf("failed to leave channel: %w", err)
					}

					// Remove from local membership
					s.channelMembership.RemoveChannel(ch.Name)
					break
				}
			}
		}

		// Join via chat service, passing worldserver's channel ID AND flags
		_, err := s.chatServiceClient.JoinChannel(ctx, &pbChat.JoinChannelRequest{
			Api:          root.Ver,
			RealmID:      root.RealmID,
			PlayerGUID:   s.character.GUID,
			PlayerName:   s.character.Name,
			TeamID:       s.getTeamID(),
			ChannelName:  channelName,
			ChannelID:    channelID,
			Password:     "",
			ChannelFlags: uint32(flags),
		})
		if err != nil {
			return fmt.Errorf("failed to join channel: %w", err)
		}
		s.channelMembership.AddChannel(channelName, channelID, flags)
	default:
		s.logger.Debug().
			Str("channelName", channelName).
			Uint8("flags", flags).
			Uint32("channelID", channelID).
			Uint8("notifyType", notifyType).
			Msg("Unhandled worldserver channel notification received")
		return nil
	}

	s.gameSocket.SendPacket(p)
	return nil
}

func (s *GameSession) RejoinWorldserverToSystemChannels(ctx context.Context) error {
	for _, ch := range s.channelMembership.initialChannels {
		if ch.ChannelID != 0 {
			p := packet.NewWriterWithSize(packet.CMsgJoinChannel, 0)
			p.Uint32(ch.ChannelID)
			p.Uint8(0) // unknown1
			p.Uint8(0) // unknown2
			p.String(ch.Name)
			p.String("")

			s.worldSocket.SendPacket(p.ToPacket())
		}
	}

	return nil
}

func (s *GameSession) HandleEventChannelNotification(ctx context.Context, e *eBroadcaster.Event) error {
	eventData := e.Payload.(*eBroadcaster.ChannelNotificationPayload)

	// For invitations (CHAT_INVITE_NOTICE), we don't need to be a member
	const ChatInviteNotice = 0x18

	// Check if we're a member (not required for invitations)
	isMember := s.channelMembership.IsMember(eventData.ChannelName)
	if !isMember && eventData.NotifyType != ChatInviteNotice {
		return nil
	}

	// Build and send the notification packet
	w := packet.NewWriterWithSize(packet.SMsgChannelNotify, 0)
	w.Uint8(eventData.NotifyType)
	w.String(eventData.ChannelName)

	// Different notification types include different data
	switch eventData.NotifyType {
	case ChatOwnerChangedNotice: // 0x08 - includes new owner GUID
		w.Uint64(eventData.TargetGUID)
	case ChatModeChangeNotice: // 0x0C - includes target GUID and flags
		w.Uint64(eventData.TargetGUID)
		w.Uint8(eventData.OldFlags)
		w.Uint8(eventData.NewFlags)
	case ChatInviteNotice: // 0x18 - includes inviter GUID
		w.Uint64(eventData.TargetGUID)
	}

	s.gameSocket.Send(w)
	return nil
}
