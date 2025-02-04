package events

import (
	"fmt"
	"github.com/walkline/ToCloud9/shared/wow/guid"
)

// MatchmakingServiceEvent is event type that matchmaking service generates
type MatchmakingServiceEvent int

const (
	// MatchmakingEventPlayersQueued matchmaking event for battleground and arena queue when players joined queue
	MatchmakingEventPlayersQueued MatchmakingServiceEvent = iota + 1

	// MatchmakingEventPlayersInvited matchmaking event for battleground and arena queue when players invited to bg or arena.
	MatchmakingEventPlayersInvited

	// MatchmakingEventPlayersInviteExpired matchmaking event for battleground and arena queue when players invite expired.
	MatchmakingEventPlayersInviteExpired
)

// SubjectName is key that nats uses
func (e MatchmakingServiceEvent) SubjectName() string {
	switch e {
	case MatchmakingEventPlayersQueued:
		return "matchmaking.pvpqueue.joined"
	case MatchmakingEventPlayersInvited:
		return "matchmaking.pvpqueue.invited"
	case MatchmakingEventPlayersInviteExpired:
		return "matchmaking.pvpqueue.invite.expired"
	}
	panic(fmt.Errorf("unk event %d", e))
}

// MatchmakingEventPlayersQueuedPayload represents payload of MatchmakingEventPlayersQueued event
type MatchmakingEventPlayersQueuedPayload struct {
	RealmID uint32

	PlayersGUID       []guid.LowType
	QueueSlotByPlayer map[guid.LowType]uint8

	// TODO: arenas not supported yet
	ArenaType uint8 // ??
	IsRated   bool

	PVPQueueMinLVL uint8
	PVPQueueMaxLVL uint8

	TypeID uint8

	AverageWaitingTimeMilliseconds uint32
}

// MatchmakingEventPlayersInvitedPayload represents payload of MatchmakingEventPlayersInvited event
type MatchmakingEventPlayersInvitedPayload struct {
	RealmID uint32

	PlayersGUID       []guid.LowType
	QueueSlotByPlayer map[guid.LowType]uint8

	// TODO: arenas not supported yet
	ArenaType uint8 // ??
	IsRated   bool

	PVPQueueMinLVL uint8
	PVPQueueMaxLVL uint8

	TypeID uint8

	MapID uint32

	TimeToAcceptMilliseconds uint32
}

type MatchmakingEventPlayersInviteExpiredPayload struct {
	RealmID uint32

	PlayersGUID       []guid.LowType
	QueueSlotByPlayer map[guid.LowType]uint8
}
