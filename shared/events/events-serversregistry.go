package events

import "fmt"

// ServerRegistryEvent is event type that servers registry service generates.
type ServerRegistryEvent int

const (
	// ServerRegistryEventLBAdded is event that occurs when server registry registers gateway.
	ServerRegistryEventLBAdded ServerRegistryEvent = iota + 1

	// ServerRegistryEventLBRemovedUnhealthy is event that occurs when gateway unhealthy, and it's been removed from registry.
	ServerRegistryEventLBRemovedUnhealthy

	// ServerRegistryEventGSMapsReassigned is event that occurs when servers registry reassigned maps to game servers.
	ServerRegistryEventGSMapsReassigned

	// ServerRegistryEventGSAdded is event that occurs when server registry registers game server.
	ServerRegistryEventGSAdded

	// ServerRegistryEventGSRemoved is event that occurs when server registry removes game server (unhealthy or shutdown).
	ServerRegistryEventGSRemoved
)

// SubjectName is key that nats uses.
func (e ServerRegistryEvent) SubjectName() string {
	switch e {
	case ServerRegistryEventLBAdded:
		return "sr.lb.added"
	case ServerRegistryEventLBRemovedUnhealthy:
		return "sr.lb.removed.unhealthy"
	case ServerRegistryEventGSMapsReassigned:
		return "sr.gs.maps.reassigned"
	case ServerRegistryEventGSAdded:
		return "sr.gs.added"
	case ServerRegistryEventGSRemoved:
		return "sr.gs.removed"
	}
	panic(fmt.Errorf("unk event %d", e))
}

// ServerRegistryEventGWAddedPayload represents payload of ServerRegistryEventLBAdded event.
type ServerRegistryEventGWAddedPayload struct {
	ID              string
	Address         string
	HealthCheckAddr string
	RealmID         uint32
}

// ServerRegistryEventGWRemovedUnhealthyPayload represents payload of ServerRegistryEventLBRemovedUnhealthy event.
type ServerRegistryEventGWRemovedUnhealthyPayload struct {
	ID              string
	Address         string
	HealthCheckAddr string
	RealmID         uint32
}

type GameServer struct {
	ID           string
	Address      string
	RealmID      uint32
	IsCrossRealm bool

	AvailableMaps []uint32

	OldAssignedMapsToHandle []uint32
	NewAssignedMapsToHandle []uint32
}

func (s GameServer) OnlyNewMaps() []uint32 {
	res := []uint32{}
	for i := range s.NewAssignedMapsToHandle {
		found := false
		for j := range s.OldAssignedMapsToHandle {
			if s.NewAssignedMapsToHandle[i] == s.OldAssignedMapsToHandle[j] {
				found = true
				break
			}
		}
		if !found {
			res = append(res, s.NewAssignedMapsToHandle[i])
		}
	}
	return res
}

func (s GameServer) OnlyRemovedMaps() []uint32 {
	res := []uint32{}
	for i := range s.OldAssignedMapsToHandle {
		found := false
		for j := range s.NewAssignedMapsToHandle {
			if s.OldAssignedMapsToHandle[i] == s.NewAssignedMapsToHandle[j] {
				found = true
				break
			}
		}
		if !found {
			res = append(res, s.OldAssignedMapsToHandle[i])
		}
	}
	return res
}

type ServerRegistryEventGSMapsReassignedPayload struct {
	Servers []GameServer
}

type ServerRegistryEventGSAddedPayload struct {
	GameServer GameServer
}

type ServerRegistryEventGSRemovedPayload struct {
	GameServer GameServer
}
