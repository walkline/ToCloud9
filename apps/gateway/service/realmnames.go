package service

import (
	"context"
	"errors"
	"strings"

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

func (r *RealmNamesService) IDByName(ctx context.Context, name string) (uint32, error) {
	normalizedName := normalizeRealmName(name)
	for _, realm := range r.cache {
		if normalizeRealmName(realm.Name) == normalizedName {
			return realm.RealmID, nil
		}
	}

	return 0, errors.New("realm not found")
}

func normalizeRealmName(name string) string {
	name = strings.ToLower(name)
	name = strings.ReplaceAll(name, " ", "")
	name = strings.ReplaceAll(name, "'", "")
	return name
}
