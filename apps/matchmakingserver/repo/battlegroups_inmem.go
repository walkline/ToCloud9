package repo

import (
	"context"
	"fmt"
	"strconv"
	"strings"
)

type BattleGroupsInMem struct {
	battleGroupIDByRealmID map[uint32]uint32
}

func NewBattleGroupsInMemWithConfigValue(valueFromConfig map[uint32]string) (BattleGroupsRepository, error) {
	repo := BattleGroupsInMem{
		battleGroupIDByRealmID: make(map[uint32]uint32),
	}
	for battleGroupID, realmsString := range valueFromConfig {
		realmsStrings := strings.Split(realmsString, ",")
		for _, realm := range realmsStrings {
			realmID, err := strconv.ParseUint(realm, 10, 32)
			if err != nil {
				return nil, fmt.Errorf("cannot parse realmID for battlegroup, realmID: %s", realm)
			}
			repo.battleGroupIDByRealmID[uint32(realmID)] = battleGroupID
		}
	}

	return &repo, nil
}

func (r *BattleGroupsInMem) BattleGroupIDByRealmID(_ context.Context, realmID uint32) (uint32, error) {
	return r.battleGroupIDByRealmID[realmID], nil
}

func (r *BattleGroupsInMem) AllRealmsInBattleGroups(_ context.Context) ([]uint32, error) {
	results := make([]uint32, 0, len(r.battleGroupIDByRealmID))
	for realmID := range r.battleGroupIDByRealmID {
		results = append(results, realmID)
	}
	return results, nil
}

func (r *BattleGroupsInMem) AllBattleGroupsIDs(_ context.Context) ([]uint32, error) {
	uniqueRealmGroupsSet := map[uint32]struct{}{}
	for _, battlegroupID := range r.battleGroupIDByRealmID {
		uniqueRealmGroupsSet[battlegroupID] = struct{}{}
	}

	results := make([]uint32, 0, len(uniqueRealmGroupsSet))
	for battlegroupID := range uniqueRealmGroupsSet {
		results = append(results, battlegroupID)
	}
	return results, nil
}
