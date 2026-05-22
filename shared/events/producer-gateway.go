package events

import (
	"encoding/json"
	"time"

	"github.com/nats-io/nats.go"
)

//go:generate mockery --name=GatewayProducer
type GatewayProducer interface {
	GatewayStarted(payload *GWEventGatewayStartedPayload) error
	CharacterLoggedIn(payload *GWEventCharacterLoggedInPayload) error
	CharacterLoggedOut(payload *GWEventCharacterLoggedOutPayload) error
	CharactersUpdates(payload *GWEventCharactersUpdatesPayload) error
	GuildCreated(payload *GWEventGuildCreatedPayload) error
}

type gatewayProducerNatsJSON struct {
	RealmID uint32
	ID      string

	conn *nats.Conn
	ver  string
}

func NewGatewayProducerNatsJSON(conn *nats.Conn, ver string, realmID uint32, gatewayID string) GatewayProducer {
	return &gatewayProducerNatsJSON{
		conn:    conn,
		ver:     ver,
		RealmID: realmID,
		ID:      gatewayID,
	}
}

func (p *gatewayProducerNatsJSON) GatewayStarted(payload *GWEventGatewayStartedPayload) error {
	payload.RealmID = p.RealmID
	payload.GatewayID = p.ID
	if payload.StartedAtMs == 0 {
		payload.StartedAtMs = uint64(time.Now().UnixMilli())
	}
	return p.publish(GWEventGatewayStarted, payload)
}

func (p *gatewayProducerNatsJSON) CharacterLoggedIn(payload *GWEventCharacterLoggedInPayload) error {
	payload.RealmID = p.RealmID
	payload.GatewayID = p.ID
	if payload.EventTimeUnixNano == 0 {
		payload.EventTimeUnixNano = uint64(time.Now().UnixNano())
	}
	return p.publish(GWEventCharacterLoggedIn, payload)
}

func (p *gatewayProducerNatsJSON) CharacterLoggedOut(payload *GWEventCharacterLoggedOutPayload) error {
	payload.RealmID = p.RealmID
	payload.GatewayID = p.ID
	if payload.EventTimeUnixNano == 0 {
		payload.EventTimeUnixNano = uint64(time.Now().UnixNano())
	}
	return p.publish(GWEventCharacterLoggedOut, payload)
}

func (p *gatewayProducerNatsJSON) CharactersUpdates(payload *GWEventCharactersUpdatesPayload) error {
	payload.RealmID = p.RealmID
	payload.GatewayID = p.ID
	if payload.EventTimeUnixNano == 0 {
		payload.EventTimeUnixNano = uint64(time.Now().UnixNano())
	}
	return p.publish(GWEventCharactersUpdates, payload)
}

func (p *gatewayProducerNatsJSON) GuildCreated(payload *GWEventGuildCreatedPayload) error {
	payload.RealmID = p.RealmID
	payload.GatewayID = p.ID
	return p.publish(GWEventGuildCreated, payload)
}

func (p *gatewayProducerNatsJSON) publish(e GatewayEvent, payload interface{}) error {
	msg := EventToSendGenericPayload{
		Version:   p.ver,
		EventType: int(e),
		Payload:   payload,
	}

	d, err := json.Marshal(&msg)
	if err != nil {
		return err
	}

	return p.conn.Publish(e.SubjectName(), d)
}
