package events

import (
	"encoding/json"
	"time"

	"github.com/nats-io/nats.go"
)

//go:generate mockery --name=ServerRegistryProducer
type ServerRegistryProducer interface {
	GatewayAdded(payload *ServerRegistryEventGWAddedPayload) error
	GatewayRemovedUnhealthy(payload *ServerRegistryEventGWRemovedUnhealthyPayload) error
	GSMapsReassigned(payload *ServerRegistryEventGSMapsReassignedPayload) error
	GSAdded(payload *ServerRegistryEventGSAddedPayload) error
	GSRemoved(payload *ServerRegistryEventGSRemovedPayload) error
	MatchmakingRemovedUnhealthy(payload *ServerRegistryEventMatchmakingRemovedUnhealthyPayload) error
	MatchmakingRecovered(payload *ServerRegistryEventMatchmakingRecoveredPayload) error
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
	if payload.EventTimeUnixNano == 0 {
		payload.EventTimeUnixNano = uint64(time.Now().UnixNano())
	}
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

func (s serverRegistryProducerNatsJSON) MatchmakingRemovedUnhealthy(payload *ServerRegistryEventMatchmakingRemovedUnhealthyPayload) error {
	return s.publish(ServerRegistryEventMatchmakingRemovedUnhealthy, payload)
}

func (s serverRegistryProducerNatsJSON) MatchmakingRecovered(payload *ServerRegistryEventMatchmakingRecoveredPayload) error {
	return s.publish(ServerRegistryEventMatchmakingRecovered, payload)
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
