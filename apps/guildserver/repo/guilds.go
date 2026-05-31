package repo

import (
	"context"
	"errors"
)

// Guild represents in game guild.
type Guild struct {
	RealmID           uint32
	ID                uint64
	Name              string
	CrateTimeUnix     int64
	LeaderGUID        uint64
	Emblem            GuildEmblem
	Info              string
	MessageOfTheDay   string
	BankMoney         uint64
	PurchasedBankTabs uint8

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

	GuildRankMinCount = 5
	GuildRankMax      = 10
)

// GuildRank represents ranks of the guild.
// Guild has limited amount of ranks - 10.
// By default guild has ranks from GuildRankDefaultID.
type GuildRank struct {
	GuildID       uint64
	Rank          uint8
	Name          string
	Rights        uint32
	MoneyPerDay   uint32
	BankTabRights [GuildBankMaxTabs]GuildBankTabRight
}

const (
	GuildBankMaxTabs       = 6
	GuildBankMaxSlots      = 98
	GuildBankWithdrawSlots = GuildBankMaxTabs + 1
	GuildBankMoneyLimit    = 0x7FFFFFFFFFFFF
)

var (
	ErrGuildBankInvalidTab    = errors.New("invalid guild bank tab")
	ErrGuildBankInvalidSlot   = errors.New("invalid guild bank slot")
	ErrGuildBankNotEnoughGold = errors.New("not enough guild bank gold")
	ErrGuildBankFull          = errors.New("guild bank full")
	ErrGuildBankWithdrawLimit = errors.New("guild bank withdraw limit")
	ErrGuildBankItemNotFound  = errors.New("guild bank item not found")
)

type GuildBankTabRight struct {
	TabID             uint8
	Flags             uint32
	WithdrawItemLimit uint32
}

const (
	GuildBankRightViewTab     uint32 = 0x01
	GuildBankRightPutItem     uint32 = 0x02
	GuildBankRightUpdateText  uint32 = 0x04
	GuildBankRightDepositItem        = GuildBankRightViewTab | GuildBankRightPutItem
)

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
	GuildID      uint64
	PlayerGUID   uint64
	Rank         uint8
	PublicNote   string
	OfficerNote  string
	BankWithdraw [GuildBankWithdrawSlots]uint32
	Name         string
	Race         uint8
	Class        uint8
	Lvl          uint8
	Gender       uint8
	AreaID       uint32
	Account      uint64
	LogoutTime   int64
	Status       GuildMemberStatus
}

// GuildPetitionSignature represents one charter signer.
type GuildPetitionSignature struct {
	PlayerGUID    uint64
	PlayerAccount uint32
}

// GuildPetition represents a native AzerothCore petition charter.
type GuildPetition struct {
	RealmID      uint32
	PetitionID   uint32
	PetitionGUID uint64
	OwnerGUID    uint64
	Name         string
	Type         uint8
	Signatures   []GuildPetitionSignature
}

// GuildBankSocketEnchant is one visible socket enchant in a guild bank item.
type GuildBankSocketEnchant struct {
	SocketIndex     uint8
	SocketEnchantID uint32
}

// GuildBankItem is the DB-backed item state needed to render and mutate guild bank slots.
type GuildBankItem struct {
	ItemGUID           uint64
	Entry              uint32
	Slot               uint8
	Count              uint32
	Flags              uint32
	RandomPropertyID   int32
	RandomPropertySeed int32
	Durability         uint32
	EnchantmentID      uint32
	SocketEnchants     []GuildBankSocketEnchant
	Charges            uint32
	Text               string
}

// GuildBankTab is one purchased guild bank tab.
type GuildBankTab struct {
	TabID uint8
	Name  string
	Icon  string
	Text  string
	Items []GuildBankItem
}

// GuildBank is the full persisted bank state needed by gateway packet renderers.
type GuildBank struct {
	GuildID uint64
	Money   uint64
	Tabs    []GuildBankTab
}

type GuildBankLogEntry struct {
	PlayerGUID uint64
	TimeOffset uint32
	EntryType  int8
	Money      uint32
	ItemID     int32
	Count      int32
	OtherTab   int8
}

