package events_broadcaster

import (
	"sync"

	"github.com/rs/zerolog/log"

	"github.com/walkline/ToCloud9/apps/gateway"
	"github.com/walkline/ToCloud9/shared/events"
	"github.com/walkline/ToCloud9/shared/wow/guid"
)

type EventType int

const (
	EventTypeIncomingWhisper EventType = iota + 1
	EventTypeIncomingMail
	EventTypeGuildInviteCreated
	EventTypeGuildMemberPromoted
	EventTypeGuildMemberDemoted
	EventTypeGuildMemberAdded
	EventTypeGuildMemberLeft
	EventTypeGuildMemberKicked
	EventTypeGuildMOTDUpdated
	EventTypeGuildRankUpdated
	EventTypeGuildRankCreated
	EventTypeGuildRankDeleted
	EventTypeGuildNewMessage
	EventTypeGuildPetitionOffered
	EventTypeGuildPetitionSigned
	EventTypeGroupInviteCreated
	EventTypeGroupCreated
	EventTypeGroupMemberOnlineStatusChanged
	EventTypeGroupMemberLeft
	EventTypeGroupDisband
	EventTypeGroupMemberAdded
	EventTypeGroupLeaderChanged
	EventTypeGroupLootTypeChanged
	EventTypeGroupConvertedToRaid
	EventTypeGroupNewMessage
	EventTypeGroupNewTargetIcon
	EventTypeGroupDifficultyChanged
	EventTypeGroupReadyCheckStarted
	EventTypeGroupReadyCheckMemberState
	EventTypeGroupReadyCheckFinished
	EventTypeGroupMemberSubGroupChanged
	EventTypeGroupMemberFlagsChanged
	EventTypeGroupMemberStateChanged
	EventTypeGroupMemberStatesChanged
	EventTypeGroupInviteDeclined
	EventTypeMMJoinedPVPQueue
	EventTypeMMInvitedToBGOrArena
	EventTypeMMInviteToBGOrArenaExpired
	EventTypeMMLfgStatusChanged
	EventTypeMMLfgProposalAccepted
	EventTypeMMServiceUnavailable
	EventTypeFriendStatusChange
	EventTypeFriendAdded
	EventTypeFriendRemoved
	EventTypeFriendNoteUpdate
	EventTypeChannelMessage
	EventTypeChannelJoined
	EventTypeChannelLeft
	EventTypeChannelNotification
	EventTypeArenaTeamInviteCreated
	EventTypeArenaTeamNativeEvent
)

type IncomingWhisperPayload struct {
	SenderRealmID   uint32
	SenderGUID      uint64
	SenderName      string
	SenderRace      uint8
	SenderClass     uint8
	SenderGender    uint8
	SenderChatTag   uint8
	ReceiverRealmID uint32
	ReceiverGUID    uint64
	ReceiverName    string
	Language        uint32
	Msg             string
}

type ChannelMessagePayload struct {
	RealmID       uint32
	ChannelName   string
	ChannelID     uint32
	TeamID        uint32
	SenderGUID    uint64
	SenderName    string
	Language      uint32
	Message       string
	SenderChatTag uint8
}

type ChannelJoinedPayload struct {
	RealmID      uint32
	ChannelName  string
	ChannelID    uint32
	ChannelFlags uint32
	TeamID       uint32
	NumMembers   uint32
	PlayerGUID   uint64
	PlayerName   string
	PlayerFlags  uint8
}

type ChannelLeftPayload struct {
	RealmID      uint32
	ChannelName  string
	ChannelID    uint32
	ChannelFlags uint32
	TeamID       uint32
	NumMembers   uint32
	PlayerGUID   uint64
	PlayerName   string
	Silent       bool
}

type ChannelNotificationPayload struct {
	RealmID       uint32
	ChannelName   string
	ChannelID     uint32
	ChannelFlags  uint32
	TeamID        uint32
	NumMembers    uint32
	NotifyType    uint8
	TargetGUID    uint64
	TargetName    string
	SecondGUID    uint64
	OldFlags      uint8
	NewFlags      uint8
	ExtraData     string
	AffectsPlayer uint64
}

type GuildInviteCreatedPayload struct {
	RealmID uint32

	GuildID   uint64
	GuildName string

	InviterGUID uint64
	InviterName string

	InviteeGUID uint64
	InviteeName string
}

type Event struct {
	Type    EventType
	Payload interface{}
}

