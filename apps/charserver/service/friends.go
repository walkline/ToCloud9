package service

import (
	"context"
	"errors"
	"sort"

	"github.com/walkline/ToCloud9/apps/charserver"
	"github.com/walkline/ToCloud9/apps/charserver/repo"
	"github.com/walkline/ToCloud9/shared/authidentity"
	"github.com/walkline/ToCloud9/shared/events"
	wowguid "github.com/walkline/ToCloud9/shared/wow/guid"
)

const (
	MaxFriendsLimit = 50
	MaxIgnoreLimit  = 50
)

// FriendsResult enum values matching AzerothCore protocol
const (
	FriendResultDBError        = 0x00
	FriendResultListFull       = 0x01
	FriendResultOnline         = 0x02
	FriendResultOffline        = 0x03
	FriendResultNotFound       = 0x04
	FriendResultRemoved        = 0x05
	FriendResultAddedOnline    = 0x06
	FriendResultAddedOffline   = 0x07
	FriendResultAlready        = 0x08
	FriendResultSelf           = 0x09
	FriendResultEnemy          = 0x0A
	FriendResultIgnoreSelf     = 0x0B
	FriendResultIgnoreNotFound = 0x0C
	FriendResultIgnoreAlready  = 0x0D
	FriendResultIgnoreAdded    = 0x0E
	FriendResultIgnoreRemoved  = 0x0F
	FriendResultIgnoreFull     = 0x10
)

var (
	ErrFriendNotFound = errors.New("friend not found")
	ErrFriendListFull = errors.New("friend list is full")
	ErrIgnoreListFull = errors.New("ignore list is full")
	ErrAlreadyFriend  = errors.New("already friends")
	ErrAlreadyIgnored = errors.New("already ignored")
	ErrCannotAddSelf  = errors.New("cannot add self")
)

type OnlinePlayerInfo struct {
	RealmID   uint32
	AccountID uint32
	GUID      uint64
	Name      string
	Race      uint32
	Level     uint32
	Class     uint32
	Area      uint32
	Status    uint8
}

type FriendInfo struct {
	RealmID uint32
	GUID    uint64
	Note    string
	Status  uint8
	Area    uint32
	Level   uint32
	ClassID uint32
}

type FriendsList struct {
	Friends []*FriendInfo
	Ignored []uint64
}

type AddFriendResult struct {
	Result     uint32
	FriendGUID uint64
	Status     uint8
	Area       uint32
	Level      uint32
	ClassID    uint32
	Pending    bool
	Accepted   bool
}

//go:generate mockery --name=FriendsService
type FriendsService interface {
	GetFriendsList(ctx context.Context, realmID uint32, playerGUID uint64) (*FriendsList, error)
	AddFriend(ctx context.Context, realmID uint32, playerGUID, friendGUID uint64, friendName, note string) (*AddFriendResult, error)
	AddRealIDFriendByEmail(ctx context.Context, realmID uint32, playerGUID uint64, accountID uint32, email, note string) (*AddFriendResult, error)
	AcceptRealIDFriend(ctx context.Context, realmID uint32, playerGUID uint64, accountID uint32, requesterAccountID uint32, note string) (*AddFriendResult, error)
	AreRealIDFriends(ctx context.Context, accountID uint32, friendAccountID uint32) (bool, error)
	RemoveFriend(ctx context.Context, realmID uint32, playerGUID, friendGUID uint64) error
	SetFriendNote(ctx context.Context, realmID uint32, playerGUID, friendGUID uint64, note string) error
	AddIgnore(ctx context.Context, realmID uint32, playerGUID, ignoredGUID uint64) (uint32, error)
	RemoveIgnore(ctx context.Context, realmID uint32, playerGUID, ignoredGUID uint64) error

	// NotifyStatusChange is called when player logs in/out
	NotifyStatusChange(ctx context.Context, realmID uint32, playerGUID uint64, status uint8, area, level, classID uint32) error
}

type friendsServiceImpl struct {
	charRepo       repo.Characters
	accountRepo    repo.Accounts
	onlineCache    OnlinePlayersCache
	eventsProducer events.FriendsServiceProducer

	realIDCrossFaction bool
}

type FriendsServiceOption func(*friendsServiceImpl)

func WithRealIDCrossFaction(enabled bool) FriendsServiceOption {
	return func(f *friendsServiceImpl) {
		f.realIDCrossFaction = enabled
	}
}

