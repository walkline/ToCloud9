package events

import (
	"encoding/json"

	"github.com/nats-io/nats.go"
)

//go:generate mockery --name=PetitionServiceProducer
type PetitionServiceProducer interface {
	SignResult(payload *PetitionEventSignResultPayload) error
	Offered(payload *PetitionEventOfferedPayload) error
	Declined(payload *PetitionEventDeclinedPayload) error
}

type petitionServiceProducerNatsJSON struct {
	conn *nats.Conn
	ver  string
}

func NewPetitionServiceProducerNatsJSON(conn *nats.Conn, ver string) PetitionServiceProducer {
	return &petitionServiceProducerNatsJSON{conn: conn, ver: ver}
}

func (p *petitionServiceProducerNatsJSON) SignResult(payload *PetitionEventSignResultPayload) error {
	return p.publish(PetitionEventSignResult, payload)
}

func (p *petitionServiceProducerNatsJSON) Offered(payload *PetitionEventOfferedPayload) error {
	return p.publish(PetitionEventOffered, payload)
}

func (p *petitionServiceProducerNatsJSON) Declined(payload *PetitionEventDeclinedPayload) error {
	return p.publish(PetitionEventDeclined, payload)
}

func (p *petitionServiceProducerNatsJSON) publish(e PetitionServiceEvent, payload interface{}) error {
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

// NoopPetitionServiceProducer is used in unit tests.
type NoopPetitionServiceProducer struct{}

func (NoopPetitionServiceProducer) SignResult(*PetitionEventSignResultPayload) error { return nil }
func (NoopPetitionServiceProducer) Offered(*PetitionEventOfferedPayload) error       { return nil }
func (NoopPetitionServiceProducer) Declined(*PetitionEventDeclinedPayload) error     { return nil }
