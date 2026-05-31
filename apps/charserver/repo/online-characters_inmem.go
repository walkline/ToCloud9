package repo

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/walkline/ToCloud9/shared/events"
)

type charactersOnlineInMem struct {
	guidStorage         map[uint32]map[uint64]*Character
	nameStorage         map[uint32]map[string]*Character
	lifecycleEventTimes map[uint32]map[uint64]uint64
	m                   sync.RWMutex
}

func NewCharactersOnlineInMem() CharactersOnline {
	return &charactersOnlineInMem{
		guidStorage:         map[uint32]map[uint64]*Character{},
		nameStorage:         map[uint32]map[string]*Character{},
		lifecycleEventTimes: map[uint32]map[uint64]uint64{},
	}
}

func (c *charactersOnlineInMem) Add(_ context.Context, character *Character) error {
	c.m.Lock()
	c.addLocked(character, 0)
	c.m.Unlock()
	return nil
}

func (c *charactersOnlineInMem) Remove(ctx context.Context, realmID uint32, guid uint64) error {
	c.m.Lock()
	c.removeLocked(realmID, guid, 0)
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

func (c *charactersOnlineInMem) AllGUIDsByRealm(ctx context.Context, realmID uint32) ([]uint64, error) {
	c.m.RLock()
	defer c.m.RUnlock()
	s, found := c.guidStorage[realmID]
	if !found {
		return []uint64{}, nil
	}

	guids := make([]uint64, 0, len(s))
	for guid := range s {
		guids = append(guids, guid)
	}
	return guids, nil
}

func (c *charactersOnlineInMem) HandleCharacterLoggedIn(payload events.GWEventCharacterLoggedInPayload) error {
	c.m.Lock()
	c.addLocked(&Character{
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
	}, payload.EventTimeUnixNano)
	c.m.Unlock()
	return nil
}

func (c *charactersOnlineInMem) HandleCharacterLoggedOut(payload events.GWEventCharacterLoggedOutPayload) error {
	c.m.Lock()
	c.removeLocked(payload.RealmID, payload.CharGUID, payload.EventTimeUnixNano)
	c.m.Unlock()
	return nil
}

func (c *charactersOnlineInMem) HandleCharactersUpdates(payload events.GWEventCharactersUpdatesPayload) error {
	c.m.Lock()
	for _, update := range payload.Updates {
		member := c.guidStorage[payload.RealmID][update.ID]
		if member != nil {
			if payload.GatewayID != "" && member.GatewayID != "" && payload.GatewayID != member.GatewayID {
				continue
			}
			eventTimeUnixNano := update.EventTimeUnixNano
			if eventTimeUnixNano == 0 {
				eventTimeUnixNano = payload.EventTimeUnixNano
			}
			if eventTimeUnixNano != 0 && c.lifecycleEventTimeLocked(payload.RealmID, update.ID) > eventTimeUnixNano {
				continue
			}
			applyCharUpdate(member, update)
		}
	}
	c.m.Unlock()
	return nil
}

func (c *charactersOnlineInMem) RemoveAllWithGatewayID(ctx context.Context, realmID uint32, gatewayID string, eventTimeUnixNano uint64) ([]uint64, error) {
	charsToDelete := make([]uint64, 0, 20)

	c.m.Lock()
	storage := c.guidStorage[realmID]
	namesStorage := c.nameStorage[realmID]
	if eventTimeUnixNano == 0 {
		eventTimeUnixNano = uint64(time.Now().UnixNano())
	}

	for guid, char := range storage {
		if char.GatewayID == gatewayID && c.lifecycleEventTimeLocked(realmID, guid) <= eventTimeUnixNano {
			charsToDelete = append(charsToDelete, guid)
		}
	}

	for _, guid := range charsToDelete {
		delete(namesStorage, characterNameKey(storage[guid].CharName))
		delete(storage, guid)
		c.rememberLifecycleEventTimeLocked(realmID, guid, eventTimeUnixNano)
	}
	c.m.Unlock()

	return charsToDelete, nil
}

func (c *charactersOnlineInMem) addLocked(character *Character, eventTimeUnixNano uint64) {
	if character == nil {
		return
	}
	if !c.shouldApplyLifecycleEventLocked(character.RealmID, character.CharGUID, eventTimeUnixNano) {
		return
	}

	c.ensureRealmStorageLocked(character.RealmID)
	if previous := c.guidStorage[character.RealmID][character.CharGUID]; previous != nil {
		delete(c.nameStorage[character.RealmID], characterNameKey(previous.CharName))
	}

	c.guidStorage[character.RealmID][character.CharGUID] = character
	c.nameStorage[character.RealmID][characterNameKey(character.CharName)] = character
	c.rememberLifecycleEventTimeLocked(character.RealmID, character.CharGUID, eventTimeUnixNano)
}

func (c *charactersOnlineInMem) removeLocked(realmID uint32, guid uint64, eventTimeUnixNano uint64) {
	if !c.shouldApplyLifecycleEventLocked(realmID, guid, eventTimeUnixNano) {
		return
	}

	storage := c.guidStorage[realmID]
	if storage != nil {
		if char := storage[guid]; char != nil {
			delete(c.nameStorage[realmID], characterNameKey(char.CharName))
		}
		delete(storage, guid)
	}
	c.rememberLifecycleEventTimeLocked(realmID, guid, eventTimeUnixNano)
}

func (c *charactersOnlineInMem) ensureRealmStorageLocked(realmID uint32) {
	if c.guidStorage[realmID] == nil {
		c.guidStorage[realmID] = map[uint64]*Character{}
	}
	if c.nameStorage[realmID] == nil {
		c.nameStorage[realmID] = map[string]*Character{}
	}
}

func (c *charactersOnlineInMem) shouldApplyLifecycleEventLocked(realmID uint32, guid uint64, eventTimeUnixNano uint64) bool {
	return eventTimeUnixNano == 0 || c.lifecycleEventTimeLocked(realmID, guid) <= eventTimeUnixNano
}

func (c *charactersOnlineInMem) lifecycleEventTimeLocked(realmID uint32, guid uint64) uint64 {
	realmEvents := c.lifecycleEventTimes[realmID]
	if realmEvents == nil {
		return 0
	}
	return realmEvents[guid]
}

func (c *charactersOnlineInMem) rememberLifecycleEventTimeLocked(realmID uint32, guid uint64, eventTimeUnixNano uint64) {
	if eventTimeUnixNano == 0 {
		return
	}
	if c.lifecycleEventTimes == nil {
		c.lifecycleEventTimes = map[uint32]map[uint64]uint64{}
	}
	if c.lifecycleEventTimes[realmID] == nil {
		c.lifecycleEventTimes[realmID] = map[uint64]uint64{}
	}
	c.lifecycleEventTimes[realmID][guid] = eventTimeUnixNano
}

func characterNameKey(name string) string {
	return strings.ToUpper(name)
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
		if len(query.Zones) > 0 {
			showZones = false
			for _, zone := range query.Zones {
				if char.CharZone == zone {
					showZones = true
					break
				}
			}
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
