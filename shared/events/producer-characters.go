package events

import (
	"encoding/json"
	"time"

	"github.com/nats-io/nats.go"
)

type CharactersServiceProducer interface {
	CharsDisconnectedUnhealthyLB(payload *CharEventCharsDisconnectedUnhealthyGWPayload) error
	ArenaTeamInviteCreated(payload *CharEventArenaTeamInviteCreatedPayload) error
	ArenaTeamNativeEvent(payload *CharEventArenaTeamNativeEventPayload) error
}

type charactersServiceProducerNatsJSON struct {
	conn      *nats.Conn
	ver       string
	gatewayID string
}

func NewCharactersServiceProducerNatsJSON(conn *nats.Conn, ver string) CharactersServiceProducer {
	return &charactersServiceProducerNatsJSON{
		conn: conn,
		ver:  ver,
	}
}

func (c *charactersServiceProducerNatsJSON) CharsDisconnectedUnhealthyLB(payload *CharEventCharsDisconnectedUnhealthyGWPayload) error {
	if payload.EventTimeUnixNano == 0 {
		payload.EventTimeUnixNano = uint64(time.Now().UnixNano())
	}
	return c.publish(CharEventCharsDisconnectedUnhealthyGW, payload)
}

func (c *charactersServiceProducerNatsJSON) ArenaTeamInviteCreated(payload *CharEventArenaTeamInviteCreatedPayload) error {
	return c.publish(CharEventArenaTeamInviteCreated, payload)
}

func (c *charactersServiceProducerNatsJSON) ArenaTeamNativeEvent(payload *CharEventArenaTeamNativeEventPayload) error {
	return c.publish(CharEventArenaTeamNativeEvent, payload)
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
