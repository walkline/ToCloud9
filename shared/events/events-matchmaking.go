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

	// MatchmakingEventLfgStatusChanged matchmaking event for LFG queue, role check, and proposal state changes.
	MatchmakingEventLfgStatusChanged

	// MatchmakingEventLfgProposalAccepted matchmaking event emitted when every LFG proposal member accepts.
	MatchmakingEventLfgProposalAccepted
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
	case MatchmakingEventLfgStatusChanged:
		return "matchmaking.lfg.status.changed"
	case MatchmakingEventLfgProposalAccepted:
		return "matchmaking.lfg.proposal.accepted"
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

type MatchmakingLfgState uint8

const (
	MatchmakingLfgStateNone MatchmakingLfgState = iota
	MatchmakingLfgStateRoleCheck
	MatchmakingLfgStateQueued
	MatchmakingLfgStateProposal
	MatchmakingLfgStateBoot
	MatchmakingLfgStateDungeon
	MatchmakingLfgStateFinishedDungeon
	MatchmakingLfgStateRaidBrowser
)

type MatchmakingLfgProposalState uint8

const (
	MatchmakingLfgProposalInitiating MatchmakingLfgProposalState = iota
	MatchmakingLfgProposalFailed
	MatchmakingLfgProposalSuccess
)

type MatchmakingLfgProposalFailure uint8

const (
	MatchmakingLfgProposalFailureNone MatchmakingLfgProposalFailure = iota
	MatchmakingLfgProposalFailureFailed
	MatchmakingLfgProposalFailureDeclined
)

type MatchmakingLfgMember struct {
	RealmID            uint32
	PlayerGUID         guid.LowType
	Roles              uint8
	Leader             bool
	QueueLeaderRealmID uint32
	QueueLeaderGUID    guid.LowType
}

type MatchmakingLfgProposalMember struct {
	RealmID       uint32
	PlayerGUID    guid.LowType
	SelectedRoles uint8
	AssignedRole  uint8
	Answered      bool
	Accepted      bool
}

type MatchmakingLfgStatusPayload struct {
	State            MatchmakingLfgState
	ProposalID       uint32
	ProposalState    MatchmakingLfgProposalState
	ProposalFailure  MatchmakingLfgProposalFailure
	DungeonEntry     uint32
	SelectedDungeons []uint32
	QueuedMembers    []MatchmakingLfgMember
	ProposalMembers  []MatchmakingLfgProposalMember

	QueuedTimeMilliseconds uint32
	TanksNeeded            uint8
	HealersNeeded          uint8
	DamageNeeded           uint8
}

type MatchmakingEventLfgStatusChangedPayload struct {
	RealmID     uint32
	PlayersGUID []guid.LowType
	Status      MatchmakingLfgStatusPayload
}

type MatchmakingLfgProposalAcceptedMember struct {
	RealmID            uint32
	PlayerGUID         guid.LowType
	SelectedRoles      uint8
	AssignedRole       uint8
	QueueLeaderRealmID uint32
	QueueLeaderGUID    guid.LowType
	WorldserverID      string
}

type MatchmakingEventLfgProposalAcceptedPayload struct {
	RealmID             uint32
	LeaderRealmID       uint32
	BattlegroupID       uint32
	CrossRealm          bool
	ProposalID          uint32
	GroupID             uint32
	DungeonEntry        uint32
	LeaderGUID          guid.LowType
	LeaderWorldserverID string
	PlayersGUID         []guid.LowType
	Members             []MatchmakingLfgProposalAcceptedMember
}