//go:generate mockery --name=Broadcaster
type Broadcaster interface {
	RegisterCharacter(charGUID uint64) <-chan Event
	UnregisterCharacter(charGUID uint64)

	NewIncomingWhisperEvent(payload *IncomingWhisperPayload)
	NewIncomingMailEvent(payload *events.MailEventIncomingMailPayload)
	NewGuildInviteCreatedEvent(payload *GuildInviteCreatedPayload)
	NewGuildMemberPromoteEvent(payload *events.GuildEventMemberPromotePayload)
	NewGuildMemberDemoteEvent(payload *events.GuildEventMemberDemotePayload)
	NewGuildMemberAddedEvent(payload *events.GuildEventMemberAddedPayload)
	NewGuildMemberLeftEvent(payload *events.GuildEventMemberLeftPayload)
	NewGuildMemberKickedEvent(payload *events.GuildEventMemberKickedPayload)
	NewGuildMOTDUpdatedEvent(payload *events.GuildEventMOTDUpdatedPayload)
	NewGuildRankUpdatedEvent(payload *events.GuildEventRankUpdatedPayload)
	NewGuildRankCreatedEvent(payload *events.GuildEventRankCreatedPayload)
	NewGuildRankDeletedEvent(payload *events.GuildEventRankDeletedPayload)
	NewGuildMessageEvent(payload *events.GuildEventNewMessagePayload)
	NewGuildPetitionOfferedEvent(payload *events.GuildEventPetitionOfferedPayload)
	NewGuildPetitionSignedEvent(payload *events.GuildEventPetitionSignedPayload)
	NewGroupInviteCreatedEvent(payload *events.GroupEventInviteCreatedPayload)
	NewGroupInviteDeclinedEvent(payload *events.GroupEventInviteDeclinedPayload)
	NewGroupCreatedEvent(payload *events.GroupEventGroupCreatedPayload)
	NewGroupMemberOnlineStatusChangedEvent(payload *events.GroupEventGroupMemberOnlineStatusChangedPayload)
	NewGroupMemberLeftEvent(payload *events.GroupEventGroupMemberLeftPayload)
	NewGroupDisbandEvent(payload *events.GroupEventGroupDisbandPayload)
	NewGroupMemberAddedEvent(payload *events.GroupEventGroupMemberAddedPayload)
	NewGroupLeaderChangedEvent(payload *events.GroupEventGroupLeaderChangedPayload)
	NewGroupLootTypeChangedEvent(payload *events.GroupEventGroupLootTypeChangedPayload)
	NewGroupConvertedToRaidEvent(payload *events.GroupEventGroupConvertedToRaidPayload)
	NewGroupMessageEvent(payload *events.GroupEventNewMessagePayload)
	NewGroupTargetIconEvent(payload *events.GroupEventNewTargetIconPayload)
	NewGroupDifficultyChangedEvent(payload *events.GroupEventGroupDifficultyChangedPayload)
	NewGroupReadyCheckStartedEvent(payload *events.GroupEventReadyCheckStartedPayload)
	NewGroupReadyCheckMemberStateEvent(payload *events.GroupEventReadyCheckMemberStatePayload)
	NewGroupReadyCheckFinishedEvent(payload *events.GroupEventReadyCheckFinishedPayload)
	NewGroupMemberSubGroupChangedEvent(payload *events.GroupEventMemberSubGroupChangedPayload)
	NewGroupMemberFlagsChangedEvent(payload *events.GroupEventMemberFlagsChangedPayload)
	NewGroupMemberStateChangedEvent(payload *events.GroupEventMemberStateChangedPayload)
	NewGroupMemberStatesChangedEvent(payload *events.GroupEventMemberStatesChangedPayload)

	NewMatchmakingJoinedPVPQueueEvent(payload *events.MatchmakingEventPlayersQueuedPayload)
	NewMatchmakingInvitedToBGOrArenaEvent(payload *events.MatchmakingEventPlayersInvitedPayload)
	NewMatchmakingInviteToBGOrArenaExpiredEvent(payload *events.MatchmakingEventPlayersInviteExpiredPayload)
	NewMatchmakingLfgStatusChangedEvent(payload *events.MatchmakingEventLfgStatusChangedPayload)
	NewMatchmakingLfgProposalAcceptedEvent(payload *events.MatchmakingEventLfgProposalAcceptedPayload)
	NewMatchmakingServiceUnavailableEvent(payload *events.ServerRegistryEventMatchmakingRemovedUnhealthyPayload)

	NewFriendStatusChangeEvent(payload *events.FriendEventStatusChangePayload)
	NewFriendAddedEvent(payload *events.FriendEventAddedPayload)
	NewFriendRemovedEvent(payload *events.FriendEventRemovedPayload)
	NewFriendNoteUpdateEvent(payload *events.FriendEventNoteUpdatePayload)

	NewChannelMessageEvent(payload *ChannelMessagePayload)
	NewChannelJoinedEvent(payload *ChannelJoinedPayload)
	NewChannelLeftEvent(payload *ChannelLeftPayload)
	NewChannelNotificationEvent(payload *ChannelNotificationPayload)
	NewArenaTeamInviteCreatedEvent(payload *events.CharEventArenaTeamInviteCreatedPayload)
	NewArenaTeamNativeEvent(payload *events.CharEventArenaTeamNativeEventPayload)
}