func NewFriendsService(charRepo repo.Characters, accountRepo repo.Accounts, onlineCache OnlinePlayersCache, eventsProducer events.FriendsServiceProducer, opts ...FriendsServiceOption) FriendsService {
	service := &friendsServiceImpl{
		charRepo:           charRepo,
		accountRepo:        accountRepo,
		onlineCache:        onlineCache,
		eventsProducer:     eventsProducer,
		realIDCrossFaction: true,
	}
	for _, opt := range opts {
		opt(service)
	}
	return service
}

func (f *friendsServiceImpl) GetFriendsList(ctx context.Context, realmID uint32, playerGUID uint64) (*FriendsList, error) {
	entries, err := f.charRepo.GetFriendsForPlayer(ctx, realmID, playerGUID)
	if err != nil {
		return nil, err
	}

	result := &FriendsList{
		Friends: make([]*FriendInfo, 0),
		Ignored: make([]uint64, 0),
	}
	seenFriendGUIDs := make(map[uint64]struct{})

	for _, entry := range entries {
		if entry.Flags == repo.SocialFlagFriend {
			friend := &FriendInfo{
				RealmID: realmID,
				GUID:    entry.FriendGUID,
				Note:    entry.Note,
			}

			// Check if friend is online
			if onlineInfo, ok := f.onlineCache.GetOnlineInfo(realmID, entry.FriendGUID); ok {
				friend.Status = 1 // online
				friend.Area = onlineInfo.Area
				friend.Level = onlineInfo.Level
				friend.ClassID = onlineInfo.Class
			} else {
				friend.Status = 0 // offline
			}

			result.Friends = append(result.Friends, friend)
			seenFriendGUIDs[friend.GUID] = struct{}{}
		} else if entry.Flags == repo.SocialFlagIgnore {
			result.Ignored = append(result.Ignored, entry.FriendGUID)
		}
	}

	if f.accountRepo != nil {
		player, err := f.charRepo.CharacterByGUID(ctx, realmID, playerGUID)
		if err != nil {
			return nil, err
		}
		if player != nil {
			realIDFriends, err := f.accountRepo.AcceptedRealIDFriends(ctx, player.AccountID)
			if err != nil {
				return nil, err
			}
			for _, realIDFriend := range realIDFriends {
				friend, err := f.realIDFriendInfo(ctx, realmID, player, realIDFriend)
				if err != nil {
					return nil, err
				}
				if friend != nil {
					if _, ok := seenFriendGUIDs[friend.GUID]; ok {
						continue
					}
					result.Friends = append(result.Friends, friend)
					seenFriendGUIDs[friend.GUID] = struct{}{}
				}
			}
		}
	}

	return result, nil
}

func (f *friendsServiceImpl) AddFriend(ctx context.Context, realmID uint32, playerGUID, friendGUID uint64, friendName, note string) (*AddFriendResult, error) {
	friendRealmID := wowguid.PlayerRealmIDOrDefault(realmID, friendGUID)
	friendLowGUID := wowguid.PlayerLowGUID(friendGUID)
	if friendRealmID != realmID {
		return &AddFriendResult{Result: FriendResultNotFound}, nil
	}

	// Cannot add self
	if playerGUID == friendLowGUID {
		return &AddFriendResult{Result: FriendResultSelf}, nil
	}

	// Check if already friends
	entries, err := f.charRepo.GetFriendsForPlayer(ctx, realmID, playerGUID)
	if err != nil {
		return &AddFriendResult{Result: FriendResultDBError}, err
	}

	friendCount := 0
	for _, entry := range entries {
		if entry.Flags == repo.SocialFlagFriend {
			if entry.FriendGUID == friendLowGUID {
				return &AddFriendResult{Result: FriendResultAlready}, nil
			}
			friendCount++
		}
	}

	// Check friend list limit
	if friendCount >= MaxFriendsLimit {
		return &AddFriendResult{Result: FriendResultListFull}, nil
	}

	// Add friend
	err = f.charRepo.AddFriend(ctx, realmID, playerGUID, friendLowGUID, note)
	if err != nil {
		return &AddFriendResult{Result: FriendResultDBError}, err
	}

	// Get friend's online status
	result := &AddFriendResult{FriendGUID: friendLowGUID, Accepted: true}
	if onlineInfo, ok := f.onlineCache.GetOnlineInfo(realmID, friendLowGUID); ok {
		result.Result = FriendResultAddedOnline
		result.Status = 1 // online
		result.Area = onlineInfo.Area
		result.Level = onlineInfo.Level
		result.ClassID = onlineInfo.Class
	} else {
		result.Result = FriendResultAddedOffline
		result.Status = 0 // offline
	}

	// Publish event
	_ = f.eventsProducer.FriendAdded(&events.FriendEventAddedPayload{
		ServiceID:  charserver.ServiceID,
		RealmID:    realmID,
		PlayerGUID: playerGUID,
		FriendGUID: friendLowGUID,
		FriendName: friendName,
		Note:       note,
	})

	return result, nil
}

