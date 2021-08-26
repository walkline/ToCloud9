package repo

import "context"

type Realm struct {
	ID                   uint32
	Name                 string
	Icon                 uint8
	Flag                 uint8
	Timezone             uint8
	AllowedSecurityLevel uint8
	GameBuild            uint32
}

type CharsCountInRealm struct {
	RealmID   uint32
	CharCount uint8
}

type RealmRepo interface {
	LoadRealms(ctx context.Context) ([]Realm, error)
	CountCharsPerRealmByAccountID(ctx context.Context, accountID uint32) ([]CharsCountInRealm, error)
}
