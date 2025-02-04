package events

import "fmt"

// CharactersServiceEvent is event type that characters service generates
type CharactersServiceEvent int

const (
	// CharEventCharsDisconnectedUnhealthyLB event that contains players that were connected to unhealthy load balancer
	CharEventCharsDisconnectedUnhealthyLB CharactersServiceEvent = iota + 1
)

// SubjectName is key that nats uses
func (e CharactersServiceEvent) SubjectName() string {
	switch e {
	case CharEventCharsDisconnectedUnhealthyLB:
		return "char.chars.unhealthy.lb"
	}
	panic(fmt.Errorf("unk event %d", e))
}

// CharEventCharsDisconnectedUnhealthyLBPayload represents payload of CharEventCharsDisconnectedUnhealthyLB event
type CharEventCharsDisconnectedUnhealthyLBPayload struct {
	RealmID        uint32
	LoadBalancerID string
	CharactersGUID []uint64
}