type broadcasterImpl struct {
	channels   map[uint64]chan Event
	channelsMu sync.RWMutex

	chatChannelsService *ChatChannelsService
}

func NewBroadcaster(chatChannelsService *ChatChannelsService) Broadcaster {
	return &broadcasterImpl{
		channels:            map[uint64]chan Event{},
		chatChannelsService: chatChannelsService,
	}
}

func (b *broadcasterImpl) RegisterCharacter(charGUID uint64) <-chan Event {
	const eventsChanBufferSize = 100

	ch := make(chan Event, eventsChanBufferSize)

	b.channelsMu.Lock()
	b.channels[charGUID] = ch
	b.channelsMu.Unlock()

	return ch
}

func (b *broadcasterImpl) UnregisterCharacter(charGUID uint64) {
	b.channelsMu.Lock()
	delete(b.channels, charGUID)
	b.channelsMu.Unlock()
}

func (b *broadcasterImpl) sendEvent(ch chan Event, event Event) {
	if ch == nil {
		return
	}

	select {
	case ch <- event:
	default:
		log.Warn().
			Int("eventType", int(event.Type)).
			Msg("dropping gateway event because session event queue is full")
	}
}

func (b *broadcasterImpl) NewIncomingWhisperEvent(payload *IncomingWhisperPayload) {
	b.channelsMu.RLock()
	ch, ok := b.channelForGUIDLocked(payload.ReceiverGUID)
	b.channelsMu.RUnlock()

	if !ok {
		return
	}

	b.sendEvent(ch, Event{
		Type:    EventTypeIncomingWhisper,
		Payload: payload,
	})
}

func (b *broadcasterImpl) NewIncomingMailEvent(payload *events.MailEventIncomingMailPayload) {
	if !isLocalRealmEvent(payload.RealmID) {
		return
	}

	b.channelsMu.RLock()
	ch, ok := b.channelForGUIDLocked(payload.ReceiverGUID)
	b.channelsMu.RUnlock()

	if !ok {
		return
	}

	b.sendEvent(ch, Event{
		Type:    EventTypeIncomingMail,
		Payload: payload,
	})
}

func (b *broadcasterImpl) NewGuildInviteCreatedEvent(payload *GuildInviteCreatedPayload) {
	if !isLocalRealmEvent(payload.RealmID) {
		return
	}

	b.channelsMu.RLock()
	ch, ok := b.channelForGUIDLocked(payload.InviteeGUID)
	b.channelsMu.RUnlock()

	if !ok {
		return
	}

	b.sendEvent(ch, Event{
		Type:    EventTypeGuildInviteCreated,
		Payload: payload,
	})
}

func (b *broadcasterImpl) NewGuildMemberPromoteEvent(payload *events.GuildEventMemberPromotePayload) {
	if !isLocalRealmEvent(payload.RealmID) {
		return
	}

	for _, ch := range b.channelsForGUIDs(payload.MembersOnline) {
		b.sendEvent(ch, Event{
			Type:    EventTypeGuildMemberPromoted,
			Payload: payload,
		})
	}
}

func (b *broadcasterImpl) NewGuildMemberDemoteEvent(payload *events.GuildEventMemberDemotePayload) {
	if !isLocalRealmEvent(payload.RealmID) {
		return
	}

	for _, ch := range b.channelsForGUIDs(payload.MembersOnline) {
		b.sendEvent(ch, Event{
			Type:    EventTypeGuildMemberDemoted,
			Payload: payload,
		})
	}
}

func (b *broadcasterImpl) NewGuildMemberAddedEvent(payload *events.GuildEventMemberAddedPayload) {
	if !isLocalRealmEvent(payload.RealmID) {
		return
	}

	for _, ch := range b.channelsForGUIDs(payload.MembersOnline) {
		b.sendEvent(ch, Event{
			Type:    EventTypeGuildMemberAdded,
			Payload: payload,
		})
	}
}

func (b *broadcasterImpl) NewGuildMemberLeftEvent(payload *events.GuildEventMemberLeftPayload) {
	if !isLocalRealmEvent(payload.RealmID) {
		return
	}

	for _, ch := range b.channelsForGUIDs(payload.MembersOnline) {
		b.sendEvent(ch, Event{
			Type:    EventTypeGuildMemberLeft,
			Payload: payload,
		})
	}
}

