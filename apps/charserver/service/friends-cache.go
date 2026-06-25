package service

import (
	"context"
	"sync"
	"time"

	"github.com/walkline/ToCloud9/shared/events"
	wowguid "github.com/walkline/ToCloud9/shared/wow/guid"
)

//go:generate mockery --name=OnlinePlayersCache
type OnlinePlayersCache interface {
	PlayerLoggedIn(realmID uint32, playerGUID uint64, accountID uint32, name string, race, level, class, area uint32)
	PlayerLoggedOut(realmID uint32, playerGUID uint64)
	GetOnlineInfo(realmID uint32, playerGUID uint64) (OnlinePlayerInfo, bool)
	GetOnlineInfoForAccount(accountID uint32) (OnlinePlayerInfo, bool)
	GetOnlineInfosForAccount(accountID uint32) []OnlinePlayerInfo
	SetFriendsService(friendsService FriendsService)

	HandleCharacterLoggedIn(payload events.GWEventCharacterLoggedInPayload) error
	HandleCharacterLoggedOut(payload events.GWEventCharacterLoggedOutPayload) error
	HandleCharactersUpdates(payload events.GWEventCharactersUpdatesPayload) error
}

type onlinePlayersCacheImpl struct {
	// cacheMutex guards onlineInfoByGUID map
	cacheMutex sync.RWMutex

	// onlineInfoByGUID maps realm-scoped player keys to their online info.
	onlineInfoByGUID map[onlinePlayerKey]*OnlinePlayerInfo

	// onlineInfoByAccount maps account ids to their active characters.
	onlineInfoByAccount map[uint32]map[onlinePlayerKey]*OnlinePlayerInfo

	// lifecycleEventTimes stores the newest login/logout event time per player
	// so delayed cross-subject NATS delivery cannot resurrect stale presence.
	lifecycleEventTimes map[onlinePlayerKey]uint64

	// friendsService is used to notify friends about status changes
	friendsService FriendsService
}

func NewOnlinePlayersCache() OnlinePlayersCache {
	return &onlinePlayersCacheImpl{
		onlineInfoByGUID:    make(map[onlinePlayerKey]*OnlinePlayerInfo),
		onlineInfoByAccount: make(map[uint32]map[onlinePlayerKey]*OnlinePlayerInfo),
		lifecycleEventTimes: make(map[onlinePlayerKey]uint64),
	}
}

func (o *onlinePlayersCacheImpl) SetFriendsService(friendsService FriendsService) {
	o.friendsService = friendsService
}

type onlinePlayerKey struct {
	realmID uint32
	guid    uint64
}

func (o *onlinePlayersCacheImpl) PlayerLoggedIn(realmID uint32, playerGUID uint64, accountID uint32, name string, race, level, class, area uint32) {
	o.playerLoggedInAt(realmID, playerGUID, accountID, name, race, level, class, area, 0)
}

func (o *onlinePlayersCacheImpl) playerLoggedInAt(realmID uint32, playerGUID uint64, accountID uint32, name string, race, level, class, area uint32, eventTimeUnixNano uint64) bool {
	o.cacheMutex.Lock()
	defer o.cacheMutex.Unlock()

	playerGUID = wowguid.PlayerLowGUID(playerGUID)
	key := onlinePlayerKey{realmID: realmID, guid: playerGUID}
	if !o.shouldApplyLifecycleEventLocked(key, eventTimeUnixNano) {
		return false
	}
	if previous := o.onlineInfoByGUID[key]; previous != nil && previous.AccountID != accountID {
		delete(o.onlineInfoByAccount[previous.AccountID], key)
		if len(o.onlineInfoByAccount[previous.AccountID]) == 0 {
			delete(o.onlineInfoByAccount, previous.AccountID)
		}
	}

	info := &OnlinePlayerInfo{
		RealmID:   realmID,
		AccountID: accountID,
		GUID:      playerGUID,
		Name:      name,
		Race:      race,
		Level:     level,
		Class:     class,
		Area:      area,
		Status:    1, // online
	}
	o.onlineInfoByGUID[key] = info
	if o.onlineInfoByAccount[accountID] == nil {
		o.onlineInfoByAccount[accountID] = make(map[onlinePlayerKey]*OnlinePlayerInfo)
	}
	o.onlineInfoByAccount[accountID][key] = info
	o.rememberLifecycleEventTimeLocked(key, eventTimeUnixNano)
	return true
}

func (o *onlinePlayersCacheImpl) PlayerLoggedOut(realmID uint32, playerGUID uint64) {
	o.playerLoggedOutAt(realmID, playerGUID, 0)
}

func (o *onlinePlayersCacheImpl) playerLoggedOutAt(realmID uint32, playerGUID uint64, eventTimeUnixNano uint64) bool {
	o.cacheMutex.Lock()
	defer o.cacheMutex.Unlock()

	playerGUID = wowguid.PlayerLowGUID(playerGUID)
	key := onlinePlayerKey{realmID: realmID, guid: playerGUID}
	if !o.shouldApplyLifecycleEventLocked(key, eventTimeUnixNano) {
		return false
	}

	info := o.onlineInfoByGUID[key]
	delete(o.onlineInfoByGUID, key)
	if info != nil {
		delete(o.onlineInfoByAccount[info.AccountID], key)
		if len(o.onlineInfoByAccount[info.AccountID]) == 0 {
			delete(o.onlineInfoByAccount, info.AccountID)
		}
	}
	o.rememberLifecycleEventTimeLocked(key, eventTimeUnixNano)
	return true
}

