package events_broadcaster

import (
	"sync"

	gameloadbalancer "github.com/walkline/ToCloud9/apps/game-load-balancer"
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
	EventTypeMMJoinedPVPQueue
	EventTypeMMInvitedToBGOrArena
	EventTypeMMInviteToBGOrArenaExpired
)

type IncomingWhisperPayload struct {
	SenderGUID   uint64
	SenderName   string
	SenderRace   uint8
	ReceiverGUID uint64
	ReceiverName string
	Language     uint32
	Msg          string
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
	NewGroupInviteCreatedEvent(payload *events.GroupEventInviteCreatedPayload)
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

	NewMatchmakingJoinedPVPQueueEvent(payload *events.MatchmakingEventPlayersQueuedPayload)
	NewMatchmakingInvitedToBGOrArenaEvent(payload *events.MatchmakingEventPlayersInvitedPayload)
	NewMatchmakingInviteToBGOrArenaExpiredEvent(payload *events.MatchmakingEventPlayersInviteExpiredPayload)
}

type broadcasterImpl struct {
	channels   map[uint64]chan Event
	channelsMu sync.RWMutex
}

func NewBroadcaster() Broadcaster {
	return &broadcasterImpl{
		channels: map[uint64]chan Event{},
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

func (b *broadcasterImpl) NewIncomingWhisperEvent(payload *IncomingWhisperPayload) {
	b.channelsMu.RLock()
	ch, ok := b.channels[payload.ReceiverGUID]
	b.channelsMu.RUnlock()

	if !ok {
		return
	}

	ch <- Event{
		Type:    EventTypeIncomingWhisper,
		Payload: payload,
	}
}

func (b *broadcasterImpl) NewIncomingMailEvent(payload *events.MailEventIncomingMailPayload) {
	b.channelsMu.RLock()
	ch, ok := b.channels[payload.ReceiverGUID]
	b.channelsMu.RUnlock()

	if !ok {
		return
	}

	ch <- Event{
		Type:    EventTypeIncomingMail,
		Payload: payload,
	}
}

func (b *broadcasterImpl) NewGuildInviteCreatedEvent(payload *GuildInviteCreatedPayload) {
	b.channelsMu.RLock()
	ch, ok := b.channels[payload.InviteeGUID]
	b.channelsMu.RUnlock()

	if !ok {
		return
	}

	ch <- Event{
		Type:    EventTypeGuildInviteCreated,
		Payload: payload,
	}
}

func (b *broadcasterImpl) NewGuildMemberPromoteEvent(payload *events.GuildEventMemberPromotePayload) {
	for _, ch := range b.channelsForGUIDs(payload.MembersOnline) {
		ch <- Event{
			Type:    EventTypeGuildMemberPromoted,
			Payload: payload,
		}
	}
}

func (b *broadcasterImpl) NewGuildMemberDemoteEvent(payload *events.GuildEventMemberDemotePayload) {
	for _, ch := range b.channelsForGUIDs(payload.MembersOnline) {
		ch <- Event{
			Type:    EventTypeGuildMemberDemoted,
			Payload: payload,
		}
	}
}

func (b *broadcasterImpl) NewGuildMemberAddedEvent(payload *events.GuildEventMemberAddedPayload) {
	for _, ch := range b.channelsForGUIDs(payload.MembersOnline) {
		ch <- Event{
			Type:    EventTypeGuildMemberAdded,
			Payload: payload,
		}
	}
}

func (b *broadcasterImpl) NewGuildMemberLeftEvent(payload *events.GuildEventMemberLeftPayload) {
	for _, ch := range b.channelsForGUIDs(payload.MembersOnline) {
		ch <- Event{
			Type:    EventTypeGuildMemberLeft,
			Payload: payload,
		}
	}
}

func (b *broadcasterImpl) NewGuildMemberKickedEvent(payload *events.GuildEventMemberKickedPayload) {
	for _, ch := range b.channelsForGUIDs(payload.MembersOnline) {
		ch <- Event{
			Type:    EventTypeGuildMemberKicked,
			Payload: payload,
		}
	}
}

func (b *broadcasterImpl) NewGuildMOTDUpdatedEvent(payload *events.GuildEventMOTDUpdatedPayload) {
	for _, ch := range b.channelsForGUIDs(payload.MembersOnline) {
		ch <- Event{
			Type:    EventTypeGuildMOTDUpdated,
			Payload: payload,
		}
	}
}

func (b *broadcasterImpl) NewGuildRankUpdatedEvent(payload *events.GuildEventRankUpdatedPayload) {
	for _, ch := range b.channelsForGUIDs(payload.MembersOnline) {
		ch <- Event{
			Type:    EventTypeGuildRankUpdated,
			Payload: payload,
		}
	}
}

func (b *broadcasterImpl) NewGuildRankCreatedEvent(payload *events.GuildEventRankCreatedPayload) {
	for _, ch := range b.channelsForGUIDs(payload.MembersOnline) {
		ch <- Event{
			Type:    EventTypeGuildRankCreated,
			Payload: payload,
		}
	}
}

func (b *broadcasterImpl) NewGuildRankDeletedEvent(payload *events.GuildEventRankDeletedPayload) {
	for _, ch := range b.channelsForGUIDs(payload.MembersOnline) {
		ch <- Event{
			Type:    EventTypeGuildRankDeleted,
			Payload: payload,
		}
	}
}

func (b *broadcasterImpl) NewGuildMessageEvent(payload *events.GuildEventNewMessagePayload) {
	for _, ch := range b.channelsForGUIDs(payload.Receivers) {
		ch <- Event{
			Type:    EventTypeGuildNewMessage,
			Payload: payload,
		}
	}
}

func (b *broadcasterImpl) NewGroupInviteCreatedEvent(payload *events.GroupEventInviteCreatedPayload) {
	b.channelsMu.RLock()
	ch, ok := b.channels[payload.InviteeGUID]
	b.channelsMu.RUnlock()

	if !ok {
		return
	}

	ch <- Event{
		Type:    EventTypeGroupInviteCreated,
		Payload: payload,
	}
}

func (b *broadcasterImpl) NewGroupCreatedEvent(payload *events.GroupEventGroupCreatedPayload) {
	membersGuids := make([]uint64, len(payload.Members))
	for i := range payload.Members {
		membersGuids[i] = payload.Members[i].MemberGUID
	}
	for _, ch := range b.channelsForGUIDs(membersGuids) {
		ch <- Event{
			Type:    EventTypeGroupCreated,
			Payload: payload,
		}
	}
}

func (b *broadcasterImpl) NewGroupMemberOnlineStatusChangedEvent(payload *events.GroupEventGroupMemberOnlineStatusChangedPayload) {
	for _, ch := range b.channelsForGUIDs(payload.OnlineMembers) {
		ch <- Event{
			Type:    EventTypeGroupMemberOnlineStatusChanged,
			Payload: payload,
		}
	}
}

func (b *broadcasterImpl) NewGroupMemberLeftEvent(payload *events.GroupEventGroupMemberLeftPayload) {
	for _, ch := range b.channelsForGUIDs(payload.OnlineMembers) {
		ch <- Event{
			Type:    EventTypeGroupMemberLeft,
			Payload: payload,
		}
	}
}

func (b *broadcasterImpl) NewGroupDisbandEvent(payload *events.GroupEventGroupDisbandPayload) {
	for _, ch := range b.channelsForGUIDs(payload.OnlineMembers) {
		ch <- Event{
			Type:    EventTypeGroupDisband,
			Payload: payload,
		}
	}
}

func (b *broadcasterImpl) NewGroupMemberAddedEvent(payload *events.GroupEventGroupMemberAddedPayload) {
	for _, ch := range b.channelsForGUIDs(payload.OnlineMembers) {
		ch <- Event{
			Type:    EventTypeGroupMemberAdded,
			Payload: payload,
		}
	}
}

func (b *broadcasterImpl) NewGroupLeaderChangedEvent(payload *events.GroupEventGroupLeaderChangedPayload) {
	for _, ch := range b.channelsForGUIDs(payload.OnlineMembers) {
		ch <- Event{
			Type:    EventTypeGroupLeaderChanged,
			Payload: payload,
		}
	}
}

func (b *broadcasterImpl) NewGroupLootTypeChangedEvent(payload *events.GroupEventGroupLootTypeChangedPayload) {
	for _, ch := range b.channelsForGUIDs(payload.OnlineMembers) {
		ch <- Event{
			Type:    EventTypeGroupLootTypeChanged,
			Payload: payload,
		}
	}
}

func (b *broadcasterImpl) NewGroupConvertedToRaidEvent(payload *events.GroupEventGroupConvertedToRaidPayload) {
	for _, ch := range b.channelsForGUIDs(payload.OnlineMembers) {
		ch <- Event{
			Type:    EventTypeGroupConvertedToRaid,
			Payload: payload,
		}
	}
}

func (b *broadcasterImpl) NewGroupMessageEvent(payload *events.GroupEventNewMessagePayload) {
	for _, ch := range b.channelsForGUIDs(payload.Receivers) {
		ch <- Event{
			Type:    EventTypeGroupNewMessage,
			Payload: payload,
		}
	}
}

func (b *broadcasterImpl) NewGroupTargetIconEvent(payload *events.GroupEventNewTargetIconPayload) {
	for _, ch := range b.channelsForGUIDs(payload.Receivers) {
		ch <- Event{
			Type:    EventTypeGroupNewTargetIcon,
			Payload: payload,
		}
	}
}

func (b *broadcasterImpl) NewGroupDifficultyChangedEvent(payload *events.GroupEventGroupDifficultyChangedPayload) {
	for _, ch := range b.channelsForGUIDs(payload.Receivers) {
		ch <- Event{
			Type:    EventTypeGroupDifficultyChanged,
			Payload: payload,
		}
	}
}

func (b *broadcasterImpl) NewMatchmakingJoinedPVPQueueEvent(payload *events.MatchmakingEventPlayersQueuedPayload) {
	if payload.RealmID != gameloadbalancer.RealmID {
		return
	}

	for _, ch := range b.channelsForGUIDs(convertLowGUIDsToUint64(payload.PlayersGUID)) {
		ch <- Event{
			Type:    EventTypeMMJoinedPVPQueue,
			Payload: payload,
		}
	}
}

func (b *broadcasterImpl) NewMatchmakingInvitedToBGOrArenaEvent(payload *events.MatchmakingEventPlayersInvitedPayload) {
	if payload.RealmID != gameloadbalancer.RealmID {
		return
	}

	for _, ch := range b.channelsForGUIDs(convertLowGUIDsToUint64(payload.PlayersGUID)) {
		ch <- Event{
			Type:    EventTypeMMInvitedToBGOrArena,
			Payload: payload,
		}
	}
}

func (b *broadcasterImpl) NewMatchmakingInviteToBGOrArenaExpiredEvent(payload *events.MatchmakingEventPlayersInviteExpiredPayload) {
	if payload.RealmID != gameloadbalancer.RealmID {
		return
	}

	for _, ch := range b.channelsForGUIDs(convertLowGUIDsToUint64(payload.PlayersGUID)) {
		ch <- Event{
			Type:    EventTypeMMInviteToBGOrArenaExpired,
			Payload: payload,
		}
	}
}

func (b *broadcasterImpl) channelsForGUIDs(guids []uint64) []chan Event {
	channels := make([]chan Event, 0, len(guids))
	b.channelsMu.RLock()
	for _, guid := range guids {
		ch, ok := b.channels[guid]
		if !ok {
			continue
		}
		channels = append(channels, ch)
	}
	b.channelsMu.RUnlock()

	return channels
}

func convertLowGUIDsToUint64(s []guid.LowType) []uint64 {
	r := make([]uint64, len(s))
	for i, lowType := range s {
		r[i] = uint64(lowType)
	}
	return r
}