func (b *broadcasterImpl) NewGuildMemberKickedEvent(payload *events.GuildEventMemberKickedPayload) {
	if !isLocalRealmEvent(payload.RealmID) {
		return
	}

	for _, ch := range b.channelsForGUIDs(payload.MembersOnline) {
		b.sendEvent(ch, Event{
			Type:    EventTypeGuildMemberKicked,
			Payload: payload,
		})
	}
}

func (b *broadcasterImpl) NewGuildMOTDUpdatedEvent(payload *events.GuildEventMOTDUpdatedPayload) {
	if !isLocalRealmEvent(payload.RealmID) {
		return
	}

	for _, ch := range b.channelsForGUIDs(payload.MembersOnline) {
		b.sendEvent(ch, Event{
			Type:    EventTypeGuildMOTDUpdated,
			Payload: payload,
		})
	}
}

func (b *broadcasterImpl) NewGuildRankUpdatedEvent(payload *events.GuildEventRankUpdatedPayload) {
	if !isLocalRealmEvent(payload.RealmID) {
		return
	}

	for _, ch := range b.channelsForGUIDs(payload.MembersOnline) {
		b.sendEvent(ch, Event{
			Type:    EventTypeGuildRankUpdated,
			Payload: payload,
		})
	}
}

func (b *broadcasterImpl) NewGuildRankCreatedEvent(payload *events.GuildEventRankCreatedPayload) {
	if !isLocalRealmEvent(payload.RealmID) {
		return
	}

	for _, ch := range b.channelsForGUIDs(payload.MembersOnline) {
		b.sendEvent(ch, Event{
			Type:    EventTypeGuildRankCreated,
			Payload: payload,
		})
	}
}

func (b *broadcasterImpl) NewGuildRankDeletedEvent(payload *events.GuildEventRankDeletedPayload) {
	if !isLocalRealmEvent(payload.RealmID) {
		return
	}

	for _, ch := range b.channelsForGUIDs(payload.MembersOnline) {
		b.sendEvent(ch, Event{
			Type:    EventTypeGuildRankDeleted,
			Payload: payload,
		})
	}
}

func (b *broadcasterImpl) NewGuildMessageEvent(payload *events.GuildEventNewMessagePayload) {
	if !isLocalRealmEvent(payload.RealmID) {
		return
	}

	for _, ch := range b.channelsForGUIDs(payload.Receivers) {
		b.sendEvent(ch, Event{
			Type:    EventTypeGuildNewMessage,
			Payload: payload,
		})
	}
}

func (b *broadcasterImpl) NewGuildPetitionOfferedEvent(payload *events.GuildEventPetitionOfferedPayload) {
	if !isLocalRealmEvent(payload.RealmID) {
		return
	}

	b.channelsMu.RLock()
	ch, ok := b.channelForGUIDLocked(payload.TargetGUID)
	b.channelsMu.RUnlock()

	if !ok {
		return
	}

	b.sendEvent(ch, Event{
		Type:    EventTypeGuildPetitionOffered,
		Payload: payload,
	})
}

func (b *broadcasterImpl) NewGuildPetitionSignedEvent(payload *events.GuildEventPetitionSignedPayload) {
	if !isLocalRealmEvent(payload.RealmID) {
		return
	}

	b.channelsMu.RLock()
	ch, ok := b.channelForGUIDLocked(payload.OwnerGUID)
	b.channelsMu.RUnlock()

	if !ok {
		return
	}

	b.sendEvent(ch, Event{
		Type:    EventTypeGuildPetitionSigned,
		Payload: payload,
	})
}

func (b *broadcasterImpl) NewGroupInviteCreatedEvent(payload *events.GroupEventInviteCreatedPayload) {
	b.channelsMu.RLock()
	ch, ok := b.channelForGroupGUIDLocked(payload.RealmID, payload.InviteeGUID)
	b.channelsMu.RUnlock()

	if !ok {
		return
	}

	b.sendEvent(ch, Event{
		Type:    EventTypeGroupInviteCreated,
		Payload: payload,
	})
}

func (b *broadcasterImpl) NewGroupInviteDeclinedEvent(payload *events.GroupEventInviteDeclinedPayload) {
	b.channelsMu.RLock()
	ch, ok := b.channelForGroupGUIDLocked(payload.RealmID, payload.InviterGUID)
	b.channelsMu.RUnlock()

	if !ok {
		return
	}

	b.sendEvent(ch, Event{
		Type:    EventTypeGroupInviteDeclined,
		Payload: payload,
	})
}

