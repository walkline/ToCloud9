package events

import (
	"encoding/json"

	"github.com/nats-io/nats.go"
)

//go:generate mockery --name=LoadBalancerProducer
type LoadBalancerProducer interface {
	CharacterLoggedIn(payload *LBEventCharacterLoggedInPayload) error
	CharacterLoggedOut(payload *LBEventCharacterLoggedOutPayload) error
	CharactersUpdates(payload *LBEventCharactersUpdatesPayload) error
}

type loadBalancerProducerNatsJSON struct {
	RealmID uint32
	ID      string

	conn *nats.Conn
	ver  string
}

func NewLoadBalancerProducerNatsJSON(conn *nats.Conn, ver string, realmID uint32, loadBalancerID string) LoadBalancerProducer {
	return &loadBalancerProducerNatsJSON{
		conn:    conn,
		ver:     ver,
		RealmID: realmID,
		ID:      loadBalancerID,
	}
}

func (p *loadBalancerProducerNatsJSON) CharacterLoggedIn(payload *LBEventCharacterLoggedInPayload) error {
	payload.RealmID = p.RealmID
	payload.LoadBalancerID = p.ID
	return p.publish(LBEventCharacterLoggedIn, payload)
}

func (p *loadBalancerProducerNatsJSON) CharacterLoggedOut(payload *LBEventCharacterLoggedOutPayload) error {
	payload.RealmID = p.RealmID
	payload.LoadBalancerID = p.ID
	return p.publish(LBEventCharacterLoggedOut, payload)
}

func (p *loadBalancerProducerNatsJSON) CharactersUpdates(payload *LBEventCharactersUpdatesPayload) error {
	payload.RealmID = p.RealmID
	payload.LoadBalancerID = p.ID
	return p.publish(LBEventCharactersUpdates, payload)
}

func (p *loadBalancerProducerNatsJSON) publish(e LoadBalancerEvent, payload interface{}) error {
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
