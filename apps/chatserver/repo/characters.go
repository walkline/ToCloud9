package repo

import (
	"context"

	"github.com/walkline/ToCloud9/apps/chatserver/sender"
)

type Character struct {
	RealmID   uint32
	GatewayID string
	GUID      uint64
	Name      string
	Race      uint8

	MsgSender sender.MsgSender
}

type CharactersRepo interface {
	AddCharacter(ctx context.Context, character *Character) error
	RemoveCharacter(ctx context.Context, realmID uint32, characterGUID uint64) error

	RemoveCharactersWithRealm(ctx context.Context, realmID uint32) error

	CharacterByRealmAndGUID(ctx context.Context, realmID uint32, characterGUID uint64) (*Character, error)
	CharacterByRealmAndName(ctx context.Context, realmID uint32, name string) (*Character, error)
}
