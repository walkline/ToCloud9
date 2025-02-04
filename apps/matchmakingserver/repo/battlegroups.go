package repo

import "context"

type BattleGroupsRepository interface {
	BattleGroupIDByRealmID(context context.Context, realmID uint32) (uint32, error)
	AllRealmsInBattleGroups(context context.Context) ([]uint32, error)
	AllBattleGroupsIDs(context context.Context) ([]uint32, error)
}
