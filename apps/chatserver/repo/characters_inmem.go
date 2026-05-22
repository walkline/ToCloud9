package repo

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"
)

type charactersInMemRepo struct {
	charsByGUID         map[string]*Character
	charsByName         map[string]*Character
	lifecycleEventTimes map[string]uint64

	guidMu sync.RWMutex
	nameMu sync.RWMutex
}

func NewCharactersInMemRepo() CharactersRepo {
	return &charactersInMemRepo{
		charsByGUID:         map[string]*Character{},
		charsByName:         map[string]*Character{},
		lifecycleEventTimes: map[string]uint64{},
	}
}

func (c *charactersInMemRepo) AddCharacter(ctx context.Context, character *Character) error {
	_, err := c.addCharacter(character, 0)
	return err
}

func (c *charactersInMemRepo) AddCharacterFromGatewayEvent(ctx context.Context, character *Character, eventTimeUnixNano uint64) (bool, error) {
	return c.addCharacter(character, eventTimeUnixNano)
}

func (c *charactersInMemRepo) RemoveCharacter(ctx context.Context, realmID uint32, characterGUID uint64) error {
	_, err := c.removeCharacter(realmID, characterGUID, 0)
	return err
}

func (c *charactersInMemRepo) RemoveCharacterFromGatewayEvent(ctx context.Context, realmID uint32, characterGUID uint64, eventTimeUnixNano uint64) (bool, error) {
	return c.removeCharacter(realmID, characterGUID, eventTimeUnixNano)
}

func (c *charactersInMemRepo) addCharacter(character *Character, eventTimeUnixNano uint64) (bool, error) {
	if character == nil {
		return false, nil
	}

	guidKey := c.mapKeyForRealmAndGuid(character.RealmID, character.GUID)
	c.guidMu.Lock()
	c.nameMu.Lock()
	defer c.nameMu.Unlock()
	defer c.guidMu.Unlock()

	if !c.shouldApplyLifecycleEvent(guidKey, eventTimeUnixNano) {
		return false, nil
	}
	if previous := c.charsByGUID[guidKey]; previous != nil {
		delete(c.charsByName, c.mapKeyForRealmAndName(previous.RealmID, previous.Name))
	}
	c.charsByGUID[guidKey] = character
	c.charsByName[c.mapKeyForRealmAndName(character.RealmID, character.Name)] = character
	c.rememberLifecycleEventTime(guidKey, eventTimeUnixNano)
	return true, nil
}

func (c *charactersInMemRepo) removeCharacter(realmID uint32, characterGUID uint64, eventTimeUnixNano uint64) (bool, error) {
	guidKey := c.mapKeyForRealmAndGuid(realmID, characterGUID)
	c.guidMu.Lock()
	c.nameMu.Lock()
	defer c.nameMu.Unlock()
	defer c.guidMu.Unlock()

	if !c.shouldApplyLifecycleEvent(guidKey, eventTimeUnixNano) {
		return false, nil
	}
	char := c.charsByGUID[guidKey]
	delete(c.charsByGUID, guidKey)

	if char != nil {
		delete(c.charsByName, c.mapKeyForRealmAndName(realmID, char.Name))
	}
	c.rememberLifecycleEventTime(guidKey, eventTimeUnixNano)

	return true, nil
}

func (c *charactersInMemRepo) RemoveCharactersWithGatewayID(ctx context.Context, realmID uint32, gatewayID string, eventTimeUnixNano uint64) error {
	c.guidMu.Lock()
	c.nameMu.Lock()
	defer c.nameMu.Unlock()
	defer c.guidMu.Unlock()
	if eventTimeUnixNano == 0 {
		eventTimeUnixNano = uint64(time.Now().UnixNano())
	}

	for guidKey, char := range c.charsByGUID {
		if char.RealmID == realmID && char.GatewayID == gatewayID && c.lifecycleEventTimes[guidKey] <= eventTimeUnixNano {
			delete(c.charsByGUID, guidKey)
			delete(c.charsByName, c.mapKeyForRealmAndName(realmID, char.Name))
			c.rememberLifecycleEventTime(guidKey, eventTimeUnixNano)
		}
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

func (c *charactersInMemRepo) CharactersByName(ctx context.Context, name string) ([]*Character, error) {
	c.nameMu.RLock()
	defer c.nameMu.RUnlock()

	matches := make([]*Character, 0)
	nameSuffix := ":" + strings.ToLower(name)
	for key, char := range c.charsByName {
		if strings.HasSuffix(key, nameSuffix) {
			matches = append(matches, char)
		}
	}

	return matches, nil
}

func (c *charactersInMemRepo) RemoveCharactersWithRealm(ctx context.Context, realmID uint32) error {
	// TODO: need to completely rewrite this

	keysToDeleteNames := []string{}
	keysToDeleteGuid := []string{}

	prefix := fmt.Sprintf("%d:", realmID)

	c.guidMu.Lock()
	c.nameMu.Lock()
	defer c.nameMu.Unlock()
	defer c.guidMu.Unlock()

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

	return nil
}

func (c *charactersInMemRepo) mapKeyForRealmAndGuid(realm uint32, guid uint64) string {
	return fmt.Sprintf("%d:%d", realm, guid)
}

func (c *charactersInMemRepo) mapKeyForRealmAndName(realm uint32, name string) string {
	return fmt.Sprintf("%d:%s", realm, strings.ToLower(name))
}

func (c *charactersInMemRepo) shouldApplyLifecycleEvent(guidKey string, eventTimeUnixNano uint64) bool {
	return eventTimeUnixNano == 0 || c.lifecycleEventTimes[guidKey] <= eventTimeUnixNano
}

func (c *charactersInMemRepo) rememberLifecycleEventTime(guidKey string, eventTimeUnixNano uint64) {
	if eventTimeUnixNano == 0 {
		return
	}
	if c.lifecycleEventTimes == nil {
		c.lifecycleEventTimes = map[string]uint64{}
	}
	c.lifecycleEventTimes[guidKey] = eventTimeUnixNano
}
