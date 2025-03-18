package repo

import (
	"context"

	"github.com/walkline/ToCloud9/shared/events"
)

type Character struct {
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

type CharactersOnline interface {
	Add(context.Context, *Character) error
	Remove(ctx context.Context, realmID uint32, guid uint64) error
	RemoveAllWithGatewayID(ctx context.Context, realmID uint32, gatewayID string) ([]uint64, error)
	OneByRealmAndGUID(ctx context.Context, realmID uint32, guid uint64) (*Character, error)
	OneByRealmAndName(ctx context.Context, realmID uint32, name string) (*Character, error)

	CharactersByRealmAndGUIDs(ctx context.Context, realmID uint32, guids []uint64) ([]Character, error)

	// GWCharacterLoggedInHandler updates cache with player logged in.
	events.GWCharacterLoggedInHandler
	// GWCharacterLoggedOutHandler updates cache with player logged out.
	events.GWCharacterLoggedOutHandler
	// GWCharactersUpdatesHandler updates cache with pack of characters updates.
	events.GWCharactersUpdatesHandler

	WhoHandler
}

// CharactersWhoQuery represents params to handle SMsgWho packet.
type CharactersWhoQuery struct {
	LvlMin    uint8
	LvlMax    uint8
	ClassMask uint32
	RaceMask  uint32
	Zones     []uint32
	Strings   []string
}

// WhoHandler represents handler for SMsgWho packet.
type WhoHandler interface {
	WhoRequest(ctx context.Context, requesterRealmID uint32, requesterGUID uint64, query CharactersWhoQuery) ([]Character, error)
}
