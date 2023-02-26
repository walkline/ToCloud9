package events

import "fmt"

// ServerRegistryEvent is event type that servers registry service generates.
type ServerRegistryEvent int

const (
	// ServerRegistryEventLBAdded is event that occurs when server registry registers game load balancer.
	ServerRegistryEventLBAdded ServerRegistryEvent = iota + 1

	// ServerRegistryEventLBRemovedUnhealthy is event that occurs when game load balancer unhealthy and it's been removed from registry.
	ServerRegistryEventLBRemovedUnhealthy
)

// SubjectName is key that nats uses.
func (e ServerRegistryEvent) SubjectName() string {
	switch e {
	case ServerRegistryEventLBAdded:
		return "sr.lb.added"
	case ServerRegistryEventLBRemovedUnhealthy:
		return "sr.lb.removed.unhealthy"
	}
	panic(fmt.Errorf("unk event %d", e))
}

// ServerRegistryEventLBAddedPayload represents payload of ServerRegistryEventLBAdded event.
type ServerRegistryEventLBAddedPayload struct {
	ID              string
	Address         string
	HealthCheckAddr string
	RealmID         uint32
}

// ServerRegistryEventLBRemovedUnhealthyPayload represents payload of ServerRegistryEventLBRemovedUnhealthy event.
type ServerRegistryEventLBRemovedUnhealthyPayload struct {
	ID              string
	Address         string
	HealthCheckAddr string
	RealmID         uint32
}
