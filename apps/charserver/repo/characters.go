package repo

import (
	"context"
)

type LogInCharacter struct {
	GUID      uint64
	AccountID uint32
	Name      string

	Race        uint8
	Class       uint8
	Gender      uint8
	Skin        uint8
	Face        uint8
	HairStyle   uint8
	HairColor   uint8
	FacialStyle uint8

	Level uint8

	Zone      uint32
	Map       uint32
	PositionX float32
	PositionY float32
	PositionZ float32

	GuildID uint32

	PlayerFlags  uint32
	AtLoginFlags uint16

	PetEntry   uint32
	PetModelID uint32
	PetLevel   uint8

	Equipments []uint32
	Enchants   []uint32

	Banned bool
}

type AccountData struct {
	Type uint8
	Time int64
	Data string
}

type Characters interface {
	ListCharactersToLogIn(ctx context.Context, realmID, accountID uint32) ([]LogInCharacter, error)
	CharacterToLogInByGUID(ctx context.Context, realmID uint32, charGUID uint64) (*LogInCharacter, error)
	CharacterByName(ctx context.Context, realmID uint32, name string) (*Character, error)
	AccountDataForAccountID(ctx context.Context, realmID, accountID uint32) ([]AccountData, error)
	SaveCharacterPosition(ctx context.Context, realmID uint32, charGUID uint64, mapID uint32, x, y, z, o float32) error
}