func (b *broadcasterImpl) NewGroupCreatedEvent(payload *events.GroupEventGroupCreatedPayload) {
	membersGuids := make([]uint64, len(payload.Members))
	for i := range payload.Members {
		membersGuids[i] = payload.Members[i].MemberGUID
	}
	for _, ch := range b.channelsForGroupGUIDs(payload.RealmID, membersGuids) {
		b.sendEvent(ch, Event{
			Type:    EventTypeGroupCreated,
			Payload: payload,
		})
	}
}

func (b *broadcasterImpl) NewGroupMemberOnlineStatusChangedEvent(payload *events.GroupEventGroupMemberOnlineStatusChangedPayload) {
	for _, ch := range b.channelsForGroupGUIDs(payload.RealmID, payload.OnlineMembers) {
		b.sendEvent(ch, Event{
			Type:    EventTypeGroupMemberOnlineStatusChanged,
			Payload: payload,
		})
	}
}

func (b *broadcasterImpl) NewGroupMemberLeftEvent(payload *events.GroupEventGroupMemberLeftPayload) {
	for _, ch := range b.channelsForGroupGUIDs(payload.RealmID, payload.OnlineMembers) {
		b.sendEvent(ch, Event{
			Type:    EventTypeGroupMemberLeft,
			Payload: payload,
		})
	}
}

func (b *broadcasterImpl) NewGroupDisbandEvent(payload *events.GroupEventGroupDisbandPayload) {
	for _, ch := range b.channelsForGroupGUIDs(payload.RealmID, payload.OnlineMembers) {
		b.sendEvent(ch, Event{
			Type:    EventTypeGroupDisband,
			Payload: payload,
		})
	}
}

func (b *broadcasterImpl) NewGroupMemberAddedEvent(payload *events.GroupEventGroupMemberAddedPayload) {
	for _, ch := range b.channelsForGroupGUIDs(payload.RealmID, payload.OnlineMembers) {
		b.sendEvent(ch, Event{
			Type:    EventTypeGroupMemberAdded,
			Payload: payload,
		})
	}
}

func (b *broadcasterImpl) NewGroupLeaderChangedEvent(payload *events.GroupEventGroupLeaderChangedPayload) {
	for _, ch := range b.channelsForGroupGUIDs(payload.RealmID, payload.OnlineMembers) {
		b.sendEvent(ch, Event{
			Type:    EventTypeGroupLeaderChanged,
			Payload: payload,
		})
	}
}

func (b *broadcasterImpl) NewGroupLootTypeChangedEvent(payload *events.GroupEventGroupLootTypeChangedPayload) {
	for _, ch := range b.channelsForGroupGUIDs(payload.RealmID, payload.OnlineMembers) {
		b.sendEvent(ch, Event{
			Type:    EventTypeGroupLootTypeChanged,
			Payload: payload,
		})
	}
}

func (b *broadcasterImpl) NewGroupConvertedToRaidEvent(payload *events.GroupEventGroupConvertedToRaidPayload) {
	for _, ch := range b.channelsForGroupGUIDs(payload.RealmID, payload.OnlineMembers) {
		b.sendEvent(ch, Event{
			Type:    EventTypeGroupConvertedToRaid,
			Payload: payload,
		})
	}
}

func (b *broadcasterImpl) NewGroupMessageEvent(payload *events.GroupEventNewMessagePayload) {
	for _, ch := range b.channelsForGroupGUIDs(payload.RealmID, payload.Receivers) {
		b.sendEvent(ch, Event{
			Type:    EventTypeGroupNewMessage,
			Payload: payload,
		})
	}
}

func (b *broadcasterImpl) NewGroupTargetIconEvent(payload *events.GroupEventNewTargetIconPayload) {
	for _, ch := range b.channelsForGroupGUIDs(payload.RealmID, payload.Receivers) {
		b.sendEvent(ch, Event{
			Type:    EventTypeGroupNewTargetIcon,
			Payload: payload,
		})
	}
}

func (b *broadcasterImpl) NewGroupDifficultyChangedEvent(payload *events.GroupEventGroupDifficultyChangedPayload) {
	for _, ch := range b.channelsForGroupGUIDs(payload.RealmID, payload.Receivers) {
		b.sendEvent(ch, Event{
			Type:    EventTypeGroupDifficultyChanged,
			Payload: payload,
		})
	}
}

func (b *broadcasterImpl) NewGroupReadyCheckStartedEvent(payload *events.GroupEventReadyCheckStartedPayload) {
	for _, ch := range b.channelsForGroupGUIDs(payload.RealmID, payload.Receivers) {
		b.sendEvent(ch, Event{
			Type:    EventTypeGroupReadyCheckStarted,
			Payload: payload,
		})
	}
}

