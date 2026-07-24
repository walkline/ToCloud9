package repo

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	shrepo "github.com/walkline/ToCloud9/shared/repo"
)

// tempSwapSlot is a slot id outside the valid range (0..97) used to swap two
// rows without violating the (guildid, TabId, SlotId) primary key.
const tempSwapSlot = 254

type guildBankMySQLRepo struct {
	db shrepo.CharactersDB
}

func NewGuildBankMySQLRepo(db shrepo.CharactersDB) GuildBankRepo {
	return &guildBankMySQLRepo{db: db}
}

func (g *guildBankMySQLRepo) BankMoney(ctx context.Context, realmID uint32, guildID uint64) (uint64, error) {
	var money uint64
	err := g.db.DBByRealm(realmID).QueryRowContext(ctx,
		"SELECT BankMoney FROM guild WHERE guildid = ?", guildID).Scan(&money)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	return money, err
}

func (g *guildBankMySQLRepo) BankTabs(ctx context.Context, realmID uint32, guildID uint64) ([]BankTab, error) {
	rows, err := g.db.DBByRealm(realmID).QueryContext(ctx,
		"SELECT TabId, TabName, TabIcon, COALESCE(TabText, '') FROM guild_bank_tab WHERE guildid = ? ORDER BY TabId ASC", guildID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tabs []BankTab
	for rows.Next() {
		tab := BankTab{}
		if err = rows.Scan(&tab.TabID, &tab.Name, &tab.Icon, &tab.Text); err != nil {
			return nil, err
		}
		tabs = append(tabs, tab)
	}
	return tabs, rows.Err()
}

const bankItemSelect = `
SELECT b.SlotId, b.item_guid, ii.itemEntry, ii.count, ii.flags, ii.durability,
       ii.randomPropertyId, ii.enchantments, ii.charges, COALESCE(ii.text, '')
FROM guild_bank_item b
JOIN item_instance ii ON ii.guid = b.item_guid`

func scanBankItem(scanner interface{ Scan(...interface{}) error }) (BankItem, error) {
	item := BankItem{}
	var enchantments, charges string
	err := scanner.Scan(
		&item.Slot, &item.ItemGUID, &item.Entry, &item.Count, &item.Flags,
		&item.Durability, &item.RandomPropertyID, &enchantments, &charges, &item.Text,
	)
	if err != nil {
		return item, err
	}
	item.EnchantmentID = enchantSlotID(enchantments, 0)
	item.Charges = firstNonZeroAbs(charges)
	// Sockets are enchant slots 2..4 (SOCK_ENCHANTMENT_SLOT..+2).
	for socket := 0; socket < 3; socket++ {
		item.SocketEnchantIDs = append(item.SocketEnchantIDs, enchantSlotID(enchantments, 2+socket))
	}
	return item, nil
}

// enchantSlotID extracts the enchant id of the given enchantment slot from
// item_instance.enchantments (space-separated triplets: id duration charges).
func enchantSlotID(s string, slot int) uint32 {
	fields := strings.Fields(s)
	idx := slot * 3
	if idx >= len(fields) {
		return 0
	}
	v, _ := strconv.ParseUint(fields[idx], 10, 32)
	return uint32(v)
}

// firstNonZeroAbs parses item_instance.charges and returns the first non-zero
// spell charge as a positive count.
func firstNonZeroAbs(s string) uint32 {
	for _, f := range strings.Fields(s) {
		v, err := strconv.ParseInt(f, 10, 32)
		if err != nil || v == 0 {
			continue
		}
		if v < 0 {
			v = -v
		}
		return uint32(v)
	}
	return 0
}

func (g *guildBankMySQLRepo) BankTabItems(ctx context.Context, realmID uint32, guildID uint64, tabID uint8) ([]BankItem, error) {
	rows, err := g.db.DBByRealm(realmID).QueryContext(ctx,
		bankItemSelect+" WHERE b.guildid = ? AND b.TabId = ? ORDER BY b.SlotId ASC", guildID, tabID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []BankItem
	for rows.Next() {
		item, err := scanBankItem(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (g *guildBankMySQLRepo) RankTabRights(ctx context.Context, realmID uint32, guildID uint64) (map[uint8][GuildBankMaxTabs]BankTabRights, error) {
	rows, err := g.db.DBByRealm(realmID).QueryContext(ctx,
		"SELECT TabId, rid, gbright, SlotPerDay FROM guild_bank_right WHERE guildid = ?", guildID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := map[uint8][GuildBankMaxTabs]BankTabRights{}
	for rows.Next() {
		var tabID, rank, rights uint8
		var slots uint32
		if err = rows.Scan(&tabID, &rank, &rights, &slots); err != nil {
			return nil, err
		}
		if tabID >= GuildBankMaxTabs {
			continue
		}
		tabs := result[rank]
		tabs[tabID] = BankTabRights{Rights: rights, SlotsPerDay: slots}
		result[rank] = tabs
	}
	return result, rows.Err()
}

func (g *guildBankMySQLRepo) SetRankTabRights(ctx context.Context, realmID uint32, guildID uint64, rank uint8, rights []BankTabRights) error {
	tx, err := g.db.DBByRealm(realmID).BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for tabID, r := range rights {
		_, err = tx.ExecContext(ctx, `
INSERT INTO guild_bank_right (guildid, TabId, rid, gbright, SlotPerDay) VALUES (?, ?, ?, ?, ?)
ON DUPLICATE KEY UPDATE gbright = VALUES(gbright), SlotPerDay = VALUES(SlotPerDay)`,
			guildID, tabID, rank, r.Rights, r.SlotsPerDay)
		if err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (g *guildBankMySQLRepo) MemberWithdrawals(ctx context.Context, realmID uint32, memberGUID uint64) (BankWithdrawals, error) {
	w := BankWithdrawals{}
	err := g.db.DBByRealm(realmID).QueryRowContext(ctx,
		"SELECT tab0, tab1, tab2, tab3, tab4, tab5, money FROM guild_member_withdraw WHERE guid = ?",
		memberGUID).Scan(&w.Tabs[0], &w.Tabs[1], &w.Tabs[2], &w.Tabs[3], &w.Tabs[4], &w.Tabs[5], &w.Money)
	if errors.Is(err, sql.ErrNoRows) {
		return BankWithdrawals{}, nil
	}
	return w, err
}

func (g *guildBankMySQLRepo) DepositMoney(ctx context.Context, realmID uint32, guildID, playerGUID, amount uint64) (uint64, error) {
	// Guards the upper bound below, which would wrap around on a huge amount
	// and let the deposit through.
	if amount > GuildBankMoneyLimit {
		return 0, ErrBankMoneyLimit
	}

	tx, err := g.db.DBByRealm(realmID).BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	res, err := tx.ExecContext(ctx,
		"UPDATE guild SET BankMoney = BankMoney + ? WHERE guildid = ? AND BankMoney <= ?",
		amount, guildID, GuildBankMoneyLimit-amount)
	if err != nil {
		return 0, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return 0, ErrBankMoneyLimit
	}

	if err = g.insertLog(ctx, tx, guildID, GuildBankMoneyLogsDBTab, BankLogDepositMoney, playerGUID, uint32(amount), 0, 0); err != nil {
		return 0, err
	}

	var newMoney uint64
	if err = tx.QueryRowContext(ctx, "SELECT BankMoney FROM guild WHERE guildid = ?", guildID).Scan(&newMoney); err != nil {
		return 0, err
	}
	return newMoney, tx.Commit()
}

func (g *guildBankMySQLRepo) WithdrawMoney(ctx context.Context, realmID uint32, guildID, playerGUID, amount uint64, dailyLimit uint32) (uint64, error) {
	tx, err := g.db.DBByRealm(realmID).BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	if dailyLimit != BankWithdrawUnlimited {
		if err = g.countWithdrawal(ctx, tx, playerGUID, "money", uint32(amount), dailyLimit); err != nil {
			return 0, err
		}
	}

	res, err := tx.ExecContext(ctx,
		"UPDATE guild SET BankMoney = BankMoney - ? WHERE guildid = ? AND BankMoney >= ?",
		amount, guildID, amount)
	if err != nil {
		return 0, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return 0, ErrBankNotEnoughMoney
	}

	if err = g.insertLog(ctx, tx, guildID, GuildBankMoneyLogsDBTab, BankLogWithdrawMoney, playerGUID, uint32(amount), 0, 0); err != nil {
		return 0, err
	}

	var newMoney uint64
	if err = tx.QueryRowContext(ctx, "SELECT BankMoney FROM guild WHERE guildid = ?", guildID).Scan(&newMoney); err != nil {
		return 0, err
	}
	return newMoney, tx.Commit()
}

// countWithdrawal counts amount against the member daily limit for the given
// counter column, failing with ErrBankWithdrawLimit when it would exceed it.
func (g *guildBankMySQLRepo) countWithdrawal(ctx context.Context, tx *sql.Tx, memberGUID uint64, column string, amount, dailyLimit uint32) error {
	if _, err := tx.ExecContext(ctx,
		"INSERT IGNORE INTO guild_member_withdraw (guid) VALUES (?)", memberGUID); err != nil {
		return err
	}
	res, err := tx.ExecContext(ctx,
		fmt.Sprintf("UPDATE guild_member_withdraw SET %s = %s + ? WHERE guid = ? AND %s + ? <= ?", column, column, column),
		amount, memberGUID, amount, dailyLimit)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrBankWithdrawLimit
	}
	return nil
}

func (g *guildBankMySQLRepo) DepositItem(ctx context.Context, realmID uint32, guildID, playerGUID uint64, tabID, slot uint8, item BankItem, logEvent bool) (uint8, error) {
	tx, err := g.db.DBByRealm(realmID).BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	var one int
	err = tx.QueryRowContext(ctx, "SELECT 1 FROM guild_bank_tab WHERE guildid = ? AND TabId = ?", guildID, tabID).Scan(&one)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, ErrBankTabNotFound
	}
	if err != nil {
		return 0, err
	}

	if slot == BankSlotAuto {
		slot, err = g.firstFreeSlot(ctx, tx, guildID, tabID)
		if err != nil {
			return 0, err
		}
	} else {
		err = tx.QueryRowContext(ctx,
			"SELECT 1 FROM guild_bank_item WHERE guildid = ? AND TabId = ? AND SlotId = ? FOR UPDATE",
			guildID, tabID, slot).Scan(&one)
		if err == nil {
			return 0, ErrBankSlotOccupied
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return 0, err
		}
	}

	_, err = tx.ExecContext(ctx,
		"INSERT INTO guild_bank_item (guildid, TabId, SlotId, item_guid) VALUES (?, ?, ?, ?)",
		guildID, tabID, slot, item.ItemGUID)
	if err != nil {
		return 0, err
	}

	if logEvent {
		if err = g.insertLog(ctx, tx, guildID, tabID, BankLogDepositItem, playerGUID, item.Entry, uint16(item.Count), 0); err != nil {
			return 0, err
		}
	}
	return slot, tx.Commit()
}

func (g *guildBankMySQLRepo) firstFreeSlot(ctx context.Context, tx *sql.Tx, guildID uint64, tabID uint8) (uint8, error) {
	rows, err := tx.QueryContext(ctx,
		"SELECT SlotId FROM guild_bank_item WHERE guildid = ? AND TabId = ? ORDER BY SlotId ASC FOR UPDATE",
		guildID, tabID)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	used := map[uint8]bool{}
	for rows.Next() {
		var s uint8
		if err = rows.Scan(&s); err != nil {
			return 0, err
		}
		used[s] = true
	}
	if rows.Err() != nil {
		return 0, rows.Err()
	}
	for s := uint8(0); s < GuildBankMaxSlots; s++ {
		if !used[s] {
			return s, nil
		}
	}
	return 0, ErrBankTabFull
}

func (g *guildBankMySQLRepo) WithdrawItem(ctx context.Context, realmID uint32, guildID, playerGUID uint64, tabID, slot uint8, dailyLimit uint32) (BankItem, error) {
	tx, err := g.db.DBByRealm(realmID).BeginTx(ctx, nil)
	if err != nil {
		return BankItem{}, err
	}
	defer tx.Rollback()

	item, err := scanBankItem(tx.QueryRowContext(ctx,
		bankItemSelect+" WHERE b.guildid = ? AND b.TabId = ? AND b.SlotId = ? FOR UPDATE",
		guildID, tabID, slot))
	if errors.Is(err, sql.ErrNoRows) {
		return BankItem{}, ErrBankSlotEmpty
	}
	if err != nil {
		return BankItem{}, err
	}

	if dailyLimit != BankWithdrawUnlimited {
		if err = g.countWithdrawal(ctx, tx, playerGUID, fmt.Sprintf("tab%d", tabID), 1, dailyLimit); err != nil {
			return BankItem{}, err
		}
	}

	_, err = tx.ExecContext(ctx,
		"DELETE FROM guild_bank_item WHERE guildid = ? AND TabId = ? AND SlotId = ?",
		guildID, tabID, slot)
	if err != nil {
		return BankItem{}, err
	}

	if err = g.insertLog(ctx, tx, guildID, tabID, BankLogWithdrawItem, playerGUID, item.Entry, uint16(item.Count), 0); err != nil {
		return BankItem{}, err
	}
	return item, tx.Commit()
}

func (g *guildBankMySQLRepo) MoveItem(ctx context.Context, realmID uint32, guildID, playerGUID uint64, srcTab, srcSlot, dstTab, dstSlot uint8, splitCount uint32) error {
	tx, err := g.db.DBByRealm(realmID).BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	src, err := scanBankItem(tx.QueryRowContext(ctx,
		bankItemSelect+" WHERE b.guildid = ? AND b.TabId = ? AND b.SlotId = ? FOR UPDATE",
		guildID, srcTab, srcSlot))
	if errors.Is(err, sql.ErrNoRows) {
		return ErrBankSlotEmpty
	}
	if err != nil {
		return err
	}

	if splitCount > 0 && splitCount < src.Count {
		return ErrBankSplitUnsupported
	}

	dstOccupied := true
	_, err = scanBankItem(tx.QueryRowContext(ctx,
		bankItemSelect+" WHERE b.guildid = ? AND b.TabId = ? AND b.SlotId = ? FOR UPDATE",
		guildID, dstTab, dstSlot))
	if errors.Is(err, sql.ErrNoRows) {
		dstOccupied = false
	} else if err != nil {
		return err
	}

	if dstOccupied {
		// Swap through a temporary slot to keep the primary key satisfied.
		steps := [][]interface{}{
			{tempSwapSlot, guildID, dstTab, dstSlot},
			{dstSlot, dstTab, guildID, srcTab, srcSlot},
			{srcSlot, srcTab, guildID, dstTab, tempSwapSlot},
		}
		for i, s := range steps {
			var q string
			if i == 0 {
				q = "UPDATE guild_bank_item SET SlotId = ? WHERE guildid = ? AND TabId = ? AND SlotId = ?"
			} else {
				q = "UPDATE guild_bank_item SET SlotId = ?, TabId = ? WHERE guildid = ? AND TabId = ? AND SlotId = ?"
			}
			if _, err = tx.ExecContext(ctx, q, s...); err != nil {
				return err
			}
		}
	} else {
		_, err = tx.ExecContext(ctx,
			"UPDATE guild_bank_item SET TabId = ?, SlotId = ? WHERE guildid = ? AND TabId = ? AND SlotId = ?",
			dstTab, dstSlot, guildID, srcTab, srcSlot)
		if err != nil {
			return err
		}
	}

	if srcTab != dstTab {
		if err = g.insertLog(ctx, tx, guildID, srcTab, BankLogMoveItem, playerGUID, src.Entry, uint16(src.Count), dstTab); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (g *guildBankMySQLRepo) CreateTab(ctx context.Context, realmID uint32, guildID, playerGUID uint64, tabID uint8, cost uint64) error {
	tx, err := g.db.DBByRealm(realmID).BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var count uint8
	if err = tx.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM guild_bank_tab WHERE guildid = ? FOR UPDATE", guildID).Scan(&count); err != nil {
		return err
	}
	if tabID != count || tabID >= GuildBankMaxTabs {
		return ErrBankTabNotFound
	}

	_, err = tx.ExecContext(ctx,
		"INSERT INTO guild_bank_tab (guildid, TabId, TabName, TabIcon, TabText) VALUES (?, ?, '', '', '')",
		guildID, tabID)
	if err != nil {
		return err
	}

	// The core records tab purchases in the money log, so the guild can see
	// who paid for a tab.
	if err = g.insertLog(ctx, tx, guildID, GuildBankMoneyLogsDBTab, BankLogBuySlot, playerGUID, uint32(cost), 0, tabID); err != nil {
		return err
	}

	return tx.Commit()
}

func (g *guildBankMySQLRepo) SetTabInfo(ctx context.Context, realmID uint32, guildID uint64, tabID uint8, name, icon string) error {
	res, err := g.db.DBByRealm(realmID).ExecContext(ctx,
		"UPDATE guild_bank_tab SET TabName = ?, TabIcon = ? WHERE guildid = ? AND TabId = ?",
		name, icon, guildID, tabID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		// Also raised when the values did not change; treat only a missing tab as an error.
		var one int
		err = g.db.DBByRealm(realmID).QueryRowContext(ctx,
			"SELECT 1 FROM guild_bank_tab WHERE guildid = ? AND TabId = ?", guildID, tabID).Scan(&one)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrBankTabNotFound
		}
		return err
	}
	return nil
}

func (g *guildBankMySQLRepo) SetTabText(ctx context.Context, realmID uint32, guildID uint64, tabID uint8, text string) error {
	res, err := g.db.DBByRealm(realmID).ExecContext(ctx,
		"UPDATE guild_bank_tab SET TabText = ? WHERE guildid = ? AND TabId = ?",
		text, guildID, tabID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		// Also raised when the text did not change; treat only a missing tab as an error.
		var one int
		err = g.db.DBByRealm(realmID).QueryRowContext(ctx,
			"SELECT 1 FROM guild_bank_tab WHERE guildid = ? AND TabId = ?", guildID, tabID).Scan(&one)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrBankTabNotFound
		}
		return err
	}
	return nil
}

func (g *guildBankMySQLRepo) BankLog(ctx context.Context, realmID uint32, guildID uint64, tabID uint8) ([]BankLogEntry, error) {
	dbTab := tabID
	if tabID == GuildBankMoneyLogTab {
		dbTab = GuildBankMoneyLogsDBTab
	}
	rows, err := g.db.DBByRealm(realmID).QueryContext(ctx, `
SELECT EventType, PlayerGuid, ItemOrMoney, ItemStackCount, DestTabId, TimeStamp
FROM guild_bank_eventlog WHERE guildid = ? AND TabId = ?
ORDER BY TimeStamp DESC, LogGuid DESC LIMIT ?`, guildID, dbTab, GuildBankLogMaxEntries)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []BankLogEntry
	for rows.Next() {
		e := BankLogEntry{}
		var itemOrMoney uint32
		if err = rows.Scan(&e.EventType, &e.PlayerGUID, &itemOrMoney, &e.Count, &e.DestTab, &e.Timestamp); err != nil {
			return nil, err
		}
		switch e.EventType {
		case BankLogDepositMoney, BankLogWithdrawMoney, BankLogRepairMoney, BankLogBuySlot:
			e.Money = uint64(itemOrMoney)
		default:
			e.ItemEntry = itemOrMoney
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

func (g *guildBankMySQLRepo) insertLog(ctx context.Context, tx *sql.Tx, guildID uint64, dbTab uint8, eventType uint8, playerGUID uint64, itemOrMoney uint32, stackCount uint16, destTab uint8) error {
	var next uint32
	if err := tx.QueryRowContext(ctx,
		"SELECT COALESCE(MAX(LogGuid), 0) + 1 FROM guild_bank_eventlog WHERE guildid = ? AND TabId = ? FOR UPDATE",
		guildID, dbTab).Scan(&next); err != nil {
		return err
	}

	_, err := tx.ExecContext(ctx, `
INSERT INTO guild_bank_eventlog (guildid, LogGuid, TabId, EventType, PlayerGuid, ItemOrMoney, ItemStackCount, DestTabId, TimeStamp)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		guildID, next, dbTab, eventType, playerGUID, itemOrMoney, stackCount, destTab, time.Now().Unix())
	if err != nil {
		return err
	}

	if next > GuildBankLogMaxEntries {
		_, err = tx.ExecContext(ctx,
			"DELETE FROM guild_bank_eventlog WHERE guildid = ? AND TabId = ? AND LogGuid <= ?",
			guildID, dbTab, next-GuildBankLogMaxEntries)
	}
	return err
}

func (g *guildBankMySQLRepo) ResetDailyWithdrawals(ctx context.Context, realmID uint32) error {
	_, err := g.db.DBByRealm(realmID).ExecContext(ctx,
		"UPDATE guild_member_withdraw SET tab0 = 0, tab1 = 0, tab2 = 0, tab3 = 0, tab4 = 0, tab5 = 0, money = 0")
	return err
}
