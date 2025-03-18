package service

import (
	"context"
	"errors"

	"github.com/walkline/ToCloud9/apps/gateway/repo"
)

type RealmNamesService struct {
	r repo.RealmNamesRepo

	cache map[uint32]repo.RealmName
}

func NewRealmNamesService(ctx context.Context, r repo.RealmNamesRepo) (*RealmNamesService, error) {
	realms, err := r.LoadRealmNames(ctx)
	if err != nil {
		return nil, err
	}

	cache := make(map[uint32]repo.RealmName, len(realms))
	for _, realm := range realms {
		cache[realm.RealmID] = *realm
	}
	return &RealmNamesService{r: r, cache: cache}, nil
}

func (r *RealmNamesService) NameByID(ctx context.Context, realmID uint32) (string, error) {
	realm, ok := r.cache[realmID]
	if !ok {
		return "", errors.New("realm not found")
	}
	return realm.Name, nil
}