func (f *friendsServiceImpl) AddRealIDFriendByEmail(ctx context.Context, realmID uint32, playerGUID uint64, accountID uint32, email, note string) (*AddFriendResult, error) {
	if f.accountRepo == nil {
		return &AddFriendResult{Result: FriendResultNotFound}, nil
	}

	if !authidentity.IsValidEmail(email) {
		return &AddFriendResult{Result: FriendResultNotFound}, nil
	}
	email = authidentity.NormalizeLoginIdentity(email)

	player, err := f.charRepo.CharacterByGUID(ctx, realmID, playerGUID)
	if err != nil {
		return &AddFriendResult{Result: FriendResultDBError}, err
	}
	if player == nil {
		return &AddFriendResult{Result: FriendResultNotFound}, nil
	}
	accountID = player.AccountID

	target, err := f.accountRepo.AccountByEmail(ctx, email)
	if err != nil {
		return &AddFriendResult{Result: FriendResultDBError}, err
	}
	if target == nil {
		return &AddFriendResult{Result: FriendResultNotFound}, nil
	}
	if target.ID == accountID {
		return &AddFriendResult{Result: FriendResultSelf}, nil
	}

	if full, err := f.realIDFriendListFull(ctx, realmID, playerGUID, accountID); err != nil {
		return &AddFriendResult{Result: FriendResultDBError}, err
	} else if full {
		return &AddFriendResult{Result: FriendResultListFull}, nil
	}

	relation, err := f.accountRepo.RequestRealIDFriend(ctx, accountID, target.ID, note)
	if err != nil {
		return &AddFriendResult{Result: FriendResultDBError}, err
	}

	result, err := f.realIDAddResult(ctx, realmID, player, relation)
	if err != nil {
		return &AddFriendResult{Result: FriendResultDBError}, err
	}
	return result, nil
}

func (f *friendsServiceImpl) AcceptRealIDFriend(ctx context.Context, realmID uint32, playerGUID uint64, accountID uint32, requesterAccountID uint32, note string) (*AddFriendResult, error) {
	if f.accountRepo == nil {
		return &AddFriendResult{Result: FriendResultNotFound}, nil
	}

	player, err := f.charRepo.CharacterByGUID(ctx, realmID, playerGUID)
	if err != nil {
		return &AddFriendResult{Result: FriendResultDBError}, err
	}
	if player == nil {
		return &AddFriendResult{Result: FriendResultNotFound}, nil
	}
	accountID = player.AccountID

	relation, err := f.accountRepo.AcceptRealIDFriend(ctx, accountID, requesterAccountID, note)
	if err != nil {
		return &AddFriendResult{Result: FriendResultDBError}, err
	}
	if relation == nil || relation.Status != repo.RealIDFriendStatusAccepted {
		return &AddFriendResult{Result: FriendResultNotFound}, nil
	}

	return f.realIDAddResult(ctx, realmID, player, relation)
}

func (f *friendsServiceImpl) AreRealIDFriends(ctx context.Context, accountID uint32, friendAccountID uint32) (bool, error) {
	if f.accountRepo == nil || accountID == 0 || friendAccountID == 0 || accountID == friendAccountID {
		return false, nil
	}

	relations, err := f.accountRepo.AcceptedRealIDFriends(ctx, accountID)
	if err != nil {
		return false, err
	}

	for _, relation := range relations {
		if relation.FriendAccountID == friendAccountID && relation.Status == repo.RealIDFriendStatusAccepted {
			return true, nil
		}
	}

	return false, nil
}

