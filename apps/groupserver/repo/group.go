package repo

import (
	"context"
)

const MaxTargetIcons = 8

type GroupTypeFlags uint8

const (
	GroupTypeFlagsNormal GroupTypeFlags = iota
	GroupTypeFlagsBG     GroupTypeFlags = 1 << (iota - 1)
	GroupTypeFlagsRaid
)

const (
	MaxGroupSize = 5
	MaxRaidSize  = 40
)

type LootType uint8

const (
	LootTypeFreeForAll LootType = 0
	LootTypeRoundRobin
	LootTypeMasterLoot
	LootTypeGroupLoot
	LootTypeNeedBeforeGreed
)

type ItemQuality uint8

const (
	ItemQualityPoor ItemQuality = 0
	ItemQualityNormal
	ItemQualityUncommon
	ItemQualityRare
	ItemQualityEpic
	ItemQualityLegendary
	ItemQualityArtifact
)

type Group struct {
	ID               uint
	LeaderGUID       uint64
	LootMethod       uint8
	LooterGUID       uint64
	LootThreshold    uint8
	TargetIcons      [MaxTargetIcons]uint64
	GroupType        GroupTypeFlags
	Difficulty       uint8
	RaidDifficulty   uint8
	MasterLooterGuid uint64

	Members []GroupMember
}

func (g *Group) MemberByGUID(guid uint64) *GroupMember {
	for i := range g.Members {
		if g.Members[i].MemberGUID == guid {
			return &g.Members[i]
		}
	}
	return nil
}

func (g *Group) IsFull() bool {
	if g.IsRaid() {
		return len(g.Members) >= MaxRaidSize
	}

	return len(g.Members) >= MaxGroupSize
}

func (g *Group) IsRaid() bool {
	return g.GroupType&GroupTypeFlagsRaid > 0
}

func (g *Group) OnlineMemberGUIDs() []uint64 {
	onlinePlayers := []uint64{}
	for _, member := range g.Members {
		if member.IsOnline {
			onlinePlayers = append(onlinePlayers, member.MemberGUID)
		}
	}
	return onlinePlayers
}

type RoleFlags uint8

const (
	RoleFlagsAssistant RoleFlags = 1 << iota
	RoleFlagsMainTank
	RoleFlagsMainAssistant
)

type GroupMember struct {
	GroupID     uint
	MemberGUID  uint64
	MemberFlags uint8
	MemberName  string
	IsOnline    bool
	SubGroup    uint8
	Roles       RoleFlags
}

func (m GroupMember) IsAssistant() bool {
	return m.Roles&RoleFlagsAssistant > 0
}

type GroupInvite struct {
	Inviter     uint64
	InviterName string

	Invitee     uint64
	InviteeName string

	GroupID uint
}

type GroupsRepo interface {
	// LoadAllForRealm loads all guilds for realm.
	// Can be time-consuming, better to use it on startup to warmup cache.
	LoadAllForRealm(ctx context.Context, realmID uint32) (map[uint]*Group, error)

	GroupByID(ctx context.Context, realmID uint32, partyID uint, loadMembers bool) (*Group, error)
	GroupIDByPlayer(ctx context.Context, realmID uint32, player uint64) (uint, error)

	Create(ctx context.Context, realmID uint32, group *Group) error
	Delete(ctx context.Context, realmID uint32, groupID uint) error
	Update(ctx context.Context, realmID uint32, group *Group) error

	AddMember(ctx context.Context, realmID uint32, groupMember *GroupMember) error
	UpdateMember(ctx context.Context, realmID uint32, groupMember *GroupMember) error
	RemoveMember(ctx context.Context, realmID uint32, memberGUID uint64) error

	AddInvite(ctx context.Context, realmID uint32, invite GroupInvite) error
	GetInviteByInvitedPlayer(ctx context.Context, realmID uint32, invitedPlayer uint64) (*GroupInvite, error)
}
