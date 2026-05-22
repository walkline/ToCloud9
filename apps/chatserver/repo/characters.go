package repo

import (
	"context"

	"github.com/walkline/ToCloud9/apps/chatserver/sender"
)

type Character struct {
	RealmID   uint32
	GatewayID string
	GUID      uint64
	AccountID uint32
	Name      string
	Race      uint8
	Class     uint8
	Gender    uint8

	MsgSender sender.MsgSender
}

type CharactersRepo interface {
	AddCharacter(ctx context.Context, character *Character) error
	AddCharacterFromGatewayEvent(ctx context.Context, character *Character, eventTimeUnixNano uint64) (bool, error)
	RemoveCharacter(ctx context.Context, realmID uint32, characterGUID uint64) error
	RemoveCharacterFromGatewayEvent(ctx context.Context, realmID uint32, characterGUID uint64, eventTimeUnixNano uint64) (bool, error)

	RemoveCharactersWithGatewayID(ctx context.Context, realmID uint32, gatewayID string, eventTimeUnixNano uint64) error
	RemoveCharactersWithRealm(ctx context.Context, realmID uint32) error

	CharacterByRealmAndGUID(ctx context.Context, realmID uint32, characterGUID uint64) (*Character, error)
	CharacterByRealmAndName(ctx context.Context, realmID uint32, name string) (*Character, error)
	CharactersByName(ctx context.Context, name string) ([]*Character, error)
}