func (f *friendsServiceImpl) RemoveFriend(ctx context.Context, realmID uint32, playerGUID, friendGUID uint64) error {
	friendRealmID := wowguid.PlayerRealmIDOrDefault(realmID, friendGUID)
	friendLowGUID := wowguid.PlayerLowGUID(friendGUID)

	if friendRealmID == realmID {
		if err := f.charRepo.RemoveFriend(ctx, realmID, playerGUID, friendLowGUID); err != nil {
			return err
		}
	}

	if err := f.removeRealIDFriendForCharacterGUID(ctx, realmID, playerGUID, friendRealmID, friendLowGUID); err != nil {
		return err
	}

	// Publish event
	_ = f.eventsProducer.FriendRemoved(&events.FriendEventRemovedPayload{
		ServiceID:  charserver.ServiceID,
		RealmID:    realmID,
		PlayerGUID: playerGUID,
		FriendGUID: wowguid.PlayerGUIDForRealm(realmID, friendRealmID, friendLowGUID),
	})

	return nil
}

func (f *friendsServiceImpl) SetFriendNote(ctx context.Context, realmID uint32, playerGUID, friendGUID uint64, note string) error {
	friendRealmID := wowguid.PlayerRealmIDOrDefault(realmID, friendGUID)
	friendLowGUID := wowguid.PlayerLowGUID(friendGUID)

	if friendRealmID == realmID {
		if err := f.charRepo.UpdateFriendNote(ctx, realmID, playerGUID, friendLowGUID, note); err != nil {
			return err
		}
	}

	if err := f.updateRealIDFriendNoteForCharacterGUID(ctx, realmID, playerGUID, friendRealmID, friendLowGUID, note); err != nil {
		return err
	}

	// Publish event
	_ = f.eventsProducer.NoteUpdated(&events.FriendEventNoteUpdatePayload{
		ServiceID:  charserver.ServiceID,
		RealmID:    realmID,
		PlayerGUID: playerGUID,
		FriendGUID: wowguid.PlayerGUIDForRealm(realmID, friendRealmID, friendLowGUID),
		Note:       note,
	})

	return nil
}

func (f *friendsServiceImpl) AddIgnore(ctx context.Context, realmID uint32, playerGUID, ignoredGUID uint64) (uint32, error) {
	// Cannot ignore self
	if playerGUID == ignoredGUID {
		return FriendResultIgnoreSelf, nil
	}

	// Check if already ignored
	entries, err := f.charRepo.GetFriendsForPlayer(ctx, realmID, playerGUID)
	if err != nil {
		return FriendResultDBError, err
	}

	ignoreCount := 0
	for _, entry := range entries {
		if entry.Flags == repo.SocialFlagIgnore {
			if entry.FriendGUID == ignoredGUID {
				return FriendResultIgnoreAlready, nil
			}
			ignoreCount++
		}
	}

	// Check ignore list limit
	if ignoreCount >= MaxIgnoreLimit {
		return FriendResultIgnoreFull, nil
	}

	// Add to ignore list
	err = f.charRepo.AddIgnore(ctx, realmID, playerGUID, ignoredGUID)
	if err != nil {
		return FriendResultDBError, err
	}

	return FriendResultIgnoreAdded, nil
}

func (f *friendsServiceImpl) RemoveIgnore(ctx context.Context, realmID uint32, playerGUID, ignoredGUID uint64) error {
	return f.charRepo.RemoveIgnore(ctx, realmID, playerGUID, ignoredGUID)
}

func (f *friendsServiceImpl) NotifyStatusChange(ctx context.Context, realmID uint32, playerGUID uint64, status uint8, area, level, classID uint32) error {
	playerGUID = wowguid.PlayerLowGUID(playerGUID)

	// Get players who have this player as friend
	notifyPlayers, err := f.charRepo.GetPlayersWhoHaveAsFriend(ctx, realmID, playerGUID)
	if err != nil {
		return err
	}

	if len(notifyPlayers) > 0 {
		if err := f.eventsProducer.StatusChange(&events.FriendEventStatusChangePayload{
			ServiceID:     charserver.ServiceID,
			RealmID:       realmID,
			PlayerGUID:    playerGUID,
			Status:        status,
			Area:          area,
			Level:         level,
			ClassID:       classID,
			NotifyPlayers: notifyPlayers,
		}); err != nil {
			return err
		}
	}

	return f.notifyRealIDFriends(ctx, realmID, playerGUID, status, area, level, classID)
}

