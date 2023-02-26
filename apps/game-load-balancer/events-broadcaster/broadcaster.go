package events_broadcaster

import (
	"sync"

	"github.com/walkline/ToCloud9/shared/events"
)

type EventType int

const (
	EventTypeIncomingWhisper EventType = iota + 1
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
