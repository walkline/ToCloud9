package events

import "fmt"

// CharactersServiceEvent is event type that characters service generates
type CharactersServiceEvent int

const (
	// CharEventCharsDisconnectedUnhealthyGW event that contains players that were connected to unhealthy gateway
	CharEventCharsDisconnectedUnhealthyGW CharactersServiceEvent = iota + 1
)

// SubjectName is key that nats uses
func (e CharactersServiceEvent) SubjectName() string {
	switch e {
	case CharEventCharsDisconnectedUnhealthyGW:
		return "char.chars.unhealthy.gw"
	}
	panic(fmt.Errorf("unk event %d", e))
}

// CharEventCharsDisconnectedUnhealthyGWPayload represents payload of CharEventCharsDisconnectedUnhealthyGW event
type CharEventCharsDisconnectedUnhealthyGWPayload struct {
	RealmID        uint32
	GatewayID      string
	CharactersGUID []uint64
}
