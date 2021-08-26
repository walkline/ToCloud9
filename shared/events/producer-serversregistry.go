package events

import (
	"encoding/json"
	"github.com/nats-io/nats.go"
)

type ServerRegistryProducer interface {
	LBAdded(payload *ServerRegistryEventLBAddedPayload) error
	LBRemovedUnhealthy(payload *ServerRegistryEventLBRemovedUnhealthyPayload) error
}

type serverRegistryProducerNatsJSON struct {
	conn *nats.Conn
	ver  string
}

func NewServerRegistryProducerNatsJSON(conn *nats.Conn, ver string) ServerRegistryProducer {
	return &serverRegistryProducerNatsJSON{
		conn: conn,
		ver:  ver,
	}
}

func (s serverRegistryProducerNatsJSON) LBAdded(payload *ServerRegistryEventLBAddedPayload) error {
	return s.publish(ServerRegistryEventLBAdded, payload)
}

func (s serverRegistryProducerNatsJSON) LBRemovedUnhealthy(payload *ServerRegistryEventLBRemovedUnhealthyPayload) error {
	return s.publish(ServerRegistryEventLBRemovedUnhealthy, payload)
}

func (s *serverRegistryProducerNatsJSON) publish(e ServerRegistryEvent, payload interface{}) error {
	msg := EventToSendGenericPayload{
		Version:   s.ver,
		EventType: int(e),
		Payload:   payload,
	}

	d, err := json.Marshal(&msg)
	if err != nil {
		return err
	}

	return s.conn.Publish(e.SubjectName(), d)
}
