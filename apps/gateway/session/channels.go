package session

import (
	"context"
	"strings"
	"time"

	root "github.com/walkline/ToCloud9/apps/gateway"
	eBroadcaster "github.com/walkline/ToCloud9/apps/gateway/events-broadcaster"
	"github.com/walkline/ToCloud9/apps/gateway/packet"
	pbChat "github.com/walkline/ToCloud9/gen/chat/pb"
	"github.com/walkline/ToCloud9/shared/wow"
	wowguid "github.com/walkline/ToCloud9/shared/wow/guid"
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

// Player flags and chat tags mirror AzerothCore Player.h values used by Player::GetChatTag.
const (
	playerFlagAFK          uint32 = 0x00000002
	playerFlagDND          uint32 = 0x00000004
	playerFlagDeveloper    uint32 = 0x00008000
	playerFlagCommentator2 uint32 = 0x00400000

	playerExtraFlagGMChat uint32 = 0x00000020

	chatTagAFK uint8 = 0x01
	chatTagDND uint8 = 0x02
	chatTagGM  uint8 = 0x04
	chatTagCOM uint8 = 0x08
	chatTagDEV uint8 = 0x10
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
	Name              string
	ChannelID         uint32
	Flags             uint8
	RealmID           uint32
	TeamID            uint32
	ServicePlayerGUID uint64
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
	cm.AddScopedChannel(name, channelID, flags, 0, 0)
}

func (cm *ChannelMembership) AddScopedChannel(name string, channelID uint32, flags uint8, realmID uint32, teamID uint32, servicePlayerGUID ...uint64) {
	cm.addChannel(name, channelID, flags, realmID, teamID, true, servicePlayerGUID...)
}

func (cm *ChannelMembership) AddLocalChannel(name string, channelID uint32, flags uint8, realmID uint32, teamID uint32, servicePlayerGUID ...uint64) {
	cm.addChannel(name, channelID, flags, realmID, teamID, false, servicePlayerGUID...)
}

func (cm *ChannelMembership) addChannel(name string, channelID uint32, flags uint8, realmID uint32, teamID uint32, registerEvents bool, servicePlayerGUID ...uint64) {
	// Normalize channel name to lowercase for case-insensitive lookups
	// This matches the behavior in chatserver's channelKey() function
	normalizedName := strings.ToLower(name)
	playerGUID := cm.playerGUID
	if len(servicePlayerGUID) > 0 && servicePlayerGUID[0] != 0 {
		playerGUID = servicePlayerGUID[0]
	}

	cm.channels[normalizedName] = &ChannelInfo{
		Name:              name, // Store original case for display
		ChannelID:         channelID,
		Flags:             flags,
		RealmID:           realmID,
		TeamID:            teamID,
		ServicePlayerGUID: playerGUID,
	}

	if registerEvents {
		cm.events = cm.eventsBroadcaster.AddPlayerToScopedChannel(cm.playerGUID, realmID, teamID, normalizedName)
	}

	if !cm.initialChannelsLoaded {
		const initialJoiningWaitingTime = 5 * time.Second
		if time.Since(cm.lastJoinDateToTrackInitialJoining) > initialJoiningWaitingTime {
			cm.initialChannelsLoaded = true
		} else {
			cm.initialChannels[normalizedName] = &ChannelInfo{
				Name:              name,
				ChannelID:         channelID,
				Flags:             flags,
				RealmID:           realmID,
				TeamID:            teamID,
				ServicePlayerGUID: playerGUID,
			}
			cm.lastJoinDateToTrackInitialJoining = time.Now()
		}
	}
}

func (cm *ChannelMembership) RemoveChannel(name string) {
	normalizedName := strings.ToLower(name)
	if ch := cm.channels[normalizedName]; ch != nil {
		cm.eventsBroadcaster.RemovePlayerFromScopedChannel(cm.playerGUID, ch.RealmID, ch.TeamID, normalizedName)
	}
	delete(cm.channels, normalizedName)
}

func (cm *ChannelMembership) RemoveLocalChannelsByID(channelID uint32, exceptName string) {
	if channelID == ChannelIDCustom {
		return
	}

	normalizedExceptName := strings.ToLower(exceptName)
	for normalizedName, ch := range cm.channels {
		if normalizedName == normalizedExceptName || ch.ChannelID != channelID || ch.Flags&ChannelFlagCustom != 0 {
			continue
		}

		delete(cm.channels, normalizedName)
	}
	for normalizedName, ch := range cm.initialChannels {
		if normalizedName == normalizedExceptName || ch.ChannelID != channelID || ch.Flags&ChannelFlagCustom != 0 {
			continue
		}

		delete(cm.initialChannels, normalizedName)
	}
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

func (cm *ChannelMembership) IsMemberForEvent(name string, realmID uint32, teamID uint32) bool {
	ch := cm.GetChannel(name)
	if ch == nil {
		return false
	}
	if ch.RealmID == 0 && ch.TeamID == 0 {
		return true
	}
	return ch.RealmID == realmID && ch.TeamID == teamID
}

func (cm *ChannelMembership) GetAllChannels() []*ChannelInfo {
	channels := make([]*ChannelInfo, 0, len(cm.channels))
	for _, ch := range cm.channels {
		channels = append(channels, ch)
	}
	return channels
}

func (s *GameSession) isGatewayManagedChannel(channelName string) bool {
	if s == nil || s.channelMembership == nil {
		return false
	}

	ch := s.channelMembership.GetChannel(channelName)
	return ch != nil && ch.Flags&ChannelFlagCustom != 0
}

func (s *GameSession) forwardNativeChannelPacket(channelName string, p *packet.Packet) bool {
	if s.isGatewayManagedChannel(channelName) || s.worldSocket == nil {
		return false
	}

	s.worldSocket.SendPacket(p)
	return true
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
	if r.Error() != nil || strings.TrimSpace(channelName) == "" {
		return nil
	}

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
	channelRealmID, channelTeamID, servicePlayerGUID := s.channelScopeForID(channelID)

	// Call chat service to join the channel
	resp, err := s.chatServiceClient.JoinChannel(ctx, &pbChat.JoinChannelRequest{
		Api:          root.Ver,
		RealmID:      channelRealmID,
		PlayerGUID:   servicePlayerGUID,
		PlayerName:   s.character.Name,
		TeamID:       channelTeamID,
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
		s.channelMembership.AddScopedChannel(channelName, resp.Channel.ChannelID, uint8(resp.Channel.Flags), channelRealmID, uint32(channelTeamID), servicePlayerGUID)
		ch := s.channelMembership.GetChannel(channelName)
		notify := s.ChannelNotify(ch)

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
	if s.forwardNativeChannelPacket(channelName, p) {
		return nil
	}
	channelRealmID, channelTeamID, servicePlayerGUID := s.channelScopeForName(channelName)

	ch := s.channelMembership.GetChannel(channelName)
	if ch == nil {
		ch = &ChannelInfo{Name: channelName}
	}

	// Call chat service to leave the channel
	resp, err := s.chatServiceClient.LeaveChannel(ctx, &pbChat.LeaveChannelRequest{
		Api:         root.Ver,
		RealmID:     channelRealmID,
		PlayerGUID:  servicePlayerGUID,
		ChannelName: channelName,
		TeamID:      channelTeamID,
	})
	if err != nil {
		return err
	}

	// Handle response
	switch resp.Status {
	case pbChat.LeaveChannelResponse_Ok:
		notify := s.ChannelNotify(ch)
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
	if s.forwardNativeChannelPacket(channelName, p) {
		return nil
	}
	channelRealmID, channelTeamID, servicePlayerGUID := s.channelScopeForName(channelName)

	// Call chat service to get channel members
	resp, err := s.chatServiceClient.GetChannelList(ctx, &pbChat.GetChannelListRequest{
		Api:         root.Ver,
		RealmID:     channelRealmID,
		PlayerGUID:  servicePlayerGUID,
		ChannelName: channelName,
		TeamID:      channelTeamID,
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
		w.Uint64(playerObjectGUIDForRealm(channelInfo.RealmID, member.Guid))
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
	w := n.header(ChatYouLeftNotice)
	w.Uint32(n.ch.ChannelID)
	if n.ch.ChannelID != ChannelIDCustom {
		w.Uint8(1)
	} else {
		w.Uint8(0)
	}
	n.send(w)
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

func channelNotifyString(primary, secondary, fallback string) string {
	if primary != "" {
		return primary
	}
	if secondary != "" {
		return secondary
	}
	return fallback
}

func appendAzerothCoreChannelNotifyPayload(w *packet.Writer, notifyType uint8, targetGUID, secondGUID uint64, targetName, extraData string, oldFlags, newFlags uint8) {
	switch notifyType {
	case ChatJoinedNotice,
		ChatLeftNotice,
		ChatPasswordChangedNotice,
		ChatOwnerChangedNotice,
		ChatAnnouncementsOnNotice,
		ChatAnnouncementsOffNotice,
		ChatModerationOnNotice,
		ChatModerationOffNotice,
		ChatPlayerAlreadyMemberNotice,
		ChatInviteNotice,
		ChatVoiceOnNotice,
		ChatVoiceOffNotice:
		w.Uint64(targetGUID)
	case ChatModeChangeNotice:
		w.Uint64(targetGUID)
		w.Uint8(oldFlags)
		w.Uint8(newFlags)
	case ChatPlayerKickedNotice,
		ChatPlayerBannedNotice,
		ChatPlayerUnbannedNotice:
		w.Uint64(targetGUID)
		w.Uint64(secondGUID)
	case ChatPlayerNotFoundNotice,
		ChatChannelOwnerNotice,
		ChatPlayerNotBannedNotice,
		ChatPlayerInvitedNotice,
		ChatPlayerInviteBannedNotice:
		w.String(channelNotifyString(targetName, extraData, ""))
	}
}

// SendChannelMessage sends a channel message to the client
func (s *GameSession) SendChannelMessage(channelName string, senderRealmID uint32, senderGUID uint64, senderName string, language uint32, message string, chatTag uint8) {
	s.sendAzerothCorePlayerChat(ChatTypeChannel, language, senderRealmID, senderGUID, senderName, s.character.GUID, channelName, message, chatTag)
}

func (s *GameSession) currentChatTag() uint8 {
	if s == nil || s.character == nil {
		return 0
	}

	var tag uint8
	if s.character.ExtraFlags&playerExtraFlagGMChat != 0 {
		tag |= chatTagGM
	}
	if s.character.PlayerFlags&playerFlagDND != 0 {
		tag |= chatTagDND
	}
	if s.character.PlayerFlags&playerFlagAFK != 0 {
		tag |= chatTagAFK
	}
	if s.character.PlayerFlags&playerFlagCommentator2 != 0 {
		tag |= chatTagCOM
	}
	if s.character.PlayerFlags&playerFlagDeveloper != 0 {
		tag |= chatTagDEV
	}
	return tag
}

func (s *GameSession) getTeamID() pbChat.TeamID {
	raceID := wow.RaceID(s.character.Race)
	if int(raceID) < len(wow.DefaultRaces) {
		return wow.DefaultRaces[raceID].Team.TeamID()
	}
	return pbChat.TeamID_TEAM_NEUTRAL
}

func (s *GameSession) channelTeamID() pbChat.TeamID {
	if root.AllowTwoSideInteractionChannel {
		return pbChat.TeamID_TEAM_ALLIANCE
	}
	return s.getTeamID()
}

func (s *GameSession) channelScopeForID(channelID uint32) (uint32, pbChat.TeamID, uint64) {
	realmID := root.RealmID
	if channelID != ChannelIDCustom {
		realmID = s.dbcChannelRealmID()
	}
	return realmID, s.channelTeamID(), s.channelServicePlayerGUID(realmID)
}

func (s *GameSession) channelScopeForName(channelName string) (uint32, pbChat.TeamID, uint64) {
	if s != nil && s.channelMembership != nil {
		if ch := s.channelMembership.GetChannel(channelName); ch != nil {
			return ch.RealmID, pbChat.TeamID(ch.TeamID), ch.ServicePlayerGUID
		}
	}
	return s.channelScopeForID(ChannelIDCustom)
}

func (s *GameSession) dbcChannelRealmID() uint32 {
	for _, routing := range []*mapTransferRouting{s.currentMapTransferRouting, s.activeMapTransferRouting, s.pendingMapTransferRouting} {
		if routing != nil && routing.isCrossRealm {
			return routing.realmID
		}
	}
	return root.RealmID
}

func (s *GameSession) channelServicePlayerGUID(channelRealmID uint32) uint64 {
	if s == nil || s.character == nil {
		return 0
	}
	if channelRealmID != 0 && channelRealmID == root.RealmID {
		return s.character.GUID
	}
	return wowguid.PlayerGUIDForRealm(channelRealmID, root.RealmID, s.character.GUID)
}

func channelMessageLanguage(language uint32) uint32 {
	if root.AllowTwoSideInteractionChannel {
		return 0 // LANG_UNIVERSAL, matching AzerothCore Channel::Say.
	}
	return language
}

// SendChannelMessageToChat sends a channel message via the chat service
func (s *GameSession) SendChannelMessageToChat(ctx context.Context, channelName string, message string, language uint32) error {
	language = channelMessageLanguage(language)
	channelRealmID, channelTeamID, servicePlayerGUID := s.channelScopeForName(channelName)
	chatTag := s.currentChatTag()

	resp, err := s.chatServiceClient.SendChannelMessage(ctx, &pbChat.SendChannelMessageRequest{
		Api:           root.Ver,
		RealmID:       channelRealmID,
		SenderGUID:    servicePlayerGUID,
		SenderName:    s.character.Name,
		ChannelName:   channelName,
		Language:      language,
		Message:       message,
		TeamID:        channelTeamID,
		SenderChatTag: uint32(chatTag),
	})
	if err != nil {
		s.logger.Error().Err(err).Str("channelName", channelName).Msg("Failed to send channel message")
		return err
	}

	switch resp.Status {
	case pbChat.SendChannelMessageResponse_Ok:
		// Echo the message back to the sender (like guild/party messages)
		s.SendChannelMessage(channelName, channelRealmID, servicePlayerGUID, s.character.Name, language, message, chatTag)
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

	// Only send if we're a member of this channel
	if !s.channelMembership.IsMemberForEvent(eventData.ChannelName, eventData.RealmID, eventData.TeamID) {
		return nil
	}

	// Don't send to the sender (they already got the echo)
	if s.character != nil && wowguid.SamePlayer(root.RealmID, s.character.GUID, eventData.RealmID, eventData.SenderGUID) {
		return nil
	}

	s.SendChannelMessage(eventData.ChannelName, eventData.RealmID, eventData.SenderGUID, eventData.SenderName, eventData.Language, eventData.Message, eventData.SenderChatTag)
	return nil
}

func (s *GameSession) HandleEventChannelJoined(ctx context.Context, e *eBroadcaster.Event) error {
	eventData := e.Payload.(*eBroadcaster.ChannelJoinedPayload)

	// Only send if we're a member of this channel
	if !s.channelMembership.IsMemberForEvent(eventData.ChannelName, eventData.RealmID, eventData.TeamID) {
		return nil
	}

	// Don't send to the player who just joined (they got YOU_JOINED notification)
	if s.character != nil && wowguid.SamePlayer(root.RealmID, s.character.GUID, eventData.RealmID, eventData.PlayerGUID) {
		return nil
	}

	if ch := s.channelMembership.GetChannel(eventData.ChannelName); ch != nil {
		// Only notify in custom channels
		if ch.Flags&ChannelFlagCustom == 0 {
			return nil
		}
	}

	s.ChannelNotify(&ChannelInfo{Name: eventData.ChannelName}).Joined(playerObjectGUIDForRealm(eventData.RealmID, eventData.PlayerGUID))
	return nil
}

func (s *GameSession) HandleEventChannelLeft(ctx context.Context, e *eBroadcaster.Event) error {
	eventData := e.Payload.(*eBroadcaster.ChannelLeftPayload)
	if eventData.Silent {
		return nil
	}

	// Only send if we're a member of this channel
	if !s.channelMembership.IsMemberForEvent(eventData.ChannelName, eventData.RealmID, eventData.TeamID) {
		return nil
	}

	// Don't send to the player who just left (they got YOU_LEFT notification)
	if s.character != nil && wowguid.SamePlayer(root.RealmID, s.character.GUID, eventData.RealmID, eventData.PlayerGUID) {
		return nil
	}

	s.ChannelNotify(&ChannelInfo{Name: eventData.ChannelName}).Left(playerObjectGUIDForRealm(eventData.RealmID, eventData.PlayerGUID))
	return nil
}

func (s *GameSession) InterceptWorldserverChannelNotify(ctx context.Context, p *packet.Packet) error {
	r := p.Reader()
	notifyType := r.Uint8()
	channelName := r.String()

	switch notifyType {
	case ChatYouJoinedNotice:
		flags := r.Uint8()
		channelID := r.Uint32()
		_ = r.Uint32()
		if r.Error() == nil && flags&ChannelFlagCustom == 0 && s.channelMembership != nil {
			channelRealmID, channelTeamID, servicePlayerGUID := s.channelScopeForID(channelID)
			s.channelMembership.RemoveLocalChannelsByID(channelID, channelName)
			s.channelMembership.AddLocalChannel(channelName, channelID, flags, channelRealmID, uint32(channelTeamID), servicePlayerGUID)
		}
	case ChatYouLeftNotice:
		_ = r.Uint32()
		_ = r.Uint8()
		if r.Error() == nil && s.channelMembership != nil {
			s.channelMembership.RemoveChannel(channelName)
		}
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

	if eventData.AffectsPlayer != 0 {
		if s.character == nil || !wowguid.SamePlayer(root.RealmID, s.character.GUID, eventData.RealmID, eventData.AffectsPlayer) {
			return nil
		}
	}

	// Check if we're a member (not required for invitations)
	isMember := s.channelMembership.IsMemberForEvent(eventData.ChannelName, eventData.RealmID, eventData.TeamID)
	if !isMember && eventData.NotifyType != ChatInviteNotice {
		return nil
	}

	// Build and send the notification packet
	w := packet.NewWriterWithSize(packet.SMsgChannelNotify, 0)
	w.Uint8(eventData.NotifyType)
	w.String(eventData.ChannelName)
	targetGUID := playerObjectGUIDForRealm(eventData.RealmID, eventData.TargetGUID)
	secondGUID := playerObjectGUIDForRealm(eventData.RealmID, eventData.SecondGUID)
	appendAzerothCoreChannelNotifyPayload(
		w,
		eventData.NotifyType,
		targetGUID,
		secondGUID,
		eventData.TargetName,
		eventData.ExtraData,
		eventData.OldFlags,
		eventData.NewFlags,
	)

	s.gameSocket.Send(w)
	if s.character != nil &&
		(eventData.NotifyType == ChatPlayerKickedNotice || eventData.NotifyType == ChatPlayerBannedNotice) &&
		wowguid.SamePlayer(root.RealmID, s.character.GUID, eventData.RealmID, eventData.TargetGUID) &&
		s.channelMembership != nil {
		s.channelMembership.RemoveChannel(eventData.ChannelName)
	}
	return nil
}
