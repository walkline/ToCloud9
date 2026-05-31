package repo

import (
	"context"

	"github.com/walkline/ToCloud9/shared/wow/guid"
)

const MaxTargetIcons = 8

type GroupTypeFlags uint8

const (
	// Mirrors AzerothCore GroupType flags in src/server/game/Groups/Group.h.
	GroupTypeFlagsNormal        GroupTypeFlags = 0x00
	GroupTypeFlagsBG            GroupTypeFlags = 0x01
	GroupTypeFlagsRaid          GroupTypeFlags = 0x02
	GroupTypeFlagsLFGRestricted GroupTypeFlags = 0x04
	GroupTypeFlagsLFG           GroupTypeFlags = 0x08
)

const (
	MaxGroupSize = 5
	MaxRaidSize  = 40
)

type LootType uint8

const (
	LootTypeFreeForAll LootType = iota
	LootTypeRoundRobin
	LootTypeMasterLoot
	LootTypeGroupLoot
	LootTypeNeedBeforeGreed
)

type ItemQuality uint8

const (
	ItemQualityPoor ItemQuality = iota
	ItemQualityNormal
	ItemQualityUncommon
	ItemQualityRare
	ItemQualityEpic
	ItemQualityLegendary
	ItemQualityArtifact
)

type Group struct {
	ID               uint
	RealmID          uint32
	LeaderGUID       uint64
	LootMethod       uint8
	LooterGUID       uint64
	LootThreshold    uint8
	TargetIcons      [MaxTargetIcons]uint64
	GroupType        GroupTypeFlags
	Difficulty       uint8
	RaidDifficulty   uint8
	MasterLooterGuid uint64
	LfgDungeonEntry  uint32

	Members []GroupMember
}

func (g *Group) MemberByGUID(playerGUID uint64) *GroupMember {
	for i := range g.Members {
		if guid.SamePlayer(g.RealmID, g.Members[i].MemberGUID, g.RealmID, playerGUID) {
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

func (g *Group) IsLFG() bool {
	return g.GroupType&GroupTypeFlagsLFG > 0
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
	MemberFlagAssistant uint8 = 1 << iota
	MemberFlagMainTank
	MemberFlagMainAssistant
)

const (
	RoleFlagsAssistant RoleFlags = 1 << iota
	RoleFlagsMainTank
	RoleFlagsMainAssistant
)

type GroupMember struct {
	GroupID     uint
	RealmID     uint32
	MemberGUID  uint64
	MemberFlags uint8
	MemberName  string
	IsOnline    bool
	SubGroup    uint8
	Roles       RoleFlags
}

func (m GroupMember) IsAssistant() bool {
	return m.MemberFlags&MemberFlagAssistant > 0
}

type GroupInvite struct {
	Inviter        uint64
	InviterRealmID uint32
	InviterName    string

	Invitee        uint64
	InviteeRealmID uint32
	InviteeName    string

	GroupID      uint
	GroupRealmID uint32
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
	RemoveInvite(ctx context.Context, realmID uint32, invitedPlayer uint64) error
}
