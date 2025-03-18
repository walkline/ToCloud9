package repo

import (
	"context"
	"strings"
	"sync"

	"github.com/walkline/ToCloud9/shared/events"
)

type charactersOnlineInMem struct {
	guidStorage map[uint32]map[uint64]*Character
	nameStorage map[uint32]map[string]*Character
	m           sync.RWMutex
}

func NewCharactersOnlineInMem() CharactersOnline {
	return &charactersOnlineInMem{
		guidStorage: map[uint32]map[uint64]*Character{},
		nameStorage: map[uint32]map[string]*Character{},
	}
}

func (c *charactersOnlineInMem) Add(_ context.Context, character *Character) error {
	c.m.Lock()
	if c.guidStorage[character.RealmID] == nil {
		c.guidStorage[character.RealmID] = map[uint64]*Character{}
		c.nameStorage[character.RealmID] = map[string]*Character{}
	}
	c.guidStorage[character.RealmID][character.CharGUID] = character
	c.nameStorage[character.RealmID][strings.ToUpper(character.CharName)] = character
	c.m.Unlock()
	return nil
}

func (c *charactersOnlineInMem) Remove(ctx context.Context, realmID uint32, guid uint64) error {
	char, err := c.OneByRealmAndGUID(ctx, realmID, guid)
	if err != nil {
		return err
	}
	if char == nil {
		return nil
	}
	c.m.Lock()
	delete(c.guidStorage[realmID], guid)
	delete(c.nameStorage[realmID], char.CharName)
	c.m.Unlock()
	return nil
}

func (c *charactersOnlineInMem) OneByRealmAndGUID(_ context.Context, realmID uint32, guid uint64) (*Character, error) {
	c.m.RLock()
	defer c.m.RUnlock()
	s, found := c.guidStorage[realmID]
	if !found {
		return nil, nil
	}
	v, found := s[guid]
	if !found {
		return nil, nil
	}
	return v, nil
}

func (c *charactersOnlineInMem) OneByRealmAndName(_ context.Context, realmID uint32, name string) (*Character, error) {
	c.m.RLock()
	defer c.m.RUnlock()
	s, found := c.nameStorage[realmID]
	if !found {
		return nil, nil
	}
	v, found := s[strings.ToUpper(name)]
	if !found {
		return nil, nil
	}
	return v, nil
}

func (c *charactersOnlineInMem) CharactersByRealmAndGUIDs(ctx context.Context, realmID uint32, guids []uint64) ([]Character, error) {
	c.m.RLock()
	defer c.m.RUnlock()
	s, found := c.guidStorage[realmID]
	if !found {
		return nil, nil
	}

	res := make([]Character, 0, len(guids))
	for _, guid := range guids {
		v, found := s[guid]
		if !found {
			continue
		}

		res = append(res, *v)
	}
	return res, nil
}

func (c *charactersOnlineInMem) HandleCharacterLoggedIn(payload events.GWEventCharacterLoggedInPayload) error {
	return c.Add(context.TODO(), &Character{
		RealmID:     payload.RealmID,
		GatewayID:   payload.GatewayID,
		CharGUID:    payload.CharGUID,
		CharName:    payload.CharName,
		CharRace:    payload.CharRace,
		CharClass:   payload.CharClass,
		CharGender:  payload.CharGender,
		CharLevel:   payload.CharLevel,
		CharZone:    payload.CharZone,
		CharMap:     payload.CharMap,
		CharPosX:    payload.CharPosX,
		CharPosY:    payload.CharPosY,
		CharPosZ:    payload.CharPosZ,
		CharGuildID: payload.CharGuildID,
		AccountID:   payload.AccountID,
	})
}

func (c *charactersOnlineInMem) HandleCharacterLoggedOut(payload events.GWEventCharacterLoggedOutPayload) error {
	return c.Remove(context.TODO(), payload.RealmID, payload.CharGUID)
}

func (c *charactersOnlineInMem) HandleCharactersUpdates(payload events.GWEventCharactersUpdatesPayload) error {
	c.m.Lock()
	for _, update := range payload.Updates {
		member := c.guidStorage[payload.RealmID][update.ID]
		if member != nil {
			applyCharUpdate(member, update)
		}
	}
	c.m.Unlock()
	return nil
}

func (c *charactersOnlineInMem) RemoveAllWithGatewayID(ctx context.Context, realmID uint32, gatewayID string) ([]uint64, error) {
	charsToDelete := make([]uint64, 0, 20)

	c.m.Lock()
	storage := c.guidStorage[realmID]
	namesStorage := c.nameStorage[realmID]

	for guid, char := range storage {
		if char.GatewayID == gatewayID {
			charsToDelete = append(charsToDelete, guid)
		}
	}

	for _, guid := range charsToDelete {
		delete(namesStorage, storage[guid].CharName)
		delete(storage, guid)
	}
	c.m.Unlock()

	return charsToDelete, nil
}

func applyCharUpdate(c *Character, upd *events.CharacterUpdate) {
	if upd.Zone != nil {
		c.CharZone = *upd.Zone
	}

	if upd.Lvl != nil {
		c.CharLevel = *upd.Lvl
	}

	if upd.Map != nil {
		c.CharMap = *upd.Map
	}
}

func (c *charactersOnlineInMem) WhoRequest(_ context.Context, requesterRealmID uint32, requesterGUID uint64, query CharactersWhoQuery) ([]Character, error) {
	c.m.RLock()
	chars := make([]Character, len(c.guidStorage[requesterRealmID]))
	i := 0
	realmStorage := c.guidStorage[requesterRealmID]
	for k := range realmStorage {
		chars[i] = *realmStorage[k]
		i++
	}
	c.m.RUnlock()

	var result []Character
	for _, char := range chars {
		if requesterRealmID != char.RealmID {
			continue
		}

		if query.LvlMax < char.CharLevel || query.LvlMin > char.CharLevel {
			continue
		}

		race := uint32(char.CharRace)
		if query.RaceMask&(1<<race) == 0 {
			continue
		}

		class := uint32(char.CharClass)
		if query.ClassMask&(1<<class) == 0 {
			continue
		}

		showZones := true
		for zone := range query.Zones {
			if char.CharZone == uint32(zone) {
				showZones = true
				break
			}
			showZones = false
		}

		if !showZones {
			continue
		}

		if len(query.Strings) > 0 {
			showName := false
			for _, s := range query.Strings {
				if strings.Contains(strings.ToLower(char.CharName), strings.ToLower(s)) {
					showName = true
					break
				}
			}
			if !showName {
				continue
			}
		}

		result = append(result, char)
	}

	return result, nil
}
