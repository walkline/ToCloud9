package events

import "fmt"

// LoadBalancerEvent is event type that game load balancer generates
type LoadBalancerEvent int

const (
	// LBEventCharacterLoggedIn is event that occurs when character logs in with CMsgPlayerLogin opcode
	LBEventCharacterLoggedIn LoadBalancerEvent = iota + 1

	// LBEventCharacterLoggedOut is event that occurs when character logs out by any reason (regular logout or tcp connection closes)
	LBEventCharacterLoggedOut
)

// SubjectName is key that nats uses
func (e LoadBalancerEvent) SubjectName() string {
	switch e {
	case LBEventCharacterLoggedIn:
		return "lb.char.logged-in"
	case LBEventCharacterLoggedOut:
		return "lb.char.logged-out"
	}
	panic(fmt.Errorf("unk event %d", e))
}

// LBEventCharacterLoggedInPayload represents payload of LBEventCharacterLoggedIn event
type LBEventCharacterLoggedInPayload struct {
	RealmID        uint32
	LoadBalancerID string
	CharGUID       uint64
	CharName       string
	CharRace       uint8
	CharClass      uint8
	CharGender     uint8
	CharLevel      uint8
	CharZone       uint32
	CharMap        uint32
	CharPosX       float32
	CharPosY       float32
	CharPosZ       float32
	CharGuildID    uint32
	AccountID      uint32
}

// LBEventCharacterLoggedOutPayload represents payload of LBEventCharacterLoggedOut event
type LBEventCharacterLoggedOutPayload struct {
	RealmID        uint32
	LoadBalancerID string
	CharGUID       uint64
	CharName       string
	CharGuildID    uint32
	AccountID      uint32
}
