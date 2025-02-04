package repo

import "context"

// Guild represents in game guild.
type Guild struct {
	RealmID         uint32
	ID              uint64
	Name            string
	CrateTimeUnix   int64
	LeaderGUID      uint64
	Emblem          GuildEmblem
	Info            string
	MessageOfTheDay string
	BankMoney       uint64

	GuildRanks   []GuildRank
	GuildMembers []*GuildMember
}

// GuildEmblem represents emblem of the guild.
type GuildEmblem struct {
	Style           uint8
	Color           uint8
	BorderStyle     uint8
	BorderColor     uint8
	BackgroundColor uint8
}

// GuildRankDefaultID is default list of guild ranks.
// These ranks can be modified, but they cannot be deleted.
// When promoting member server does: rank--
// When demoting member server does: rank++
type GuildRankDefaultID uint8

const (
	GuildRankGuildMaster GuildRankDefaultID = iota
	GuildRankOfficer
	GuildRankVeteran
	GuildRankMember
	GuildRankInitiate

	GuildRankMax = 10
)

// GuildRank represents ranks of the guild.
// Guild has limited amount of ranks - 10.
// By default guild has ranks from GuildRankDefaultID.
type GuildRank struct {
	GuildID     uint64
	Rank        uint8
	Name        string
	Rights      uint32
	MoneyPerDay uint32
}

func (r *GuildRank) HasRight(right uint32) bool {
	return (r.Rights & right) != RightEmpty
}

const (
	RightEmpty              = 0x00000040
	RightChatListen         = RightEmpty | 0x00000001
	RightChatSpeak          = RightEmpty | 0x00000002
	RightOfficerChatListen  = RightEmpty | 0x00000004
	RightOfficerChatSpeak   = RightEmpty | 0x00000008
	RightInvite             = RightEmpty | 0x00000010
	RightRemove             = RightEmpty | 0x00000020
	RightPromote            = RightEmpty | 0x00000080
	RightDemote             = RightEmpty | 0x00000100
	RightSetMessageOfTheDay = RightEmpty | 0x00001000
	RightEditPublicNote     = RightEmpty | 0x00002000
	RightViewOfficersNote   = RightEmpty | 0x00004000
	RightEditOfficersNote   = RightEmpty | 0x00008000
	RightModifyGuildInfo    = RightEmpty | 0x00010000
	RightWithdrawGoldLock   = 0x00020000
	RightWithdrawRepair     = 0x00040000
	RightWithdrawGold       = 0x00080000
	RightCreateGuildEvent   = 0x00100000
	RightAll                = 0x001DF1FF
)

// GuildMemberStatus is the status of guild member.
type GuildMemberStatus uint8

const (
	GuildMemberStatusOffline GuildMemberStatus = iota
	GuildMemberStatusOnline
)

// GuildMember represents member of the guild.
type GuildMember struct {
	GuildID     uint64
	PlayerGUID  uint64
	Rank        uint8
	PublicNote  string
	OfficerNote string
	Name        string
	Race        uint8
	Class       uint8
	Lvl         uint8
	Gender      uint8
	AreaID      uint32
	Account     uint64
	LogoutTime  int64
	Status      GuildMemberStatus
}

// GuildsRepo represents repository for Guilds.
//
//go:generate mockery --name=GuildsRepo --filename=guilds-repo.go
type GuildsRepo interface {
	// LoadAllForRealm loads all guilds for realm.
	// Can be time-consuming, better to use it on startup to warmup cache.
	LoadAllForRealm(ctx context.Context, realmID uint32) (map[uint64]*Guild, error)

	// GuildByRealmAndID loads guild by realm and id.
	GuildByRealmAndID(ctx context.Context, realmID uint32, guildID uint64) (*Guild, error)

	// GuildIDByRealmAndMemberGUID returns guild id by guild member GUID.
	GuildIDByRealmAndMemberGUID(ctx context.Context, realmID uint32, memberGUID uint64) (uint64, error)

	// AddGuildInvite links user invite to a specific guild.
	AddGuildInvite(ctx context.Context, realmID uint32, charGUID, guildID uint64) error

	// GuildIDByCharInvite returns guild id by invited character.
	GuildIDByCharInvite(ctx context.Context, realmID uint32, charGUID uint64) (uint64, error)

	// RemoveGuildInviteForCharacter removes guild invite by character.
	RemoveGuildInviteForCharacter(ctx context.Context, realmID uint32, charGUID uint64) error

	// AddGuildMember adds guild member to the guild.
	AddGuildMember(ctx context.Context, realmID uint32, member GuildMember) error

	// RemoveGuildMember removes guild member from the guild.
	RemoveGuildMember(ctx context.Context, realmID uint32, characterGUID uint64) error

	// SetMessageOfTheDay updates message of the day for the guild.
	SetMessageOfTheDay(ctx context.Context, realmID uint32, guildID uint64, message string) error

	// SetMemberPublicNote sets public not for guild member.
	SetMemberPublicNote(ctx context.Context, realmID uint32, memberGUID uint64, note string) error

	// SetMemberOfficerNote sets officer not for guild member.
	SetMemberOfficerNote(ctx context.Context, realmID uint32, memberGUID uint64, note string) error

	// SetMemberRank sets rank for the guild member.
	SetMemberRank(ctx context.Context, realmID uint32, memberGUID uint64, rank uint8) error

	// SetGuildInfo updates guild info text of the guild.
	SetGuildInfo(ctx context.Context, realmID uint32, guildID uint64, info string) error

	// UpdateGuildRank updates guild rank.
	UpdateGuildRank(ctx context.Context, realmID uint32, guildID uint64, rank uint8, name string, rights, moneyPerDay uint32) error

	// AddGuildRank adds guild rank.
	AddGuildRank(ctx context.Context, realmID uint32, guildID uint64, rank uint8, name string, rights, moneyPerDay uint32) error

	// DeleteLowestGuildRank deletes lowes guild rank.
	DeleteLowestGuildRank(ctx context.Context, realmID uint32, guildID uint64, rank uint8) error
}
