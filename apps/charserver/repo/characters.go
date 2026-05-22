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
	ExtraFlags   uint32
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

const (
	SocialFlagFriend uint8 = 0x01
	SocialFlagIgnore uint8 = 0x02
)

type FriendEntry struct {
	PlayerRealmID uint32
	PlayerGUID    uint64
	FriendGUID    uint64
	Flags         uint8
	Note          string
}

type LfgDungeonRoute struct {
	RealmID               uint32
	PlayerGUID            uint64
	DungeonEntry          uint32
	MapID                 uint32
	Difficulty            uint8
	OwnerRealmID          uint32
	IsCrossRealm          bool
	RequiresBoundInstance bool
	InstanceID            uint32
	BoundInstanceID       uint32
}

type Characters interface {
	ListCharactersToLogIn(ctx context.Context, realmID, accountID uint32) ([]LogInCharacter, error)
	CharacterToLogInByGUID(ctx context.Context, realmID, accountID uint32, charGUID uint64) (*LogInCharacter, error)
	CharacterByName(ctx context.Context, realmID uint32, name string) (*Character, error)
	CharacterByGUID(ctx context.Context, realmID uint32, charGUID uint64) (*Character, error)
	DisplayCharacterByAccount(ctx context.Context, accountID uint32) (*Character, error)
	AccountDataForAccountID(ctx context.Context, realmID, accountID uint32) ([]AccountData, error)
	UpdateAccountDataForAccountID(ctx context.Context, realmID, accountID uint32, data AccountData) error
	SaveCharacterPosition(ctx context.Context, realmID uint32, charGUID uint64, mapID uint32, x, y, z, o float32) error
	RecordLfgDungeonRoute(ctx context.Context, route LfgDungeonRoute) error
	ConfirmLfgDungeonRouteEntered(ctx context.Context, realmID uint32, playerGUID uint64, mapID uint32, difficulty uint8, instanceID uint32) (*LfgDungeonRoute, error)
	ClearUnboundLfgDungeonRoute(ctx context.Context, realmID uint32, playerGUID uint64, mapID uint32) error
	LfgDungeonRouteForPlayer(ctx context.Context, realmID uint32, playerGUID uint64, mapID uint32) (*LfgDungeonRoute, error)

	// Friends and social methods
	GetFriendsForPlayer(ctx context.Context, realmID uint32, playerGUID uint64) ([]*FriendEntry, error)
	AddFriend(ctx context.Context, realmID uint32, playerGUID, friendGUID uint64, note string) error
	RemoveFriend(ctx context.Context, realmID uint32, playerGUID, friendGUID uint64) error
	UpdateFriendNote(ctx context.Context, realmID uint32, playerGUID, friendGUID uint64, note string) error
	AddIgnore(ctx context.Context, realmID uint32, playerGUID, ignoredGUID uint64) error
	RemoveIgnore(ctx context.Context, realmID uint32, playerGUID, ignoredGUID uint64) error
	GetPlayersWhoHaveAsFriend(ctx context.Context, realmID uint32, playerGUID uint64) ([]uint64, error)
}
