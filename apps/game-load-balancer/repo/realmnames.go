package repo

import "context"

type RealmName struct {
	RealmID uint32
	Name    string
}

type RealmNamesRepo interface {
	LoadRealmNames(context.Context) ([]*RealmName, error)
}
