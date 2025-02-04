package events

import (
	"encoding/json"

	"github.com/nats-io/nats.go"
)

type CharactersServiceProducer interface {
	CharsDisconnectedUnhealthyLB(payload *CharEventCharsDisconnectedUnhealthyLBPayload) error
}

type charactersServiceProducerNatsJSON struct {
	conn           *nats.Conn
	ver            string
	loadBalancerID string
}

func NewCharactersServiceProducerNatsJSON(conn *nats.Conn, ver string) CharactersServiceProducer {
	return &charactersServiceProducerNatsJSON{
		conn: conn,
		ver:  ver,
	}
}

func (c *charactersServiceProducerNatsJSON) CharsDisconnectedUnhealthyLB(payload *CharEventCharsDisconnectedUnhealthyLBPayload) error {
	return c.publish(CharEventCharsDisconnectedUnhealthyLB, payload)
}

func (c *charactersServiceProducerNatsJSON) publish(e CharactersServiceEvent, payload interface{}) error {
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
