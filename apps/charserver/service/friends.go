package service

import (
	"context"
	"errors"

	"github.com/walkline/ToCloud9/apps/charserver"
	"github.com/walkline/ToCloud9/apps/charserver/repo"
	"github.com/walkline/ToCloud9/shared/events"
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
	GUID   uint64
	Level  uint32
	Class  uint32
	Area   uint32
	Status uint8
}

type FriendInfo struct {
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
	Result  uint32
	Status  uint8
	Area    uint32
	Level   uint32
	ClassID uint32
}

//go:generate mockery --name=FriendsService
type FriendsService interface {
	GetFriendsList(ctx context.Context, realmID uint32, playerGUID uint64) (*FriendsList, error)
	AddFriend(ctx context.Context, realmID uint32, playerGUID, friendGUID uint64, friendName, note string) (*AddFriendResult, error)
	RemoveFriend(ctx context.Context, realmID uint32, playerGUID, friendGUID uint64) error
	SetFriendNote(ctx context.Context, realmID uint32, playerGUID, friendGUID uint64, note string) error
	AddIgnore(ctx context.Context, realmID uint32, playerGUID, ignoredGUID uint64) (uint32, error)
	RemoveIgnore(ctx context.Context, realmID uint32, playerGUID, ignoredGUID uint64) error

	// NotifyStatusChange is called when player logs in/out
	NotifyStatusChange(ctx context.Context, realmID uint32, playerGUID uint64, status uint8, area, level, classID uint32) error
}

type friendsServiceImpl struct {
	charRepo       repo.Characters
	onlineCache    OnlinePlayersCache
	eventsProducer events.FriendsServiceProducer
}

func NewFriendsService(charRepo repo.Characters, onlineCache OnlinePlayersCache, eventsProducer events.FriendsServiceProducer) FriendsService {
	return &friendsServiceImpl{
		charRepo:       charRepo,
		onlineCache:    onlineCache,
		eventsProducer: eventsProducer,
	}
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

	for _, entry := range entries {
		if entry.Flags == repo.SocialFlagFriend {
			friend := &FriendInfo{
				GUID: entry.FriendGUID,
				Note: entry.Note,
			}

			// Check if friend is online
			if onlineInfo, ok := f.onlineCache.GetOnlineInfo(entry.FriendGUID); ok {
				friend.Status = 1 // online
				friend.Area = onlineInfo.Area
				friend.Level = onlineInfo.Level
				friend.ClassID = onlineInfo.Class
			} else {
				friend.Status = 0 // offline
			}

			result.Friends = append(result.Friends, friend)
		} else if entry.Flags == repo.SocialFlagIgnore {
			result.Ignored = append(result.Ignored, entry.FriendGUID)
		}
	}

	return result, nil
}

func (f *friendsServiceImpl) AddFriend(ctx context.Context, realmID uint32, playerGUID, friendGUID uint64, friendName, note string) (*AddFriendResult, error) {
	// Cannot add self
	if playerGUID == friendGUID {
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
			if entry.FriendGUID == friendGUID {
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
	err = f.charRepo.AddFriend(ctx, realmID, playerGUID, friendGUID, note)
	if err != nil {
		return &AddFriendResult{Result: FriendResultDBError}, err
	}

	// Get friend's online status
	result := &AddFriendResult{}
	if onlineInfo, ok := f.onlineCache.GetOnlineInfo(friendGUID); ok {
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
		FriendGUID: friendGUID,
		FriendName: friendName,
		Note:       note,
	})

	return result, nil
}

func (f *friendsServiceImpl) RemoveFriend(ctx context.Context, realmID uint32, playerGUID, friendGUID uint64) error {
	err := f.charRepo.RemoveFriend(ctx, realmID, playerGUID, friendGUID)
	if err != nil {
		return err
	}

	// Publish event
	_ = f.eventsProducer.FriendRemoved(&events.FriendEventRemovedPayload{
		ServiceID:  charserver.ServiceID,
		RealmID:    realmID,
		PlayerGUID: playerGUID,
		FriendGUID: friendGUID,
	})

	return nil
}

func (f *friendsServiceImpl) SetFriendNote(ctx context.Context, realmID uint32, playerGUID, friendGUID uint64, note string) error {
	err := f.charRepo.UpdateFriendNote(ctx, realmID, playerGUID, friendGUID, note)
	if err != nil {
		return err
	}

	// Publish event
	_ = f.eventsProducer.NoteUpdated(&events.FriendEventNoteUpdatePayload{
		ServiceID:  charserver.ServiceID,
		RealmID:    realmID,
		PlayerGUID: playerGUID,
		FriendGUID: friendGUID,
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
	// Get players who have this player as friend
	notifyPlayers, err := f.charRepo.GetPlayersWhoHaveAsFriend(ctx, realmID, playerGUID)
	if err != nil {
		return err
	}

	if len(notifyPlayers) == 0 {
		return nil
	}

	// Publish status change event
	return f.eventsProducer.StatusChange(&events.FriendEventStatusChangePayload{
		ServiceID:     charserver.ServiceID,
		RealmID:       realmID,
		PlayerGUID:    playerGUID,
		Status:        status,
		Area:          area,
		Level:         level,
		ClassID:       classID,
		NotifyPlayers: notifyPlayers,
	})
}