func (b *broadcasterImpl) NewGroupReadyCheckMemberStateEvent(payload *events.GroupEventReadyCheckMemberStatePayload) {
	for _, ch := range b.channelsForGroupGUIDs(payload.RealmID, payload.Receivers) {
		b.sendEvent(ch, Event{
			Type:    EventTypeGroupReadyCheckMemberState,
			Payload: payload,
		})
	}
}

func (b *broadcasterImpl) NewGroupReadyCheckFinishedEvent(payload *events.GroupEventReadyCheckFinishedPayload) {
	for _, ch := range b.channelsForGroupGUIDs(payload.RealmID, payload.Receivers) {
		b.sendEvent(ch, Event{
			Type:    EventTypeGroupReadyCheckFinished,
			Payload: payload,
		})
	}
}

func (b *broadcasterImpl) NewGroupMemberSubGroupChangedEvent(payload *events.GroupEventMemberSubGroupChangedPayload) {
	for _, ch := range b.channelsForGroupGUIDs(payload.RealmID, payload.Receivers) {
		b.sendEvent(ch, Event{
			Type:    EventTypeGroupMemberSubGroupChanged,
			Payload: payload,
		})
	}
}

func (b *broadcasterImpl) NewGroupMemberFlagsChangedEvent(payload *events.GroupEventMemberFlagsChangedPayload) {
	for _, ch := range b.channelsForGroupGUIDs(payload.RealmID, payload.Receivers) {
		b.sendEvent(ch, Event{
			Type:    EventTypeGroupMemberFlagsChanged,
			Payload: payload,
		})
	}
}

func (b *broadcasterImpl) NewGroupMemberStateChangedEvent(payload *events.GroupEventMemberStateChangedPayload) {
	for _, ch := range b.channelsForGroupGUIDs(payload.RealmID, payload.Receivers) {
		b.sendEvent(ch, Event{
			Type:    EventTypeGroupMemberStateChanged,
			Payload: payload,
		})
	}
}

func (b *broadcasterImpl) NewGroupMemberStatesChangedEvent(payload *events.GroupEventMemberStatesChangedPayload) {
	for _, ch := range b.channelsForGroupGUIDs(payload.RealmID, payload.Receivers) {
		b.sendEvent(ch, Event{
			Type:    EventTypeGroupMemberStatesChanged,
			Payload: payload,
		})
	}
}

func (b *broadcasterImpl) NewMatchmakingJoinedPVPQueueEvent(payload *events.MatchmakingEventPlayersQueuedPayload) {
	if !isLocalRealmEvent(payload.RealmID) {
		return
	}

	for _, ch := range b.channelsForGUIDs(convertLowGUIDsToUint64(payload.PlayersGUID)) {
		b.sendEvent(ch, Event{
			Type:    EventTypeMMJoinedPVPQueue,
			Payload: payload,
		})
	}
}

func (b *broadcasterImpl) NewMatchmakingInvitedToBGOrArenaEvent(payload *events.MatchmakingEventPlayersInvitedPayload) {
	if !isLocalRealmEvent(payload.RealmID) {
		return
	}

	for _, ch := range b.channelsForGUIDs(convertLowGUIDsToUint64(payload.PlayersGUID)) {
		b.sendEvent(ch, Event{
			Type:    EventTypeMMInvitedToBGOrArena,
			Payload: payload,
		})
	}
}

func (b *broadcasterImpl) NewMatchmakingInviteToBGOrArenaExpiredEvent(payload *events.MatchmakingEventPlayersInviteExpiredPayload) {
	if !isLocalRealmEvent(payload.RealmID) {
		return
	}

	for _, ch := range b.channelsForGUIDs(convertLowGUIDsToUint64(payload.PlayersGUID)) {
		b.sendEvent(ch, Event{
			Type:    EventTypeMMInviteToBGOrArenaExpired,
			Payload: payload,
		})
	}
}

func (b *broadcasterImpl) NewMatchmakingLfgStatusChangedEvent(payload *events.MatchmakingEventLfgStatusChangedPayload) {
	if !isLocalRealmEvent(payload.RealmID) {
		return
	}

	for _, ch := range b.channelsForGUIDs(convertLowGUIDsToUint64(payload.PlayersGUID)) {
		b.sendEvent(ch, Event{
			Type:    EventTypeMMLfgStatusChanged,
			Payload: payload,
		})
	}
}

func (b *broadcasterImpl) NewMatchmakingLfgProposalAcceptedEvent(payload *events.MatchmakingEventLfgProposalAcceptedPayload) {
	playerGUIDs := lfgProposalAcceptedLocalPlayers(payload)
	if len(playerGUIDs) == 0 {
		return
	}

	for _, ch := range b.channelsForGUIDs(playerGUIDs) {
		b.sendEvent(ch, Event{
			Type:    EventTypeMMLfgProposalAccepted,
			Payload: payload,
		})
	}
}