func (o *onlinePlayersCacheImpl) GetOnlineInfo(realmID uint32, playerGUID uint64) (OnlinePlayerInfo, bool) {
	o.cacheMutex.RLock()
	defer o.cacheMutex.RUnlock()

	playerGUID = wowguid.PlayerLowGUID(playerGUID)
	info, ok := o.onlineInfoByGUID[onlinePlayerKey{realmID: realmID, guid: playerGUID}]
	if !ok {
		return OnlinePlayerInfo{}, false
	}
	return *info, true
}

func (o *onlinePlayersCacheImpl) GetOnlineInfoForAccount(accountID uint32) (OnlinePlayerInfo, bool) {
	o.cacheMutex.RLock()
	defer o.cacheMutex.RUnlock()

	infos := o.onlineInfoByAccount[accountID]
	for _, info := range infos {
		return *info, true
	}
	return OnlinePlayerInfo{}, false
}

func (o *onlinePlayersCacheImpl) GetOnlineInfosForAccount(accountID uint32) []OnlinePlayerInfo {
	o.cacheMutex.RLock()
	defer o.cacheMutex.RUnlock()

	infos := o.onlineInfoByAccount[accountID]
	result := make([]OnlinePlayerInfo, 0, len(infos))
	for _, info := range infos {
		result = append(result, *info)
	}
	return result
}

// HandleCharacterLoggedIn handles character login event from gateway
func (o *onlinePlayersCacheImpl) HandleCharacterLoggedIn(payload events.GWEventCharacterLoggedInPayload) error {
	playerGUID := wowguid.PlayerLowGUID(payload.CharGUID)
	if !o.playerLoggedInAt(payload.RealmID, playerGUID, payload.AccountID, payload.CharName, uint32(payload.CharRace), uint32(payload.CharLevel), uint32(payload.CharClass), payload.CharZone, payload.EventTimeUnixNano) {
		return nil
	}

	// Notify friends service about login
	if o.friendsService != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return o.friendsService.NotifyStatusChange(
			ctx,
			payload.RealmID,
			playerGUID,
			1, // online
			payload.CharZone,
			uint32(payload.CharLevel),
			uint32(payload.CharClass),
		)
	}

	return nil
}

// HandleCharacterLoggedOut handles character logout event from gateway
func (o *onlinePlayersCacheImpl) HandleCharacterLoggedOut(payload events.GWEventCharacterLoggedOutPayload) error {
	playerGUID := wowguid.PlayerLowGUID(payload.CharGUID)
	if !o.playerLoggedOutAt(payload.RealmID, playerGUID, payload.EventTimeUnixNano) {
		return nil
	}

	// Notify friends service about logout
	if o.friendsService != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return o.friendsService.NotifyStatusChange(
			ctx,
			payload.RealmID,
			playerGUID,
			0, // offline
			0, 0, 0,
		)
	}

	return nil
}

// HandleCharactersUpdates handles character updates (zone/level/area changes) from gateway
func (o *onlinePlayersCacheImpl) HandleCharactersUpdates(payload events.GWEventCharactersUpdatesPayload) error {
	o.cacheMutex.Lock()
	defer o.cacheMutex.Unlock()

	for _, update := range payload.Updates {
		key := onlinePlayerKey{realmID: payload.RealmID, guid: wowguid.PlayerLowGUID(update.ID)}
		info, exists := o.onlineInfoByGUID[key]
		if !exists {
			continue
		}
		eventTimeUnixNano := update.EventTimeUnixNano
		if eventTimeUnixNano == 0 {
			eventTimeUnixNano = payload.EventTimeUnixNano
		}
		if eventTimeUnixNano != 0 && o.lifecycleEventTimes[key] > eventTimeUnixNano {
			continue
		}

		// Update level
		if update.Lvl != nil {
			info.Level = uint32(*update.Lvl)
		}

		// Update area/zone (use Zone for Area field since that's what friends see)
		if update.Zone != nil {
			info.Area = *update.Zone
		} else if update.Area != nil {
			info.Area = *update.Area
		}
	}

	return nil
}

func (o *onlinePlayersCacheImpl) shouldApplyLifecycleEventLocked(key onlinePlayerKey, eventTimeUnixNano uint64) bool {
	return eventTimeUnixNano == 0 || o.lifecycleEventTimes[key] <= eventTimeUnixNano
}

func (o *onlinePlayersCacheImpl) rememberLifecycleEventTimeLocked(key onlinePlayerKey, eventTimeUnixNano uint64) {
	if eventTimeUnixNano == 0 {
		return
	}
	if o.lifecycleEventTimes == nil {
		o.lifecycleEventTimes = map[onlinePlayerKey]uint64{}
	}
	o.lifecycleEventTimes[key] = eventTimeUnixNano
}
