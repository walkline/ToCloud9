package repo

import (
	"context"
	"errors"
)

// Guild bank dimensions and AC-compatible constants.
const (
	GuildBankMaxTabs  = 6
	GuildBankMaxSlots = 98

	// BankSlotAuto asks for the first free slot of the tab.
	BankSlotAuto = 0xFF

	// Money log entries are stored with this TabId in DB (AC compat).
	GuildBankMoneyLogsDBTab = 100
	// Client-facing pseudo tab id of the money log.
	GuildBankMoneyLogTab = GuildBankMaxTabs

	// GuildBankLogMaxEntries is how many log entries are kept per tab (AC compat).
	GuildBankLogMaxEntries = 25

	// GuildBankMoneyLimit is the maximum amount of money the bank can hold (AC compat).
	GuildBankMoneyLimit = uint64(0x7FFFFFFFFFFFF)
)

// Guild bank per-tab rights (AC GuildBankRights).
const (
	BankRightViewTab     = 0x01
	BankRightPutItem     = 0x02
	BankRightUpdateText  = 0x04
	BankRightDepositItem = BankRightViewTab | BankRightPutItem
	BankRightFull        = 0xFF
)

// BankWithdrawUnlimited marks unlimited daily withdrawals (guild master).
const BankWithdrawUnlimited = uint32(0xFFFFFFFF)

// Guild bank event log types (AC GuildBankEventLogTypes).
const (
	BankLogDepositItem   = 1
	BankLogWithdrawItem  = 2
	BankLogMoveItem      = 3
	BankLogDepositMoney  = 4
	BankLogWithdrawMoney = 5
	BankLogRepairMoney   = 6
	BankLogMoveItem2     = 7
	BankLogBuySlot       = 9
)

var (
	// ErrBankTabNotFound returned when the tab is not purchased.
	ErrBankTabNotFound = errors.New("bank tab not found")
	// ErrBankSlotOccupied returned when the destination slot already holds an item.
	ErrBankSlotOccupied = errors.New("bank slot occupied")
	// ErrBankSlotEmpty returned when the source slot holds no item.
	ErrBankSlotEmpty = errors.New("bank slot empty")
	// ErrBankTabFull returned when no free slot is left in the tab.
	ErrBankTabFull = errors.New("bank tab full")
	// ErrBankNotEnoughMoney returned when the bank holds less than requested.
	ErrBankNotEnoughMoney = errors.New("not enough money in guild bank")
	// ErrBankMoneyLimit returned when a deposit would overflow the bank.
	ErrBankMoneyLimit = errors.New("guild bank money limit reached")
	// ErrBankWithdrawLimit returned when the member daily limit is reached.
	ErrBankWithdrawLimit = errors.New("daily withdrawal limit reached")
	// ErrBankSplitUnsupported returned for stack splits, which the cluster
	// bank does not support yet.
	ErrBankSplitUnsupported = errors.New("stack split is not supported yet")
)

// BankTab represents a purchased guild bank tab.
type BankTab struct {
	TabID uint8
	Name  string
	Icon  string
	Text  string
}

// BankItem represents an item placed in the guild bank, with the item_instance
// fields needed to render it client side.
type BankItem struct {
	Slot             uint8
	ItemGUID         uint64
	Entry            uint32
	Count            uint32
	Flags            uint32
	Durability       uint32
	RandomPropertyID int32
	EnchantmentID    uint32
	Charges          uint32
	Text             string
	// Gem enchant ids by socket index (0 = empty socket).
	SocketEnchantIDs []uint32
}

// BankTabRights is the per-tab rights of a rank.
type BankTabRights struct {
	Rights      uint8
	SlotsPerDay uint32
}

// BankWithdrawals is what a member has withdrawn today.
type BankWithdrawals struct {
	Tabs  [GuildBankMaxTabs]uint32
	Money uint32
}

// BankLogEntry is one guild bank event log record.
type BankLogEntry struct {
	EventType  uint8
	PlayerGUID uint64
	ItemEntry  uint32
	Count      uint32
	DestTab    uint8
	Money      uint64
	Timestamp  int64
}

// GuildBankRepo is the storage of the guild bank state. Unlike the guilds
// repo it is not fronted by the in-memory cache: bank reads always hit the
// DB, the guild service being its only writer.
//
//go:generate mockery --name=GuildBankRepo --filename=guild-bank-repo.go
type GuildBankRepo interface {
	BankMoney(ctx context.Context, realmID uint32, guildID uint64) (uint64, error)
	BankTabs(ctx context.Context, realmID uint32, guildID uint64) ([]BankTab, error)
	BankTabItems(ctx context.Context, realmID uint32, guildID uint64, tabID uint8) ([]BankItem, error)

	// RankTabRights returns the per-rank per-tab rights of the guild.
	RankTabRights(ctx context.Context, realmID uint32, guildID uint64) (map[uint8][GuildBankMaxTabs]BankTabRights, error)
	// SetRankTabRights replaces the rights of the given rank for the purchased tabs.
	SetRankTabRights(ctx context.Context, realmID uint32, guildID uint64, rank uint8, rights []BankTabRights) error

	// MemberWithdrawals returns what the member has withdrawn today.
	MemberWithdrawals(ctx context.Context, realmID uint32, memberGUID uint64) (BankWithdrawals, error)

	// DepositMoney adds to the bank money and logs the event.
	DepositMoney(ctx context.Context, realmID uint32, guildID, playerGUID, amount uint64) (uint64, error)
	// WithdrawMoney removes from the bank money, counts it against the member
	// daily limit (unless unlimited) and logs the event. dailyLimit is the
	// member's rank limit; BankWithdrawUnlimited skips the counter.
	WithdrawMoney(ctx context.Context, realmID uint32, guildID, playerGUID, amount uint64, dailyLimit uint32) (uint64, error)

	// DepositItem records the item placement. Slot BankSlotAuto picks the first
	// free slot. Returns the slot used.
	DepositItem(ctx context.Context, realmID uint32, guildID, playerGUID uint64, tabID, slot uint8, item BankItem, logEvent bool) (uint8, error)
	// WithdrawItem removes the item placement, counts it against the member
	// daily limit (unless unlimited) and returns the removed item.
	WithdrawItem(ctx context.Context, realmID uint32, guildID, playerGUID uint64, tabID, slot uint8, dailyLimit uint32) (BankItem, error)
	// MoveItem moves or swaps an item inside the bank. splitCount below the
	// source stack size fails with ErrBankSplitUnsupported (a value equal to
	// the stack size is a whole move, which is how the client asks for one).
	MoveItem(ctx context.Context, realmID uint32, guildID, playerGUID uint64, srcTab, srcSlot, dstTab, dstSlot uint8, splitCount uint32) error

	// CreateTab creates the next bank tab (index must equal the purchased count).
	CreateTab(ctx context.Context, realmID uint32, guildID, playerGUID uint64, tabID uint8, cost uint64) error
	SetTabInfo(ctx context.Context, realmID uint32, guildID uint64, tabID uint8, name, icon string) error
	SetTabText(ctx context.Context, realmID uint32, guildID uint64, tabID uint8, text string) error

	// BankLog returns the newest log entries of a tab (GuildBankMoneyLogTab = money log).
	BankLog(ctx context.Context, realmID uint32, guildID uint64, tabID uint8) ([]BankLogEntry, error)

	// ResetDailyWithdrawals zeroes all member withdrawal counters of the realm.
	ResetDailyWithdrawals(ctx context.Context, realmID uint32) error
}