type GuildBankEventLogType int8

const (
	GuildBankLogDepositItem   GuildBankEventLogType = 1
	GuildBankLogWithdrawItem  GuildBankEventLogType = 2
	GuildBankLogMoveItem      GuildBankEventLogType = 3
	GuildBankLogDepositMoney  GuildBankEventLogType = 4
	GuildBankLogWithdrawMoney GuildBankEventLogType = 5
	GuildBankLogRepairMoney   GuildBankEventLogType = 6
	GuildBankLogMoveItem2     GuildBankEventLogType = 7
	GuildBankLogBuySlot       GuildBankEventLogType = 9
)

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

	// IgnoredByGuildMembers returns receivers that ignore the sender.
	IgnoredByGuildMembers(ctx context.Context, realmID uint32, senderGUID uint64, receiverGUIDs []uint64) (map[uint64]bool, error)

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
	UpdateGuildRank(ctx context.Context, realmID uint32, guildID uint64, rank uint8, name string, rights, moneyPerDay uint32, bankTabRights [GuildBankMaxTabs]GuildBankTabRight) error

	// AddGuildRank adds guild rank.
	AddGuildRank(ctx context.Context, realmID uint32, guildID uint64, rank uint8, name string, rights, moneyPerDay uint32) error

	// DeleteLowestGuildRank deletes lowes guild rank.
	DeleteLowestGuildRank(ctx context.Context, realmID uint32, guildID uint64, rank uint8) error

	// GuildPetitionByGUID loads a native petition by item GUID.
	GuildPetitionByGUID(ctx context.Context, realmID uint32, petitionGUID uint64) (*GuildPetition, error)

	// AddGuildPetitionSignature persists a native guild petition signature.
	AddGuildPetitionSignature(ctx context.Context, realmID uint32, petitionID uint32, petitionGUID, ownerGUID, playerGUID uint64, playerAccount uint32) error

	// GuildBank loads the persisted guild bank state for one guild.
	GuildBank(ctx context.Context, realmID uint32, guildID uint64, tabID uint8, fullUpdate bool) (*GuildBank, error)

	// GuildBankLog loads the persisted guild bank log for one tab.
	GuildBankLog(ctx context.Context, realmID uint32, guildID uint64, tabID uint8) ([]GuildBankLogEntry, error)

	SetGuildBankTabInfo(ctx context.Context, realmID uint32, guildID uint64, tabID uint8, name, icon string) error
	SetGuildBankTabText(ctx context.Context, realmID uint32, guildID uint64, tabID uint8, text string) error
	BuyGuildBankTab(ctx context.Context, realmID uint32, guildID uint64, tabID uint8) error
	DepositGuildBankMoney(ctx context.Context, realmID uint32, guildID uint64, memberGUID uint64, amount uint32) error
	WithdrawGuildBankMoney(ctx context.Context, realmID uint32, guildID uint64, memberGUID uint64, amount uint32, repair bool) (uint32, error)
	RollbackGuildBankMoneyWithdraw(ctx context.Context, realmID uint32, guildID uint64, memberGUID uint64, amount uint32, repair bool, logGUID uint32) error
	DepositGuildBankItem(ctx context.Context, realmID uint32, guildID uint64, memberGUID uint64, tabID, slotID uint8, item GuildBankItem) error
	WithdrawGuildBankItem(ctx context.Context, realmID uint32, guildID uint64, memberGUID uint64, tabID, slotID uint8, count uint32, splitItemGUID uint64) (*GuildBankItem, uint32, error)
	RollbackGuildBankItemWithdraw(ctx context.Context, realmID uint32, guildID uint64, memberGUID uint64, tabID, slotID uint8, item GuildBankItem, logGUID uint32) ([]uint8, error)
	MoveGuildBankItem(ctx context.Context, realmID uint32, guildID uint64, memberGUID uint64, sourceTabID, sourceSlotID, destinationTabID, destinationSlotID uint8, count uint32, splitItemGUID uint64) ([]uint8, error)
}
