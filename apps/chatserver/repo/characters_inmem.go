package repo

import (
	"context"
	"fmt"
	"strings"
	"sync"
)

type charactersInMemRepo struct {
	charsByGUID map[string]*Character
	charsByName map[string]*Character

	guidMu sync.RWMutex
	nameMu sync.RWMutex
}

func NewCharactersInMemRepo() CharactersRepo {
	return &charactersInMemRepo{
		charsByGUID: map[string]*Character{},
		charsByName: map[string]*Character{},
	}
}

func (c *charactersInMemRepo) AddCharacter(ctx context.Context, character *Character) error {
	c.guidMu.Lock()
	c.charsByGUID[c.mapKeyForRealmAndGuid(character.RealmID, character.GUID)] = character
	c.guidMu.Unlock()

	c.nameMu.Lock()
	c.charsByName[c.mapKeyForRealmAndName(character.RealmID, character.Name)] = character
	c.nameMu.Unlock()

	return nil
}

func (c *charactersInMemRepo) RemoveCharacter(ctx context.Context, realmID uint32, characterGUID uint64) error {
	guidKey := c.mapKeyForRealmAndGuid(realmID, characterGUID)
	c.guidMu.Lock()
	char := c.charsByGUID[guidKey]
	delete(c.charsByGUID, guidKey)
	c.guidMu.Unlock()

	if char != nil {
		c.nameMu.Lock()
		delete(c.charsByName, c.mapKeyForRealmAndName(realmID, char.Name))
		c.nameMu.Unlock()
	}

	return nil
}

func (c *charactersInMemRepo) CharacterByRealmAndGUID(ctx context.Context, realmID uint32, characterGUID uint64) (*Character, error) {
	c.guidMu.RLock()
	defer c.guidMu.RUnlock()
	return c.charsByGUID[c.mapKeyForRealmAndGuid(realmID, characterGUID)], nil
}

func (c *charactersInMemRepo) CharacterByRealmAndName(ctx context.Context, realmID uint32, name string) (*Character, error) {
	c.nameMu.RLock()
	defer c.nameMu.RUnlock()
	return c.charsByName[c.mapKeyForRealmAndName(realmID, name)], nil
}

func (c *charactersInMemRepo) RemoveCharactersWithRealm(ctx context.Context, realmID uint32) error {
	// TODO: need to completely rewrite this

	keysToDeleteNames := []string{}
	keysToDeleteGuid := []string{}

	prefix := fmt.Sprintf("%d:", realmID)

	c.nameMu.Lock()

	for k, char := range c.charsByName {
		if strings.HasPrefix(k, prefix) {
			keysToDeleteNames = append(keysToDeleteNames, k)
			keysToDeleteGuid = append(keysToDeleteGuid, c.mapKeyForRealmAndGuid(realmID, char.GUID))
		}
	}

	for _, k := range keysToDeleteNames {
		delete(c.charsByName, k)
	}

	for _, k := range keysToDeleteGuid {
		delete(c.charsByGUID, k)
	}

	c.nameMu.Unlock()

	return nil
}

func (c *charactersInMemRepo) mapKeyForRealmAndGuid(realm uint32, guid uint64) string {
	return fmt.Sprintf("%d:%d", realm, guid)
}

func (c *charactersInMemRepo) mapKeyForRealmAndName(realm uint32, name string) string {
	return fmt.Sprintf("%d:%s", realm, strings.ToLower(name))
}