func (b *broadcasterImpl) NewMatchmakingServiceUnavailableEvent(payload *events.ServerRegistryEventMatchmakingRemovedUnhealthyPayload) {
	b.channelsMu.RLock()
	channels := make([]chan Event, 0, len(b.channels))
	for _, ch := range b.channels {
		channels = append(channels, ch)
	}
	b.channelsMu.RUnlock()

	for _, ch := range channels {
		b.sendEvent(ch, Event{
			Type:    EventTypeMMServiceUnavailable,
			Payload: payload,
		})
	}
}

func (b *broadcasterImpl) NewFriendStatusChangeEvent(payload *events.FriendEventStatusChangePayload) {
	if !isLocalRealmEvent(payload.RealmID) {
		return
	}

	for _, ch := range b.channelsForGUIDs(payload.NotifyPlayers) {
		b.sendEvent(ch, Event{
			Type:    EventTypeFriendStatusChange,
			Payload: payload,
		})
	}
}

func (b *broadcasterImpl) NewFriendAddedEvent(payload *events.FriendEventAddedPayload) {
	if !isLocalRealmEvent(payload.RealmID) {
		return
	}

	b.channelsMu.RLock()
	ch, ok := b.channelForGUIDLocked(payload.PlayerGUID)
	b.channelsMu.RUnlock()

	if !ok {
		return
	}

	b.sendEvent(ch, Event{
		Type:    EventTypeFriendAdded,
		Payload: payload,
	})
}

func (b *broadcasterImpl) NewFriendRemovedEvent(payload *events.FriendEventRemovedPayload) {
	if !isLocalRealmEvent(payload.RealmID) {
		return
	}

	b.channelsMu.RLock()
	ch, ok := b.channelForGUIDLocked(payload.PlayerGUID)
	b.channelsMu.RUnlock()

	if !ok {
		return
	}

	b.sendEvent(ch, Event{
		Type:    EventTypeFriendRemoved,
		Payload: payload,
	})
}

func (b *broadcasterImpl) NewFriendNoteUpdateEvent(payload *events.FriendEventNoteUpdatePayload) {
	if !isLocalRealmEvent(payload.RealmID) {
		return
	}

	b.channelsMu.RLock()
	ch, ok := b.channelForGUIDLocked(payload.PlayerGUID)
	b.channelsMu.RUnlock()

	if !ok {
		return
	}

	b.sendEvent(ch, Event{
		Type:    EventTypeFriendNoteUpdate,
		Payload: payload,
	})
}

func (b *broadcasterImpl) NewArenaTeamInviteCreatedEvent(payload *events.CharEventArenaTeamInviteCreatedPayload) {
	if !isLocalRealmEvent(payload.RealmID) {
		return
	}

	b.channelsMu.RLock()
	ch, ok := b.channelForGUIDLocked(payload.TargetGUID)
	b.channelsMu.RUnlock()

	if !ok {
		return
	}

	b.sendEvent(ch, Event{
		Type:    EventTypeArenaTeamInviteCreated,
		Payload: payload,
	})
}

func (b *broadcasterImpl) NewArenaTeamNativeEvent(payload *events.CharEventArenaTeamNativeEventPayload) {
	if !isLocalRealmEvent(payload.RealmID) {
		return
	}

	b.channelsMu.RLock()
	defer b.channelsMu.RUnlock()

	for _, receiverGUID := range payload.ReceiverGUIDs {
		ch, ok := b.channelForGUIDLocked(receiverGUID)
		if !ok {
			continue
		}

		b.sendEvent(ch, Event{
			Type:    EventTypeArenaTeamNativeEvent,
			Payload: payload,
		})
	}
}

func (b *broadcasterImpl) NewChannelMessageEvent(payload *ChannelMessagePayload) {
	if !isLocalRealmEvent(payload.RealmID) {
		return
	}

	b.chatChannelsService.BroadcastToScopedChannel(payload.RealmID, payload.TeamID, payload.ChannelName, Event{
		Type:    EventTypeChannelMessage,
		Payload: payload,
	})
}

func (b *broadcasterImpl) NewChannelJoinedEvent(payload *ChannelJoinedPayload) {
	if !isLocalRealmEvent(payload.RealmID) {
		return
	}

	b.chatChannelsService.BroadcastToScopedChannel(payload.RealmID, payload.TeamID, payload.ChannelName, Event{
		Type:    EventTypeChannelJoined,
		Payload: payload,
	})
}

func (b *broadcasterImpl) NewChannelLeftEvent(payload *ChannelLeftPayload) {
	if !isLocalRealmEvent(payload.RealmID) {
		return
	}

	b.chatChannelsService.BroadcastToScopedChannel(payload.RealmID, payload.TeamID, payload.ChannelName, Event{
		Type:    EventTypeChannelLeft,
		Payload: payload,
	})
}

