package repo

import (
	"context"

	"github.com/walkline/ToCloud9/apps/matchmakingserver/battleground"
)

type RealmWithBattlegroupKey struct {
	RealmID       uint32
	BattlegroupID uint32
}

type BattlegroundRepo interface {
	SaveBattleground(ctx context.Context, battleground *battleground.Battleground) error

	UpdateBattleground(ctx context.Context, instanceID uint32, realmIDAndBattlegroup RealmWithBattlegroupKey, update func(*battleground.Battleground) error) error

	// GetBattlegroundByInstanceID returns battleground by instance id. If realmID is 0 then returns cross realm version.
	GetBattlegroundByInstanceID(ctx context.Context, instanceID uint32, realmIDAndBattlegroup RealmWithBattlegroupKey) (*battleground.Battleground, error)

	GetActiveBattlegrounds(ctx context.Context, bgType battleground.QueueTypeID, bracket uint8, realmIDAndBattlegroup RealmWithBattlegroupKey) ([]battleground.Battleground, error)

	GetAllActiveBattlegrounds(ctx context.Context) ([]battleground.Battleground, error)

	DeleteAllWithGameServerAddress(ctx context.Context, address string) ([]battleground.Battleground, error)
}
