package events

import (
	"encoding/json"

	"github.com/nats-io/nats.go"
)

//go:generate mockery --name=ServerRegistryProducer
type ServerRegistryProducer interface {
	GatewayAdded(payload *ServerRegistryEventGWAddedPayload) error
	GatewayRemovedUnhealthy(payload *ServerRegistryEventGWRemovedUnhealthyPayload) error
	GSMapsReassigned(payload *ServerRegistryEventGSMapsReassignedPayload) error
	GSAdded(payload *ServerRegistryEventGSAddedPayload) error
	GSRemoved(payload *ServerRegistryEventGSRemovedPayload) error
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

func (s serverRegistryProducerNatsJSON) GatewayAdded(payload *ServerRegistryEventGWAddedPayload) error {
	return s.publish(ServerRegistryEventGWAdded, payload)
}

func (s serverRegistryProducerNatsJSON) GatewayRemovedUnhealthy(payload *ServerRegistryEventGWRemovedUnhealthyPayload) error {
	return s.publish(ServerRegistryEventGWRemovedUnhealthy, payload)
}

func (s serverRegistryProducerNatsJSON) GSMapsReassigned(payload *ServerRegistryEventGSMapsReassignedPayload) error {
	return s.publish(ServerRegistryEventGSMapsReassigned, payload)
}

func (s serverRegistryProducerNatsJSON) GSAdded(payload *ServerRegistryEventGSAddedPayload) error {
	return s.publish(ServerRegistryEventGSAdded, payload)
}

func (s serverRegistryProducerNatsJSON) GSRemoved(payload *ServerRegistryEventGSRemovedPayload) error {
	return s.publish(ServerRegistryEventGSRemoved, payload)
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
