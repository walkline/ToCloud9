package events

import "fmt"

// GroupServiceEvent is event type that group service generates
type GroupServiceEvent int

const (
	// GroupEventInviteCreated group event when player invite created
	GroupEventInviteCreated GroupServiceEvent = iota + 1

	// GroupEventGroupCreated group event when group created
	GroupEventGroupCreated

	// GroupEventGroupMemberOnlineStatusChanged group event when group member cam online or offline
	GroupEventGroupMemberOnlineStatusChanged

	// GroupEventGroupMemberLeft group event when group member leaves
	GroupEventGroupMemberLeft

	// GroupEventGroupDisband group event when group is disbanded
	GroupEventGroupDisband

	// GroupEventGroupMemberAdded group event when a member is added to the group
	GroupEventGroupMemberAdded

	// GroupEventGroupLeaderChanged group event when the group leader is changed
	GroupEventGroupLeaderChanged

	// GroupEventGroupLootTypeChanged group event when the loot type is changed
	GroupEventGroupLootTypeChanged

	// GroupEventGroupConvertedToRaid group event when the group is converted to a raid
	GroupEventGroupConvertedToRaid
)

// SubjectName is key that nats uses
func (e GroupServiceEvent) SubjectName() string {
	switch e {
	case GroupEventInviteCreated:
		return "group.invite.created"
	case GroupEventGroupCreated:
		return "group.created"
	case GroupEventGroupMemberOnlineStatusChanged:
		return "group.member.online.changed"
	case GroupEventGroupMemberLeft:
		return "group.member.left"
	case GroupEventGroupDisband:
		return "group.disband"
	case GroupEventGroupMemberAdded:
		return "group.member.added"
	case GroupEventGroupLeaderChanged:
		return "group.leader.changed"
	case GroupEventGroupLootTypeChanged:
		return "group.loot.changed"
	case GroupEventGroupConvertedToRaid:
		return "group.converted.raid"
	}
	panic(fmt.Errorf("unk event %d", e))
}

type GroupEventInviteCreatedPayload struct {
	// ServiceID is identifier of guild service
	ServiceID string
	RealmID   uint32

	GroupID uint

	InviterGUID uint64
	InviterName string

	InviteeGUID uint64
	InviteeName string
}

type GroupEventGroupCreatedPayload struct {
	ServiceID string
	RealmID   uint32

	GroupID          uint
	LeaderGUID       uint64
	LootMethod       uint8
	LooterGUID       uint64
	LootThreshold    uint8
	GroupType        uint8
	Difficulty       uint8
	RaidDifficulty   uint8
	MasterLooterGuid uint64
	Members          []GroupMember
}

type GroupEventGroupMemberOnlineStatusChangedPayload struct {
	ServiceID string
	RealmID   uint32

	GroupID    uint
	MemberGUID uint64
	IsOnline   bool

	OnlineMembers []uint64
}

type GroupEventGroupMemberLeftPayload struct {
	ServiceID string
	RealmID   uint32

	GroupID     uint
	MemberGUID  uint64
	MemberName  string
	NewLeaderID uint64 // If the leaving member was the leader, specify the new leader ID

	OnlineMembers []uint64
}

type GroupEventGroupMemberAddedPayload struct {
	ServiceID string
	RealmID   uint32

	GroupID    uint
	MemberGUID uint64
	MemberName string

	OnlineMembers []uint64
}

type GroupEventGroupLeaderChangedPayload struct {
	ServiceID string
	RealmID   uint32

	GroupID        uint
	PreviousLeader uint64
	NewLeader      uint64

	OnlineMembers []uint64
}

type GroupEventGroupLootTypeChangedPayload struct {
	ServiceID string
	RealmID   uint32

	GroupID     uint
	Leader      uint64
	NewLootType uint8

	OnlineMembers []uint64
}

type GroupEventGroupConvertedToRaidPayload struct {
	ServiceID string
	RealmID   uint32

	GroupID uint
	Leader  uint64

	OnlineMembers []uint64
}

type GroupEventGroupDisbandPayload struct {
	ServiceID string
	RealmID   uint32

	GroupID       uint
	OnlineMembers []uint64
}

type GroupMember struct {
	MemberGUID  uint64
	MemberFlags uint8
	MemberName  string
	IsOnline    bool
	SubGroup    uint8
	Roles       uint8
}
