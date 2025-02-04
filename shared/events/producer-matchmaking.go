package events

import (
	"encoding/json"

	"github.com/nats-io/nats.go"
)

//go:generate mockery --name=MatchmakingServiceProducer
type MatchmakingServiceProducer interface {
	JoinedQueue(payload *MatchmakingEventPlayersQueuedPayload) error
	InvitedToBGOrArena(payload *MatchmakingEventPlayersInvitedPayload) error
	InviteExpired(payload *MatchmakingEventPlayersInviteExpiredPayload) error
}

type matchmakingServiceProducerNatsJSON struct {
	conn *nats.Conn
	ver  string
}

func NewMatchmakingServiceProducerNatsJSON(conn *nats.Conn, ver string) MatchmakingServiceProducer {
	return &matchmakingServiceProducerNatsJSON{
		conn: conn,
		ver:  ver,
	}
}

func (c *matchmakingServiceProducerNatsJSON) JoinedQueue(payload *MatchmakingEventPlayersQueuedPayload) error {
	return c.publish(MatchmakingEventPlayersQueued, payload)
}

func (c *matchmakingServiceProducerNatsJSON) InvitedToBGOrArena(payload *MatchmakingEventPlayersInvitedPayload) error {
	return c.publish(MatchmakingEventPlayersInvited, payload)
}

func (c *matchmakingServiceProducerNatsJSON) InviteExpired(payload *MatchmakingEventPlayersInviteExpiredPayload) error {
	return c.publish(MatchmakingEventPlayersInviteExpired, payload)
}

func (c *matchmakingServiceProducerNatsJSON) publish(e MatchmakingServiceEvent, payload interface{}) error {
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