func (f *friendsServiceImpl) realIDFriendListFull(ctx context.Context, realmID uint32, playerGUID uint64, accountID uint32) (bool, error) {
	entries, err := f.charRepo.GetFriendsForPlayer(ctx, realmID, playerGUID)
	if err != nil {
		return false, err
	}
	friendCount := 0
	for _, entry := range entries {
		if entry.Flags == repo.SocialFlagFriend {
			friendCount++
		}
	}

	if f.accountRepo != nil {
		realIDFriends, err := f.accountRepo.AcceptedRealIDFriends(ctx, accountID)
		if err != nil {
			return false, err
		}
		friendCount += len(realIDFriends)
	}

	return friendCount >= MaxFriendsLimit, nil
}

func (f *friendsServiceImpl) realIDAddResult(ctx context.Context, realmID uint32, player *repo.Character, relation *repo.RealIDFriendRelation) (*AddFriendResult, error) {
	if relation == nil {
		return &AddFriendResult{Result: FriendResultNotFound}, nil
	}

	friend, err := f.realIDFriendInfo(ctx, realmID, player, relation)
	if err != nil {
		return nil, err
	}
	if friend == nil {
		return &AddFriendResult{Result: FriendResultNotFound}, nil
	}

	result := &AddFriendResult{
		FriendGUID: friend.GUID,
		Status:     friend.Status,
		Area:       friend.Area,
		Level:      friend.Level,
		ClassID:    friend.ClassID,
		Pending:    relation.Status == repo.RealIDFriendStatusPending,
		Accepted:   relation.Status == repo.RealIDFriendStatusAccepted,
	}
	if friend.Status > 0 && result.Accepted {
		result.Result = FriendResultAddedOnline
	} else {
		result.Result = FriendResultAddedOffline
	}
	if result.Accepted {
		_ = f.eventsProducer.FriendAdded(&events.FriendEventAddedPayload{
			ServiceID:  charserver.ServiceID,
			RealmID:    realmID,
			PlayerGUID: player.CharGUID,
			FriendGUID: friend.GUID,
			FriendName: "",
			Note:       friend.Note,
		})
		_ = f.notifyRealIDAccountAboutPlayer(ctx, player, relation.FriendAccountID)
	}
	return result, nil
}

func (f *friendsServiceImpl) realIDFriendInfo(ctx context.Context, viewerRealmID uint32, viewer *repo.Character, relation *repo.RealIDFriendRelation) (*FriendInfo, error) {
	if onlineInfo, ok := f.representativeOnlineInfoForAccount(viewerRealmID, relation.FriendAccountID); ok {
		if !f.realIDCrossFaction && !sameFaction(uint32(viewer.CharRace), onlineInfo.Race) {
			return nil, nil
		}
		friendRealmID := onlineInfo.RealmID
		return &FriendInfo{
			RealmID: friendRealmID,
			GUID:    wowguid.PlayerGUIDForRealm(viewerRealmID, friendRealmID, onlineInfo.GUID),
			Note:    relation.Note,
			Status:  1,
			Area:    onlineInfo.Area,
			Level:   onlineInfo.Level,
			ClassID: onlineInfo.Class,
		}, nil
	}

	displayChar, err := f.charRepo.DisplayCharacterByAccount(ctx, relation.FriendAccountID)
	if err != nil {
		return nil, err
	}
	if displayChar == nil {
		return nil, nil
	}
	if !f.realIDCrossFaction && !sameFaction(uint32(viewer.CharRace), uint32(displayChar.CharRace)) {
		return nil, nil
	}

	return &FriendInfo{
		RealmID: displayChar.RealmID,
		GUID:    wowguid.PlayerGUIDForRealm(viewerRealmID, displayChar.RealmID, displayChar.CharGUID),
		Note:    relation.Note,
		Status:  0,
	}, nil
}

func (f *friendsServiceImpl) representativeOnlineInfoForAccount(viewerRealmID uint32, accountID uint32) (OnlinePlayerInfo, bool) {
	infos := f.onlineCache.GetOnlineInfosForAccount(accountID)
	if len(infos) == 0 {
		return OnlinePlayerInfo{}, false
	}

	sort.Slice(infos, func(i, j int) bool {
		leftSameRealm := infos[i].RealmID == viewerRealmID
		rightSameRealm := infos[j].RealmID == viewerRealmID
		if leftSameRealm != rightSameRealm {
			return leftSameRealm
		}
		if infos[i].RealmID != infos[j].RealmID {
			return infos[i].RealmID < infos[j].RealmID
		}
		return infos[i].GUID < infos[j].GUID
	})

	return infos[0], true
}

