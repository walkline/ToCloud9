package sender

import (
	"github.com/nats-io/nats.go"

	"github.com/walkline/ToCloud9/shared/events"
)

type Character struct {
	RealmID uint32
	GUID    uint64
	Race    uint8
	Name    string
}

type MsgSender interface {
	SendWhisper(sender *Character, receiver *Character, language uint32, msg string) error
}

type msgSenderNatsJSON struct {
	producer events.ChatServiceProducer
}

func NewMsgSenderNatsJSON(conn *nats.Conn, loadBalancerID string) MsgSender {
	return &msgSenderNatsJSON{
		producer: events.NewChatServiceProducerNatsJSON(conn, "0.0.1", loadBalancerID),
	}
}

func (m msgSenderNatsJSON) SendWhisper(sender *Character, receiver *Character, language uint32, msg string) error {
	return m.producer.IncomingWhisper(&events.ChatEventIncomingWhisperPayload{
		SenderGUID:   sender.GUID,
		SenderName:   sender.Name,
		SenderRace:   sender.Race,
		ReceiverGUID: receiver.GUID,
		ReceiverName: receiver.Name,
		Language:     language,
		Msg:          msg,
	})
}
