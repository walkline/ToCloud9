package events

import (
	"encoding/json"

	"github.com/nats-io/nats.go"
	"github.com/rs/zerolog/log"
)

type ChatServiceProducer interface {
	IncomingWhisper(payload *ChatEventIncomingWhisperPayload) error
	ChannelMessage(payload *ChatEventChannelMessagePayload) error
	ChannelJoined(payload *ChatEventChannelJoinedPayload) error
	ChannelLeft(payload *ChatEventChannelLeftPayload) error
	ChannelNotification(payload *ChatEventChannelNotificationPayload) error
}

type chatServiceProducerNatsJSON struct {
	conn      *nats.Conn
	ver       string
	gatewayID string
}

func NewChatServiceProducerNatsJSON(conn *nats.Conn, ver string, gatewayID string) ChatServiceProducer {
	return &chatServiceProducerNatsJSON{
		conn:      conn,
		ver:       ver,
		gatewayID: gatewayID,
	}
}

func (c *chatServiceProducerNatsJSON) IncomingWhisper(payload *ChatEventIncomingWhisperPayload) error {
	return c.publish(ChatEventIncomingWhisper, payload)
}

func (c *chatServiceProducerNatsJSON) ChannelMessage(payload *ChatEventChannelMessagePayload) error {
	log.Debug().
		Str("channelName", payload.ChannelName).
		Uint64("senderGUID", payload.SenderGUID).
		Str("gatewayID", c.gatewayID).
		Str("subject", ChatEventChannelMessage.SubjectName(c.gatewayID)).
		Msg("Publishing channel message to NATS")
	return c.publish(ChatEventChannelMessage, payload)
}

func (c *chatServiceProducerNatsJSON) ChannelJoined(payload *ChatEventChannelJoinedPayload) error {
	return c.publish(ChatEventChannelJoined, payload)
}

func (c *chatServiceProducerNatsJSON) ChannelLeft(payload *ChatEventChannelLeftPayload) error {
	return c.publish(ChatEventChannelLeft, payload)
}

func (c *chatServiceProducerNatsJSON) ChannelNotification(payload *ChatEventChannelNotificationPayload) error {
	return c.publish(ChatEventChannelNotification, payload)
}

func (c *chatServiceProducerNatsJSON) publish(e ChatServiceEvent, payload interface{}) error {
	msg := EventToSendGenericPayload{
		Version:   c.ver,
		EventType: int(e),
		Payload:   payload,
	}

	d, err := json.Marshal(&msg)
	if err != nil {
		return err
	}

	subject := e.SubjectName(c.gatewayID)
	log.Debug().
		Str("subject", subject).
		Int("dataSize", len(d)).
		Msg("Publishing to NATS")

	return c.conn.Publish(subject, d)
}