func (b *broadcasterImpl) NewChannelNotificationEvent(payload *ChannelNotificationPayload) {
	if !isLocalRealmEvent(payload.RealmID) {
		return
	}

	// If AffectsPlayer is set, send only to that specific player (e.g., invitations)
	if payload.AffectsPlayer != 0 {
		b.channelsMu.RLock()
		ch, ok := b.channelForGUIDLocked(payload.AffectsPlayer)
		b.channelsMu.RUnlock()

		if ok {
			b.sendEvent(ch, Event{
				Type:    EventTypeChannelNotification,
				Payload: payload,
			})
		}
		return
	}

	// Otherwise broadcast to all channel members
	b.chatChannelsService.BroadcastToScopedChannel(payload.RealmID, payload.TeamID, payload.ChannelName, Event{
		Type:    EventTypeChannelNotification,
		Payload: payload,
	})
}

func (b *broadcasterImpl) channelsForGUIDs(guids []uint64) []chan Event {
	channels := make([]chan Event, 0, len(guids))
	seen := make(map[chan Event]struct{}, len(guids))
	b.channelsMu.RLock()
	for _, guid := range guids {
		ch, ok := b.channelForGUIDLocked(guid)
		if !ok {
			continue
		}

		if _, ok = seen[ch]; ok {
			continue
		}

		seen[ch] = struct{}{}
		channels = append(channels, ch)
	}
	b.channelsMu.RUnlock()

	return channels
}

func (b *broadcasterImpl) channelsForGroupGUIDs(groupRealmID uint32, guids []uint64) []chan Event {
	channels := make([]chan Event, 0, len(guids))
	seen := make(map[chan Event]struct{}, len(guids))
	b.channelsMu.RLock()
	for _, guid := range guids {
		ch, ok := b.channelForGroupGUIDLocked(groupRealmID, guid)
		if !ok {
			continue
		}

		if _, ok = seen[ch]; ok {
			continue
		}

		seen[ch] = struct{}{}
		channels = append(channels, ch)
	}
	b.channelsMu.RUnlock()

	return channels
}

func isLocalRealmEvent(realmID uint32) bool {
	return realmID == gateway.RealmID
}

func lfgProposalAcceptedLocalPlayers(payload *events.MatchmakingEventLfgProposalAcceptedPayload) []uint64 {
	if payload == nil {
		return nil
	}
	if len(payload.Members) == 0 {
		if !isLocalRealmEvent(payload.RealmID) {
			return nil
		}
		return convertLowGUIDsToUint64(payload.PlayersGUID)
	}

	players := make([]uint64, 0, len(payload.Members))
	for _, member := range payload.Members {
		realmID := member.RealmID
		if realmID == 0 {
			realmID = payload.RealmID
		}
		if realmID != gateway.RealmID {
			continue
		}
		players = append(players, uint64(member.PlayerGUID))
	}
	return players
}

func (b *broadcasterImpl) channelForGUIDLocked(playerGUID uint64) (chan Event, bool) {
	ch, ok := b.channels[playerGUID]
	if ok {
		return ch, true
	}

	lowGUID := playerLowGUID(playerGUID)
	if lowGUID == playerGUID {
		return nil, false
	}

	ch, ok = b.channels[lowGUID]
	return ch, ok
}

func (b *broadcasterImpl) channelForGroupGUIDLocked(groupRealmID uint32, playerGUID uint64) (chan Event, bool) {
	if !isLocalGroupPlayer(groupRealmID, playerGUID) {
		return nil, false
	}
	return b.channelForGUIDLocked(playerGUID)
}

func isLocalGroupPlayer(groupRealmID uint32, playerGUID uint64) bool {
	if playerGUID == 0 {
		return false
	}
	if playerGUID>>48 == 0 {
		if playerRealmID := uint32((playerGUID >> 32) & 0xffff); playerRealmID != 0 {
			return playerRealmID == gateway.RealmID
		}
	}
	return groupRealmID == gateway.RealmID
}

func playerLowGUID(playerGUID uint64) uint64 {
	if playerGUID == 0 || playerGUID>>32 == 0 || playerGUID>>48 != 0 {
		return playerGUID
	}

	if uint32((playerGUID>>32)&0xffff) != gateway.RealmID {
		return playerGUID
	}

	return playerGUID & 0xffffffff
}

func convertLowGUIDsToUint64(s []guid.LowType) []uint64 {
	r := make([]uint64, len(s))
	for i, lowType := range s {
		r[i] = uint64(lowType)
	}
	return r
}