func (f *friendsServiceImpl) removeRealIDFriendForCharacterGUID(ctx context.Context, playerRealmID uint32, playerGUID uint64, friendRealmID uint32, friendGUID uint64) error {
	if f.accountRepo == nil {
		return nil
	}

	player, err := f.charRepo.CharacterByGUID(ctx, playerRealmID, playerGUID)
	if err != nil || player == nil {
		return err
	}
	friend, err := f.charRepo.CharacterByGUID(ctx, friendRealmID, friendGUID)
	if err != nil || friend == nil {
		return err
	}

	return f.accountRepo.RemoveRealIDFriend(ctx, player.AccountID, friend.AccountID)
}

func (f *friendsServiceImpl) updateRealIDFriendNoteForCharacterGUID(ctx context.Context, playerRealmID uint32, playerGUID uint64, friendRealmID uint32, friendGUID uint64, note string) error {
	if f.accountRepo == nil {
		return nil
	}

	player, err := f.charRepo.CharacterByGUID(ctx, playerRealmID, playerGUID)
	if err != nil || player == nil {
		return err
	}
	friend, err := f.charRepo.CharacterByGUID(ctx, friendRealmID, friendGUID)
	if err != nil || friend == nil {
		return err
	}

	return f.accountRepo.UpdateRealIDFriendNote(ctx, player.AccountID, friend.AccountID, note)
}

func (f *friendsServiceImpl) notifyRealIDFriends(ctx context.Context, realmID uint32, playerGUID uint64, status uint8, area, level, classID uint32) error {
	if f.accountRepo == nil {
		return nil
	}

	player, err := f.charRepo.CharacterByGUID(ctx, realmID, playerGUID)
	if err != nil || player == nil {
		return err
	}

	realIDFriends, err := f.accountRepo.AcceptedRealIDFriends(ctx, player.AccountID)
	if err != nil {
		return err
	}

	for _, relation := range realIDFriends {
		listeners := f.onlineCache.GetOnlineInfosForAccount(relation.FriendAccountID)
		for _, listener := range listeners {
			if !f.realIDCrossFaction && !sameFaction(uint32(player.CharRace), listener.Race) {
				continue
			}
			if err := f.eventsProducer.StatusChange(&events.FriendEventStatusChangePayload{
				ServiceID:     charserver.ServiceID,
				RealmID:       listener.RealmID,
				PlayerGUID:    wowguid.PlayerGUIDForRealm(listener.RealmID, realmID, playerGUID),
				Status:        status,
				Area:          area,
				Level:         level,
				ClassID:       classID,
				NotifyPlayers: []uint64{listener.GUID},
			}); err != nil {
				return err
			}
		}
	}

	return nil
}

func (f *friendsServiceImpl) notifyRealIDAccountAboutPlayer(ctx context.Context, player *repo.Character, accountID uint32) error {
	if player == nil {
		return nil
	}

	status := uint8(0)
	area := player.CharZone
	level := uint32(player.CharLevel)
	classID := uint32(player.CharClass)
	if onlineInfo, ok := f.onlineCache.GetOnlineInfo(player.RealmID, player.CharGUID); ok {
		status = onlineInfo.Status
		area = onlineInfo.Area
		level = onlineInfo.Level
		classID = onlineInfo.Class
	}

	for _, listener := range f.onlineCache.GetOnlineInfosForAccount(accountID) {
		if !f.realIDCrossFaction && !sameFaction(uint32(player.CharRace), listener.Race) {
			continue
		}
		if err := f.eventsProducer.StatusChange(&events.FriendEventStatusChangePayload{
			ServiceID:     charserver.ServiceID,
			RealmID:       listener.RealmID,
			PlayerGUID:    wowguid.PlayerGUIDForRealm(listener.RealmID, player.RealmID, player.CharGUID),
			Status:        status,
			Area:          area,
			Level:         level,
			ClassID:       classID,
			NotifyPlayers: []uint64{listener.GUID},
		}); err != nil {
			return err
		}
	}

	return nil
}

func sameFaction(leftRace uint32, rightRace uint32) bool {
	return isAllianceRace(leftRace) == isAllianceRace(rightRace)
}

func isAllianceRace(race uint32) bool {
	switch race {
	case 1, 3, 4, 7, 11:
		return true
	default:
		return false
	}
}
