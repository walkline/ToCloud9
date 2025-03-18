package events

import "fmt"

// GatewayEvent is event type that gateway generates.
type GatewayEvent int

const (
	// GWEventCharacterLoggedIn is event that occurs when character logs in with CMsgPlayerLogin opcode.
	GWEventCharacterLoggedIn GatewayEvent = iota + 1

	// GWEventCharacterLoggedOut is event that occurs when character logs out by any reason (regular logout or tcp connection closes).
	GWEventCharacterLoggedOut

	// GWEventCharactersUpdates pack of characters update that occurs every N seconds.
	GWEventCharactersUpdates
)

// SubjectName is key that nats uses.
func (e GatewayEvent) SubjectName() string {
	switch e {
	case GWEventCharacterLoggedIn:
		return "gw.char.logged-in"
	case GWEventCharacterLoggedOut:
		return "gw.char.logged-out"
	case GWEventCharactersUpdates:
		return "gw.char.chars-updates"
	}
	panic(fmt.Errorf("unk event %d", e))
}

// GWEventCharacterLoggedInPayload represents payload of GWEventCharacterLoggedIn event.
type GWEventCharacterLoggedInPayload struct {
	RealmID     uint32
	GatewayID   string
	CharGUID    uint64
	CharName    string
	CharRace    uint8
	CharClass   uint8
	CharGender  uint8
	CharLevel   uint8
	CharZone    uint32
	CharMap     uint32
	CharPosX    float32
	CharPosY    float32
	CharPosZ    float32
	CharGuildID uint32
	AccountID   uint32
}

// GWEventCharacterLoggedOutPayload represents payload of GWEventCharacterLoggedOut event.
type GWEventCharacterLoggedOutPayload struct {
	RealmID     uint32
	GatewayID   string
	CharGUID    uint64
	CharName    string
	CharGuildID uint32
	AccountID   uint32
}

type GWEventCharactersUpdatesPayload struct {
	RealmID   uint32
	GatewayID string
	Updates   []*CharacterUpdate
}

// CharacterUpdate represents new values of fields for the character.
type CharacterUpdate struct {
	ID   uint64  `json:"i"`
	Lvl  *uint8  `json:"l,omitempty"`
	Map  *uint32 `json:"m,omitempty"`
	Area *uint32 `json:"a,omitempty"`
	Zone *uint32 `json:"z,omitempty"`
}
