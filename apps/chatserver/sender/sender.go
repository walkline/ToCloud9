package sender

import (
	"github.com/nats-io/nats.go"

	"github.com/walkline/ToCloud9/apps/chatserver"
	"github.com/walkline/ToCloud9/shared/events"
)

type Character struct {
	RealmID uint32
	GUID    uint64
	Race    uint8
	Class   uint8
	Gender  uint8
	Name    string
	ChatTag uint8
}

type MsgSender interface {
	SendWhisper(sender *Character, receiver *Character, language uint32, msg string) error
}

type MsgProducer interface {
	ProduceChannelMessage(payload *events.ChatEventChannelMessagePayload) error
	ProduceChannelJoined(payload *events.ChatEventChannelJoinedPayload) error
	ProduceChannelLeft(payload *events.ChatEventChannelLeftPayload) error
	ProduceChannelNotification(payload *events.ChatEventChannelNotificationPayload) error
}

type msgSenderNatsJSON struct {
	producer events.ChatServiceProducer
}

func NewMsgSenderNatsJSON(conn *nats.Conn, gatewayID string) MsgSender {
	return &msgSenderNatsJSON{
		producer: events.NewChatServiceProducerNatsJSON(conn, chatserver.Ver, gatewayID),
	}
}

func (m msgSenderNatsJSON) SendWhisper(sender *Character, receiver *Character, language uint32, msg string) error {
	return m.producer.IncomingWhisper(&events.ChatEventIncomingWhisperPayload{
		SenderRealmID:   sender.RealmID,
		SenderGUID:      sender.GUID,
		SenderName:      sender.Name,
		SenderRace:      sender.Race,
		SenderClass:     sender.Class,
		SenderGender:    sender.Gender,
		SenderChatTag:   sender.ChatTag,
		ReceiverRealmID: receiver.RealmID,
		ReceiverGUID:    receiver.GUID,
		ReceiverName:    receiver.Name,
		Language:        language,
		Msg:             msg,
	})
}

type msgProducerNatsJSON struct {
	producer events.ChatServiceProducer
}

func NewMsgProducerNatsJSON(conn *nats.Conn, gatewayID string) MsgProducer {
	return &msgProducerNatsJSON{
		producer: events.NewChatServiceProducerNatsJSON(conn, chatserver.Ver, gatewayID),
	}
}

func (m *msgProducerNatsJSON) ProduceChannelMessage(payload *events.ChatEventChannelMessagePayload) error {
	return m.producer.ChannelMessage(payload)
}

func (m *msgProducerNatsJSON) ProduceChannelJoined(payload *events.ChatEventChannelJoinedPayload) error {
	return m.producer.ChannelJoined(payload)
}

func (m *msgProducerNatsJSON) ProduceChannelLeft(payload *events.ChatEventChannelLeftPayload) error {
	return m.producer.ChannelLeft(payload)
}

func (m *msgProducerNatsJSON) ProduceChannelNotification(payload *events.ChatEventChannelNotificationPayload) error {
	return m.producer.ChannelNotification(payload)
}
