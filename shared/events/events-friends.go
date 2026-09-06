package events

import "fmt"

type FriendsServiceEvent int

const (
	FriendEventStatusChange FriendsServiceEvent = iota + 1
	FriendEventAdded
	FriendEventRemoved
	FriendEventNoteUpdate
)

func (e FriendsServiceEvent) SubjectName() string {
	switch e {
	case FriendEventStatusChange:
		return "friends.status.changed"
	case FriendEventAdded:
		return "friends.friend.added"
	case FriendEventRemoved:
		return "friends.friend.removed"
	case FriendEventNoteUpdate:
		return "friends.note.updated"
	}
	panic(fmt.Errorf("unknown event %d", e))
}

type FriendEventStatusChangePayload struct {
	ServiceID  string
	RealmID    uint32
	PlayerGUID uint64
	Status     uint8 // 0=offline, 1=online
	Area       uint32
	Level      uint32
	ClassID    uint32
	// List of players who have this player as friend
	NotifyPlayers []uint64
}

type FriendEventAddedPayload struct {
	ServiceID  string
	RealmID    uint32
	PlayerGUID uint64
	FriendGUID uint64
	FriendName string
	Note       string
}

type FriendEventRemovedPayload struct {
	ServiceID  string
	RealmID    uint32
	PlayerGUID uint64
	FriendGUID uint64
}

type FriendEventNoteUpdatePayload struct {
	ServiceID  string
	RealmID    uint32
	PlayerGUID uint64
	FriendGUID uint64
	Note       string
}
