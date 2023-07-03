package events

import (
	"encoding/json"

	"github.com/nats-io/nats.go"
)

type MailServiceProducer interface {
	IncomingMail(payload *MailEventIncomingMailPayload) error
}

type mailServiceProducerNatsJSON struct {
	conn *nats.Conn
	ver  string
}

func NewMailServiceProducerNatsJSON(conn *nats.Conn, ver string) MailServiceProducer {
	return &mailServiceProducerNatsJSON{
		conn: conn,
		ver:  ver,
	}
}

func (c *mailServiceProducerNatsJSON) IncomingMail(payload *MailEventIncomingMailPayload) error {
	return c.publish(MailEventIncomingMail, payload)
}

func (c *mailServiceProducerNatsJSON) publish(e MailServiceEvent, payload interface{}) error {
	msg := EventToSendGenericPayload{
		Version:   c.ver,
		EventType: int(e),
		Payload:   payload,
	}

	d, err := json.Marshal(&msg)
	if err != nil {
		return err
	}

	return c.conn.Publish(e.SubjectName(), d)
}
