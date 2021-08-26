package events_broadcaster

import "sync"

type EventType int

const (
	EventTypeIncomingWhisper EventType = iota + 1
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

type Event struct {
	Type    EventType
	Payload interface{}
}

type Broadcaster interface {
	RegisterCharacter(charGUID uint64) <-chan Event
	UnregisterCharacter(charGUID uint64)

	NewIncomingWhisperEvent(payload *IncomingWhisperPayload)
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
