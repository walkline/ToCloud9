package repo

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/walkline/ToCloud9/apps/matchmakingserver/battleground"
)

var ErrNotFound = errors.New("battleground_in_mem_repo: not found")

type battlegroundInMemRepo struct {
	m                        sync.RWMutex
	instanceIDWithRealmIndex map[string]*battleground.Battleground
}

func NewBattlegroundInMemRepo() BattlegroundRepo {
	return &battlegroundInMemRepo{
		instanceIDWithRealmIndex: make(map[string]*battleground.Battleground),
	}
}

func (b *battlegroundInMemRepo) SaveBattleground(ctx context.Context, bg *battleground.Battleground) error {
	b.m.Lock()
	defer b.m.Unlock()

	if bg.Status == battleground.StatusEnded {
		delete(b.instanceIDWithRealmIndex, b.indexKeyForInstanceAndRealm(bg.InstanceID, RealmWithBattlegroupKey{RealmID: bg.RealmID, BattlegroupID: bg.BattleGroupID}))
		return nil
	}

	b.instanceIDWithRealmIndex[b.indexKeyForInstanceAndRealm(bg.InstanceID, RealmWithBattlegroupKey{RealmID: bg.RealmID, BattlegroupID: bg.BattleGroupID})] = bg
	return nil
}

func (b *battlegroundInMemRepo) GetBattlegroundByInstanceID(ctx context.Context, instanceID uint32, realmIDAndBattlegroup RealmWithBattlegroupKey) (*battleground.Battleground, error) {
	b.m.RLock()
	defer b.m.RUnlock()

	return b.instanceIDWithRealmIndex[b.indexKeyForInstanceAndRealm(instanceID, realmIDAndBattlegroup)], nil
}

func (b *battlegroundInMemRepo) GetActiveBattlegrounds(ctx context.Context, bgType battleground.QueueTypeID, bracket uint8, realmIDAndBattlegroup RealmWithBattlegroupKey) ([]battleground.Battleground, error) {
	b.m.RLock()
	defer b.m.RUnlock()

	result := make([]battleground.Battleground, 0)

	// TODO: probably we need to add index here...
	for _, bg := range b.instanceIDWithRealmIndex {
		if bg.QueueTypeID == bgType && bg.BracketID == bracket &&
			bg.RealmID == realmIDAndBattlegroup.RealmID &&
			bg.BattleGroupID == realmIDAndBattlegroup.BattlegroupID &&
			bg.IsActive() {
			result = append(result, *bg.DeepCopy())
		}
	}

	return result, nil
}

func (b *battlegroundInMemRepo) GetAllActiveBattlegrounds(ctx context.Context) ([]battleground.Battleground, error) {
	b.m.RLock()
	defer b.m.RUnlock()

	result := make([]battleground.Battleground, 0)

	// TODO: probably we need to add index here...
	for _, bg := range b.instanceIDWithRealmIndex {
		if bg.IsActive() {
			result = append(result, *bg.DeepCopy())
		}
	}

	return result, nil
}

func (b *battlegroundInMemRepo) UpdateBattleground(ctx context.Context, instanceID uint32, realmIDAndBattlegroup RealmWithBattlegroupKey, u func(*battleground.Battleground) error) error {
	b.m.Lock()
	defer b.m.Unlock()

	bg := b.instanceIDWithRealmIndex[b.indexKeyForInstanceAndRealm(instanceID, realmIDAndBattlegroup)]
	if bg == nil {
		return ErrNotFound
	}

	if err := u(bg); err != nil {
		return err
	}

	if bg.Status == battleground.StatusEnded {
		delete(b.instanceIDWithRealmIndex, b.indexKeyForInstanceAndRealm(bg.InstanceID, RealmWithBattlegroupKey{RealmID: bg.RealmID, BattlegroupID: bg.BattleGroupID}))
	}

	return nil
}

func (b *battlegroundInMemRepo) DeleteAllWithGameServerAddress(ctx context.Context, address string) ([]battleground.Battleground, error) {
	b.m.Lock()
	defer b.m.Unlock()

	var keysToDelete []string
	var bgs []battleground.Battleground

	for key, bg := range b.instanceIDWithRealmIndex {
		if bg.GameserverAddress == address {
			keysToDelete = append(keysToDelete, key)
			bgs = append([]battleground.Battleground{*bg}, bgs...)
		}
	}

	for _, key := range keysToDelete {
		delete(b.instanceIDWithRealmIndex, key)
	}

	return bgs, nil
}

func (b *battlegroundInMemRepo) indexKeyForInstanceAndRealm(instanceID uint32, realmIDAndBattlegroup RealmWithBattlegroupKey) string {
	return fmt.Sprintf("%d:%d", instanceID, realmIDAndBattlegroup.RealmID)
}
