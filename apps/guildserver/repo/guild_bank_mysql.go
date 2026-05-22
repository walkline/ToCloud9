package repo

import (
	"context"
	"database/sql"
	"errors"
	"sort"
	"strconv"
	"strings"
	"time"
)

const guildBankLogLimit = 25

var errGuildBankItemCantStack = errors.New("guild bank item can't stack")

func (g *guildsMySQLRepo) GuildBank(ctx context.Context, realmID uint32, guildID uint64, tabID uint8, fullUpdate bool) (*GuildBank, error) {
	bank := &GuildBank{GuildID: guildID}

	rows, err := g.db.DBByRealm(realmID).QueryContext(ctx, `
SELECT
	t.TabId, COALESCE(t.TabName, ''), COALESCE(t.TabIcon, ''), COALESCE(t.TabText, ''),
	bi.SlotId, ii.guid, ii.itemEntry, ii.count, ii.flags, ii.randomPropertyId, ii.durability,
	ii.charges, ii.enchantments, COALESCE(ii.text, '')
FROM guild_bank_tab t
LEFT JOIN guild_bank_item bi ON bi.guildid = t.guildid AND bi.TabId = t.TabId
LEFT JOIN item_instance ii ON ii.guid = bi.item_guid
WHERE t.guildid = ? AND (? OR t.TabId = ?)
ORDER BY t.TabId ASC, bi.SlotId ASC`, guildID, fullUpdate, tabID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	tabByID := map[uint8]*GuildBankTab{}
	for rows.Next() {
		var tab GuildBankTab
		var slotID sql.NullInt64
		var itemGUID sql.NullInt64
		var itemEntry sql.NullInt64
		var count sql.NullInt64
		var flags sql.NullInt64
		var randomPropertyID sql.NullInt64
		var durability sql.NullInt64
		var charges sql.NullString
		var enchantments sql.NullString
		var itemText string

		if err = rows.Scan(
			&tab.TabID, &tab.Name, &tab.Icon, &tab.Text,
			&slotID, &itemGUID, &itemEntry, &count, &flags, &randomPropertyID, &durability,
			&charges, &enchantments, &itemText,
		); err != nil {
			return nil, err
		}

		bankTab := tabByID[tab.TabID]
		if bankTab == nil {
			bank.Tabs = append(bank.Tabs, tab)
			bankTab = &bank.Tabs[len(bank.Tabs)-1]
			tabByID[tab.TabID] = bankTab
		}

		if !itemGUID.Valid || !itemEntry.Valid || !slotID.Valid {
			continue
		}

		item := GuildBankItem{
			ItemGUID:         uint64(itemGUID.Int64),
			Entry:            uint32(itemEntry.Int64),
			Slot:             uint8(slotID.Int64),
			Count:            uint32(count.Int64),
			Flags:            uint32(flags.Int64),
			RandomPropertyID: int32(randomPropertyID.Int64),
			Durability:       uint32(durability.Int64),
			Charges:          firstItemCharge(charges.String),
			Text:             itemText,
		}
		item.EnchantmentID, item.SocketEnchants = guildBankEnchants(enchantments.String)
		bankTab.Items = append(bankTab.Items, item)
	}
	if rows.Err() != nil {
		return nil, rows.Err()
	}

	if err = g.db.DBByRealm(realmID).QueryRowContext(ctx, `
SELECT BankMoney
FROM guild
WHERE guildid = ?`, guildID).Scan(&bank.Money); err == sql.ErrNoRows {
		return nil, nil
	} else if err != nil {
		return nil, err
	}

	return bank, nil
}

func (g *guildsMySQLRepo) GuildBankLog(ctx context.Context, realmID uint32, guildID uint64, tabID uint8) ([]GuildBankLogEntry, error) {
	rows, err := g.db.DBByRealm(realmID).QueryContext(ctx, `
SELECT EventType, PlayerGuid, ItemOrMoney, ItemStackCount, DestTabId, TimeStamp
FROM guild_bank_eventlog
WHERE guildid = ? AND TabId = ?
ORDER BY TimeStamp DESC, LogGuid DESC
LIMIT ?`, guildID, tabID, guildBankLogLimit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	now := uint32(time.Now().Unix())
	var result []GuildBankLogEntry
	for rows.Next() {
		var eventType int8
		var playerGUID uint64
		var itemOrMoney uint32
		var itemStackCount uint32
		var destTab uint8
		var timestamp uint32
		if err = rows.Scan(&eventType, &playerGUID, &itemOrMoney, &itemStackCount, &destTab, &timestamp); err != nil {
			return nil, err
		}

		entry := GuildBankLogEntry{
			PlayerGUID: playerGUID,
			TimeOffset: func() uint32 {
				if now < timestamp {
					return 0
				}
				return now - timestamp
			}(),
			EntryType: eventType,
		}
		switch GuildBankEventLogType(eventType) {
		case GuildBankLogDepositItem, GuildBankLogWithdrawItem:
			entry.ItemID = int32(itemOrMoney)
			entry.Count = int32(itemStackCount)
		case GuildBankLogMoveItem, GuildBankLogMoveItem2:
			entry.ItemID = int32(itemOrMoney)
			entry.Count = int32(itemStackCount)
			entry.OtherTab = int8(destTab)
		default:
			entry.Money = itemOrMoney
		}
		result = append(result, entry)
	}

	return result, rows.Err()
}

func (g *guildsMySQLRepo) SetGuildBankTabInfo(ctx context.Context, realmID uint32, guildID uint64, tabID uint8, name, icon string) error {
	_, err := g.db.DBByRealm(realmID).ExecContext(ctx, `
UPDATE guild_bank_tab
SET TabName = ?, TabIcon = ?
WHERE guildid = ? AND TabId = ?`, name, icon, guildID, tabID)
	return err
}

func (g *guildsMySQLRepo) SetGuildBankTabText(ctx context.Context, realmID uint32, guildID uint64, tabID uint8, text string) error {
	_, err := g.db.DBByRealm(realmID).ExecContext(ctx, `
UPDATE guild_bank_tab
SET TabText = ?
WHERE guildid = ? AND TabId = ?`, text, guildID, tabID)
	return err
}

func (g *guildsMySQLRepo) BuyGuildBankTab(ctx context.Context, realmID uint32, guildID uint64, tabID uint8) error {
	tx, err := g.db.DBByRealm(realmID).BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer rollbackTx(tx)

	var guildIDFound uint64
	if err = tx.QueryRowContext(ctx, `
SELECT guildid
FROM guild
WHERE guildid = ?
FOR UPDATE`, guildID).Scan(&guildIDFound); err != nil {
		return err
	}

	var tabs uint8
	if err = tx.QueryRowContext(ctx, `
SELECT COUNT(*)
FROM guild_bank_tab
WHERE guildid = ?`, guildID).Scan(&tabs); err != nil {
		return err
	}
	if tabs >= GuildBankMaxTabs || tabID != tabs {
		return ErrGuildBankInvalidTab
	}

	if _, err = tx.ExecContext(ctx, `
INSERT INTO guild_bank_tab (guildid, TabId)
VALUES (?, ?)`, guildID, tabID); err != nil {
		return err
	}
	if err = insertGuildBankLogTx(ctx, tx, guildID, GuildBankLogBuySlot, tabID, 0, 0, 0, 0); err != nil {
		return err
	}

	return tx.Commit()
}

func (g *guildsMySQLRepo) DepositGuildBankMoney(ctx context.Context, realmID uint32, guildID uint64, memberGUID uint64, amount uint32) error {
	tx, err := g.db.DBByRealm(realmID).BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer rollbackTx(tx)

	var money uint64
	if err = tx.QueryRowContext(ctx, `
SELECT BankMoney
FROM guild
WHERE guildid = ?
FOR UPDATE`, guildID).Scan(&money); err != nil {
		return err
	}
	if money > GuildBankMoneyLimit-uint64(amount) {
		return ErrGuildBankFull
	}

	if _, err = tx.ExecContext(ctx, `
UPDATE guild
SET BankMoney = BankMoney + ?
WHERE guildid = ?`, amount, guildID); err != nil {
		return err
	}
	if err = insertGuildBankLogTx(ctx, tx, guildID, GuildBankLogDepositMoney, 0, memberGUID, amount, 0, 0); err != nil {
		return err
	}

	return tx.Commit()
}

func (g *guildsMySQLRepo) WithdrawGuildBankMoney(ctx context.Context, realmID uint32, guildID uint64, memberGUID uint64, amount uint32, repair bool) (uint32, error) {
	tx, err := g.db.DBByRealm(realmID).BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer rollbackTx(tx)

	var money uint64
	if err = tx.QueryRowContext(ctx, `
SELECT BankMoney
FROM guild
WHERE guildid = ?
FOR UPDATE`, guildID).Scan(&money); err != nil {
		return 0, err
	}
	if money < uint64(amount) {
		return 0, ErrGuildBankNotEnoughGold
	}

	if _, err = tx.ExecContext(ctx, `
UPDATE guild
SET BankMoney = BankMoney - ?
WHERE guildid = ?`, amount, guildID); err != nil {
		return 0, err
	}
	if err = incrementGuildBankWithdrawTx(ctx, tx, guildID, memberGUID, GuildBankMaxTabs, amount); err != nil {
		return 0, err
	}

	eventType := GuildBankLogWithdrawMoney
	if repair {
		eventType = GuildBankLogRepairMoney
	}
	logGUID, err := insertGuildBankLogReturningGUIDTx(ctx, tx, guildID, eventType, 0, memberGUID, amount, 0, 0)
	if err != nil {
		return 0, err
	}

	return logGUID, tx.Commit()
}

func (g *guildsMySQLRepo) RollbackGuildBankMoneyWithdraw(ctx context.Context, realmID uint32, guildID uint64, memberGUID uint64, amount uint32, repair bool, logGUID uint32) error {
	tx, err := g.db.DBByRealm(realmID).BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer rollbackTx(tx)

	eventType := GuildBankLogWithdrawMoney
	if repair {
		eventType = GuildBankLogRepairMoney
	}
	deleted, err := deleteGuildBankLogTx(ctx, tx, guildID, logGUID, eventType, 0, memberGUID, amount, 0, 0)
	if err != nil {
		return err
	}
	if !deleted {
		return tx.Commit()
	}

	if _, err = tx.ExecContext(ctx, `
UPDATE guild
SET BankMoney = BankMoney + ?
WHERE guildid = ?`, amount, guildID); err != nil {
		return err
	}
	if err = decrementGuildBankWithdrawTx(ctx, tx, memberGUID, GuildBankMaxTabs, amount); err != nil {
		return err
	}

	return tx.Commit()
}

func (g *guildsMySQLRepo) DepositGuildBankItem(ctx context.Context, realmID uint32, guildID uint64, memberGUID uint64, tabID, slotID uint8, item GuildBankItem) error {
	if tabID >= GuildBankMaxTabs {
		return ErrGuildBankInvalidTab
	}
	if slotID >= GuildBankMaxSlots {
		return ErrGuildBankInvalidSlot
	}

	tx, err := g.db.DBByRealm(realmID).BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer rollbackTx(tx)

	if err = lockGuildBankTabTx(ctx, tx, guildID, tabID); err != nil {
		return err
	}
	existingGuildID, existingTabID, existingSlotID, exists, err := guildBankItemLocationForUpdateTx(ctx, tx, item.ItemGUID)
	if err != nil {
		return err
	}
	if exists {
		if existingGuildID == guildID && existingTabID == tabID && existingSlotID == slotID {
			return tx.Commit()
		}
		return ErrGuildBankFull
	}
	tabItems, err := guildBankTabItemsForUpdateTx(ctx, tx, guildID, tabID)
	if err != nil {
		return err
	}
	maxStack, err := g.guildBankItemMaxStack(ctx, item.Entry)
	if err != nil {
		return err
	}
	placements, err := guildBankPlanStore(tabItems, nil, false, slotID, item.Entry, item.Count, maxStack)
	if errors.Is(err, errGuildBankItemCantStack) {
		return ErrGuildBankFull
	}
	if err != nil {
		return err
	}
	if err = applyGuildBankIncomingStoreTx(ctx, tx, guildID, tabID, item.ItemGUID, item.Count, placements); err != nil {
		return err
	}
	if err = insertGuildBankLogTx(ctx, tx, guildID, GuildBankLogDepositItem, tabID, memberGUID, item.Entry, uint16(item.Count), 0); err != nil {
		return err
	}

	return tx.Commit()
}

func (g *guildsMySQLRepo) WithdrawGuildBankItem(ctx context.Context, realmID uint32, guildID uint64, memberGUID uint64, tabID, slotID uint8, count uint32, splitItemGUID uint64) (*GuildBankItem, uint32, error) {
	if tabID >= GuildBankMaxTabs {
		return nil, 0, ErrGuildBankInvalidTab
	}
	if slotID >= GuildBankMaxSlots {
		return nil, 0, ErrGuildBankInvalidSlot
	}

	tx, err := g.db.DBByRealm(realmID).BeginTx(ctx, nil)
	if err != nil {
		return nil, 0, err
	}
	defer rollbackTx(tx)

	if err = lockGuildBankTabTx(ctx, tx, guildID, tabID); err != nil {
		return nil, 0, err
	}
	item, err := guildBankItemForUpdateTx(ctx, tx, guildID, tabID, slotID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, 0, ErrGuildBankItemNotFound
	}
	if err != nil {
		return nil, 0, err
	}
	moveCount := guildBankMoveCount(item.Count, count)

	if moveCount < item.Count {
		if splitItemGUID == 0 {
			return nil, 0, ErrGuildBankItemNotFound
		}
		if err = cloneGuildBankItemInstanceTx(ctx, tx, item.ItemGUID, splitItemGUID, moveCount); err != nil {
			return nil, 0, err
		}
		if err = setItemInstanceCountTx(ctx, tx, item.ItemGUID, item.Count-moveCount); err != nil {
			return nil, 0, err
		}
		item.ItemGUID = splitItemGUID
		item.Count = moveCount
	} else if _, err = tx.ExecContext(ctx, `
DELETE FROM guild_bank_item
WHERE guildid = ? AND TabId = ? AND SlotId = ?`, guildID, tabID, slotID); err != nil {
		return nil, 0, err
	}
	if err = incrementGuildBankWithdrawTx(ctx, tx, guildID, memberGUID, tabID, 1); err != nil {
		return nil, 0, err
	}
	logGUID, err := insertGuildBankLogReturningGUIDTx(ctx, tx, guildID, GuildBankLogWithdrawItem, tabID, memberGUID, item.Entry, uint16(moveCount), 0)
	if err != nil {
		return nil, 0, err
	}

	return item, logGUID, tx.Commit()
}

func (g *guildsMySQLRepo) RollbackGuildBankItemWithdraw(ctx context.Context, realmID uint32, guildID uint64, memberGUID uint64, tabID, slotID uint8, item GuildBankItem, logGUID uint32) ([]uint8, error) {
	if tabID >= GuildBankMaxTabs {
		return nil, ErrGuildBankInvalidTab
	}
	if slotID >= GuildBankMaxSlots {
		return nil, ErrGuildBankInvalidSlot
	}
	if item.ItemGUID == 0 || item.Entry == 0 || item.Count == 0 {
		return nil, ErrGuildBankItemNotFound
	}

	tx, err := g.db.DBByRealm(realmID).BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer rollbackTx(tx)

	if err = lockGuildBankTabTx(ctx, tx, guildID, tabID); err != nil {
		return nil, err
	}
	deleted, err := deleteGuildBankLogTx(ctx, tx, guildID, logGUID, GuildBankLogWithdrawItem, tabID, memberGUID, item.Entry, uint16(item.Count), 0)
	if err != nil {
		return nil, err
	}
	if !deleted {
		return nil, tx.Commit()
	}

	existingGuildID, _, existingSlotID, exists, err := guildBankItemLocationForUpdateTx(ctx, tx, item.ItemGUID)
	if err != nil {
		return nil, err
	}
	if exists {
		if existingGuildID == guildID {
			if err = decrementGuildBankWithdrawTx(ctx, tx, memberGUID, tabID, 1); err != nil {
				return nil, err
			}
			return uniqueGuildBankSlots(existingSlotID), tx.Commit()
		}
		return nil, ErrGuildBankFull
	}

	tabItems, err := guildBankTabItemsForUpdateTx(ctx, tx, guildID, tabID)
	if err != nil {
		return nil, err
	}
	maxStack, err := g.guildBankItemMaxStack(ctx, item.Entry)
	if err != nil {
		return nil, err
	}
	placements, err := guildBankPlanStore(tabItems, nil, false, slotID, item.Entry, item.Count, maxStack)
	if errors.Is(err, errGuildBankItemCantStack) {
		return nil, ErrGuildBankFull
	}
	if err != nil {
		return nil, err
	}
	if err = applyGuildBankIncomingStoreTx(ctx, tx, guildID, tabID, item.ItemGUID, item.Count, placements); err != nil {
		return nil, err
	}
	if err = decrementGuildBankWithdrawTx(ctx, tx, memberGUID, tabID, 1); err != nil {
		return nil, err
	}

	changed := make([]uint8, 0, len(placements))
	for _, placement := range placements {
		changed = append(changed, placement.slotID)
	}
	return uniqueGuildBankSlots(changed...), tx.Commit()
}

func (g *guildsMySQLRepo) MoveGuildBankItem(ctx context.Context, realmID uint32, guildID uint64, memberGUID uint64, sourceTabID, sourceSlotID, destinationTabID, destinationSlotID uint8, count uint32, splitItemGUID uint64) ([]uint8, error) {
	if sourceTabID >= GuildBankMaxTabs || destinationTabID >= GuildBankMaxTabs {
		return nil, ErrGuildBankInvalidTab
	}
	if sourceSlotID >= GuildBankMaxSlots || destinationSlotID >= GuildBankMaxSlots {
		return nil, ErrGuildBankInvalidSlot
	}
	if sourceTabID == destinationTabID && sourceSlotID == destinationSlotID {
		return []uint8{sourceSlotID}, nil
	}

	tx, err := g.db.DBByRealm(realmID).BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer rollbackTx(tx)

	if err = lockGuildBankTabsTx(ctx, tx, guildID, sourceTabID, destinationTabID); err != nil {
		return nil, err
	}
	source, err := guildBankItemForUpdateTx(ctx, tx, guildID, sourceTabID, sourceSlotID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrGuildBankItemNotFound
	}
	if err != nil {
		return nil, err
	}
	moveCount := guildBankMoveCount(source.Count, count)
	split := moveCount < source.Count
	if split && splitItemGUID == 0 {
		return nil, ErrGuildBankItemNotFound
	}
	maxStack, err := g.guildBankItemMaxStack(ctx, source.Entry)
	if err != nil {
		return nil, err
	}
	destTabItems, err := guildBankTabItemsForUpdateTx(ctx, tx, guildID, destinationTabID)
	if err != nil {
		return nil, err
	}
	sourceSlot := sourceSlotID
	sourceSlotAvailable := !split && sourceTabID == destinationTabID
	placements, err := guildBankPlanStore(destTabItems, &sourceSlot, sourceSlotAvailable, destinationSlotID, source.Entry, moveCount, maxStack)
	if errors.Is(err, errGuildBankItemCantStack) {
		if split {
			return nil, ErrGuildBankFull
		}
		if err = swapGuildBankItemsTx(ctx, tx, guildID, sourceTabID, sourceSlotID, destinationTabID, destinationSlotID); err != nil {
			return nil, err
		}
		if sourceTabID != destinationTabID {
			if err = incrementGuildBankWithdrawTx(ctx, tx, guildID, memberGUID, sourceTabID, 1); err != nil {
				return nil, err
			}
		}
		if err = insertGuildBankLogTx(ctx, tx, guildID, GuildBankLogMoveItem, sourceTabID, memberGUID, source.Entry, uint16(moveCount), destinationTabID); err != nil {
			return nil, err
		}
		return uniqueGuildBankSlots(sourceSlotID, destinationSlotID), tx.Commit()
	}
	if err != nil {
		return nil, err
	}

	if split {
		if err = applyGuildBankSplitMoveTx(ctx, tx, guildID, source, sourceTabID, sourceSlotID, destinationTabID, splitItemGUID, moveCount, placements); err != nil {
			return nil, err
		}
	} else if err = applyGuildBankFullMoveTx(ctx, tx, guildID, source, sourceTabID, sourceSlotID, destinationTabID, placements); err != nil {
		return nil, err
	}

	if sourceTabID != destinationTabID {
		if err = incrementGuildBankWithdrawTx(ctx, tx, guildID, memberGUID, sourceTabID, 1); err != nil {
			return nil, err
		}
	}
	if err = insertGuildBankLogTx(ctx, tx, guildID, GuildBankLogMoveItem, sourceTabID, memberGUID, source.Entry, uint16(moveCount), destinationTabID); err != nil {
		return nil, err
	}

	changed := []uint8{sourceSlotID}
	for _, placement := range placements {
		changed = append(changed, placement.slotID)
	}
	return uniqueGuildBankSlots(changed...), tx.Commit()
}

type guildBankStorePlacement struct {
	slotID       uint8
	existingGUID uint64
	count        uint32
}

func guildBankMoveCount(sourceCount, requestedCount uint32) uint32 {
	if requestedCount == 0 || requestedCount >= sourceCount {
		return sourceCount
	}
	return requestedCount
}

func guildBankPlanStore(tabItems map[uint8]*GuildBankItem, sourceSlot *uint8, sourceSlotAvailable bool, destinationSlot uint8, entry uint32, count uint32, maxStack uint32) ([]guildBankStorePlacement, error) {
	if count == 0 {
		return nil, ErrGuildBankItemNotFound
	}
	if maxStack == 0 {
		return nil, ErrGuildBankItemNotFound
	}

	placements := make([]guildBankStorePlacement, 0, 2)
	remaining := count
	skipMerge := map[uint8]bool{destinationSlot: true}
	skipEmpty := map[uint8]bool{destinationSlot: true}
	if sourceSlot != nil {
		skipMerge[*sourceSlot] = true
		if !sourceSlotAvailable {
			skipEmpty[*sourceSlot] = true
		}
	}

	if destination := tabItems[destinationSlot]; destination != nil {
		if destination.Entry != entry || destination.Count >= maxStack {
			return nil, errGuildBankItemCantStack
		}
		free := maxStack - destination.Count
		if free > remaining {
			free = remaining
		}
		placements = append(placements, guildBankStorePlacement{
			slotID:       destinationSlot,
			existingGUID: destination.ItemGUID,
			count:        free,
		})
		remaining -= free
	} else {
		free := maxStack
		if free > remaining {
			free = remaining
		}
		placements = append(placements, guildBankStorePlacement{
			slotID: destinationSlot,
			count:  free,
		})
		remaining -= free
	}
	if remaining == 0 {
		return placements, nil
	}

	if maxStack > 1 {
		for slotID := uint8(0); slotID < GuildBankMaxSlots && remaining > 0; slotID++ {
			if skipMerge[slotID] {
				continue
			}
			item := tabItems[slotID]
			if item == nil || item.Entry != entry || item.Count >= maxStack {
				continue
			}
			free := maxStack - item.Count
			if free > remaining {
				free = remaining
			}
			placements = append(placements, guildBankStorePlacement{
				slotID:       slotID,
				existingGUID: item.ItemGUID,
				count:        free,
			})
			remaining -= free
		}
	}

	for slotID := uint8(0); slotID < GuildBankMaxSlots && remaining > 0; slotID++ {
		sourceSlotIsAvailable := sourceSlot != nil && *sourceSlot == slotID && sourceSlotAvailable
		if skipEmpty[slotID] || (tabItems[slotID] != nil && !sourceSlotIsAvailable) {
			continue
		}
		free := maxStack
		if free > remaining {
			free = remaining
		}
		placements = append(placements, guildBankStorePlacement{
			slotID: slotID,
			count:  free,
		})
		remaining -= free
	}
	if remaining != 0 {
		return nil, ErrGuildBankFull
	}

	return placements, nil
}

func applyGuildBankIncomingStoreTx(ctx context.Context, tx *sql.Tx, guildID uint64, tabID uint8, itemGUID uint64, itemCount uint32, placements []guildBankStorePlacement) error {
	remainingForNewSlot := itemCount
	var emptyPlacement *guildBankStorePlacement
	for i := range placements {
		placement := placements[i]
		if placement.existingGUID == 0 {
			if emptyPlacement != nil {
				return ErrGuildBankFull
			}
			emptyPlacement = &placements[i]
			continue
		}
		if err := addItemInstanceCountTx(ctx, tx, placement.existingGUID, placement.count); err != nil {
			return err
		}
		remainingForNewSlot -= placement.count
	}

	if emptyPlacement == nil {
		return deleteItemInstanceTx(ctx, tx, itemGUID)
	}
	if remainingForNewSlot != emptyPlacement.count {
		return ErrGuildBankFull
	}
	if err := setItemInstanceOwnerAndCountTx(ctx, tx, itemGUID, 0, emptyPlacement.count); err != nil {
		return err
	}
	return insertGuildBankItemTx(ctx, tx, guildID, tabID, emptyPlacement.slotID, itemGUID)
}

func applyGuildBankSplitMoveTx(ctx context.Context, tx *sql.Tx, guildID uint64, source *GuildBankItem, sourceTabID, sourceSlotID, destinationTabID uint8, splitItemGUID uint64, moveCount uint32, placements []guildBankStorePlacement) error {
	if err := setItemInstanceCountTx(ctx, tx, source.ItemGUID, source.Count-moveCount); err != nil {
		return err
	}

	remainingForNewSlot := moveCount
	var emptyPlacement *guildBankStorePlacement
	for i := range placements {
		placement := placements[i]
		if placement.existingGUID == 0 {
			if emptyPlacement != nil {
				return ErrGuildBankFull
			}
			emptyPlacement = &placements[i]
			continue
		}
		if err := addItemInstanceCountTx(ctx, tx, placement.existingGUID, placement.count); err != nil {
			return err
		}
		remainingForNewSlot -= placement.count
	}
	if emptyPlacement == nil {
		return nil
	}
	if remainingForNewSlot != emptyPlacement.count {
		return ErrGuildBankFull
	}
	if err := cloneGuildBankItemInstanceTx(ctx, tx, source.ItemGUID, splitItemGUID, emptyPlacement.count); err != nil {
		return err
	}
	return insertGuildBankItemTx(ctx, tx, guildID, destinationTabID, emptyPlacement.slotID, splitItemGUID)
}

func applyGuildBankFullMoveTx(ctx context.Context, tx *sql.Tx, guildID uint64, source *GuildBankItem, sourceTabID, sourceSlotID, destinationTabID uint8, placements []guildBankStorePlacement) error {
	remainingForNewSlot := source.Count
	var emptyPlacement *guildBankStorePlacement
	for i := range placements {
		placement := placements[i]
		if placement.existingGUID == 0 {
			if emptyPlacement != nil {
				return ErrGuildBankFull
			}
			emptyPlacement = &placements[i]
			continue
		}
		if err := addItemInstanceCountTx(ctx, tx, placement.existingGUID, placement.count); err != nil {
			return err
		}
		remainingForNewSlot -= placement.count
	}

	if emptyPlacement == nil {
		if _, err := tx.ExecContext(ctx, `
DELETE FROM guild_bank_item
WHERE guildid = ? AND TabId = ? AND SlotId = ?`, guildID, sourceTabID, sourceSlotID); err != nil {
			return err
		}
		return deleteItemInstanceTx(ctx, tx, source.ItemGUID)
	}
	if remainingForNewSlot != emptyPlacement.count {
		return ErrGuildBankFull
	}
	if err := setItemInstanceOwnerAndCountTx(ctx, tx, source.ItemGUID, 0, emptyPlacement.count); err != nil {
		return err
	}
	_, err := tx.ExecContext(ctx, `
UPDATE guild_bank_item
SET TabId = ?, SlotId = ?
WHERE guildid = ? AND TabId = ? AND SlotId = ?`, destinationTabID, emptyPlacement.slotID, guildID, sourceTabID, sourceSlotID)
	return err
}

func swapGuildBankItemsTx(ctx context.Context, tx *sql.Tx, guildID uint64, sourceTabID, sourceSlotID, destinationTabID, destinationSlotID uint8) error {
	source, dest, err := guildBankMoveItemsForUpdateTx(ctx, tx, guildID, sourceTabID, sourceSlotID, destinationTabID, destinationSlotID)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrGuildBankItemNotFound
	}
	if err != nil {
		return err
	}
	if source == nil || dest == nil {
		return ErrGuildBankItemNotFound
	}
	if _, err = tx.ExecContext(ctx, `
DELETE FROM guild_bank_item
WHERE guildid = ? AND ((TabId = ? AND SlotId = ?) OR (TabId = ? AND SlotId = ?))`,
		guildID, sourceTabID, sourceSlotID, destinationTabID, destinationSlotID); err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `
INSERT INTO guild_bank_item (guildid, TabId, SlotId, item_guid)
VALUES (?, ?, ?, ?), (?, ?, ?, ?)`,
		guildID, destinationTabID, destinationSlotID, source.ItemGUID,
		guildID, sourceTabID, sourceSlotID, dest.ItemGUID)
	return err
}

func uniqueGuildBankSlots(slots ...uint8) []uint8 {
	if len(slots) == 0 {
		return nil
	}
	seen := map[uint8]bool{}
	out := make([]uint8, 0, len(slots))
	for _, slot := range slots {
		if seen[slot] {
			continue
		}
		seen[slot] = true
		out = append(out, slot)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

func firstItemCharge(charges string) uint32 {
	fields := strings.Fields(charges)
	if len(fields) == 0 {
		return 0
	}
	v, _ := strconv.ParseInt(fields[0], 10, 32)
	if v < 0 {
		return uint32(-v)
	}
	return uint32(v)
}

func guildBankEnchants(enchantments string) (uint32, []GuildBankSocketEnchant) {
	fields := strings.Fields(enchantments)
	if len(fields) < 3 {
		return 0, nil
	}

	enchantID := parseEnchant(fields, 0)
	var sockets []GuildBankSocketEnchant
	for socketIndex := 0; socketIndex < 3; socketIndex++ {
		value := parseEnchant(fields, (2+socketIndex)*3)
		if value != 0 {
			sockets = append(sockets, GuildBankSocketEnchant{
				SocketIndex:     uint8(socketIndex),
				SocketEnchantID: value,
			})
		}
	}
	return enchantID, sockets
}

func parseEnchant(fields []string, index int) uint32 {
	if index < 0 || index >= len(fields) {
		return 0
	}
	v, _ := strconv.ParseUint(fields[index], 10, 32)
	return uint32(v)
}

func rollbackTx(tx *sql.Tx) {
	_ = tx.Rollback()
}

func lockGuildBankTabTx(ctx context.Context, tx *sql.Tx, guildID uint64, tabID uint8) error {
	var found uint8
	err := tx.QueryRowContext(ctx, `
SELECT TabId
FROM guild_bank_tab
WHERE guildid = ? AND TabId = ?
FOR UPDATE`, guildID, tabID).Scan(&found)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrGuildBankInvalidTab
	}
	return err
}

func lockGuildBankTabsTx(ctx context.Context, tx *sql.Tx, guildID uint64, tabs ...uint8) error {
	seen := map[uint8]bool{}
	uniqueTabs := make([]int, 0, len(tabs))
	for _, tab := range tabs {
		if seen[tab] {
			continue
		}
		seen[tab] = true
		uniqueTabs = append(uniqueTabs, int(tab))
	}
	sort.Ints(uniqueTabs)
	for _, tab := range uniqueTabs {
		if err := lockGuildBankTabTx(ctx, tx, guildID, uint8(tab)); err != nil {
			return err
		}
	}
	return nil
}

func (g *guildsMySQLRepo) guildBankItemMaxStack(ctx context.Context, entry uint32) (uint32, error) {
	if g.worldDB == nil {
		return 1, nil
	}

	var stackable int32
	err := g.worldDB.QueryRowContext(ctx, `
SELECT stackable
FROM item_template
WHERE entry = ?`, entry).Scan(&stackable)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, ErrGuildBankItemNotFound
	}
	if err != nil {
		return 0, err
	}
	if stackable == 2147483647 || stackable <= 0 {
		return 0x7FFFFFFF - 1, nil
	}
	return uint32(stackable), nil
}

func guildBankSlotOccupiedTx(ctx context.Context, tx *sql.Tx, guildID uint64, tabID, slotID uint8) (bool, error) {
	var itemGUID uint64
	err := tx.QueryRowContext(ctx, `
SELECT item_guid
FROM guild_bank_item
WHERE guildid = ? AND TabId = ? AND SlotId = ?
FOR UPDATE`, guildID, tabID, slotID).Scan(&itemGUID)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return err == nil, err
}

func guildBankItemLocationForUpdateTx(ctx context.Context, tx *sql.Tx, itemGUID uint64) (uint64, uint8, uint8, bool, error) {
	var guildID uint64
	var tabID uint8
	var slotID uint8
	err := tx.QueryRowContext(ctx, `
SELECT guildid, TabId, SlotId
FROM guild_bank_item
WHERE item_guid = ?
FOR UPDATE`, itemGUID).Scan(&guildID, &tabID, &slotID)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, 0, 0, false, nil
	}
	if err != nil {
		return 0, 0, 0, false, err
	}
	return guildID, tabID, slotID, true, nil
}

func guildBankTabItemsForUpdateTx(ctx context.Context, tx *sql.Tx, guildID uint64, tabID uint8) (map[uint8]*GuildBankItem, error) {
	rows, err := tx.QueryContext(ctx, `
SELECT
	bi.SlotId,
	ii.guid, ii.itemEntry, ii.count, ii.flags, ii.randomPropertyId, ii.durability,
	ii.charges, ii.enchantments, COALESCE(ii.text, '')
FROM guild_bank_item bi
INNER JOIN item_instance ii ON ii.guid = bi.item_guid
WHERE bi.guildid = ? AND bi.TabId = ?
ORDER BY bi.SlotId ASC
FOR UPDATE`, guildID, tabID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := map[uint8]*GuildBankItem{}
	for rows.Next() {
		item := GuildBankItem{}
		var charges string
		var enchantments string
		if err = rows.Scan(
			&item.Slot,
			&item.ItemGUID, &item.Entry, &item.Count, &item.Flags, &item.RandomPropertyID, &item.Durability,
			&charges, &enchantments, &item.Text,
		); err != nil {
			return nil, err
		}
		item.Charges = firstItemCharge(charges)
		item.EnchantmentID, item.SocketEnchants = guildBankEnchants(enchantments)
		items[item.Slot] = &item
	}
	return items, rows.Err()
}

func insertGuildBankItemTx(ctx context.Context, tx *sql.Tx, guildID uint64, tabID, slotID uint8, itemGUID uint64) error {
	_, err := tx.ExecContext(ctx, `
INSERT INTO guild_bank_item (guildid, TabId, SlotId, item_guid)
VALUES (?, ?, ?, ?)`, guildID, tabID, slotID, itemGUID)
	return err
}

func cloneGuildBankItemInstanceTx(ctx context.Context, tx *sql.Tx, sourceItemGUID, newItemGUID uint64, count uint32) error {
	res, err := tx.ExecContext(ctx, `
INSERT INTO item_instance (guid, itemEntry, owner_guid, creatorGuid, giftCreatorGuid, count, duration, charges, flags, enchantments, randomPropertyId, durability, playedTime, text)
SELECT ?, itemEntry, 0, creatorGuid, giftCreatorGuid, ?, duration, charges, flags, enchantments, randomPropertyId, durability, playedTime, text
FROM item_instance
WHERE guid = ?`, newItemGUID, count, sourceItemGUID)
	if err != nil {
		return err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return ErrGuildBankItemNotFound
	}
	return nil
}

func setItemInstanceCountTx(ctx context.Context, tx *sql.Tx, itemGUID uint64, count uint32) error {
	_, err := tx.ExecContext(ctx, `
UPDATE item_instance
SET count = ?
WHERE guid = ?`, count, itemGUID)
	return err
}

func addItemInstanceCountTx(ctx context.Context, tx *sql.Tx, itemGUID uint64, count uint32) error {
	_, err := tx.ExecContext(ctx, `
UPDATE item_instance
SET count = count + ?
WHERE guid = ?`, count, itemGUID)
	return err
}

func setItemInstanceOwnerAndCountTx(ctx context.Context, tx *sql.Tx, itemGUID uint64, ownerGUID uint64, count uint32) error {
	_, err := tx.ExecContext(ctx, `
UPDATE item_instance
SET owner_guid = ?, count = ?
WHERE guid = ?`, ownerGUID, count, itemGUID)
	return err
}

func deleteItemInstanceTx(ctx context.Context, tx *sql.Tx, itemGUID uint64) error {
	_, err := tx.ExecContext(ctx, `
DELETE FROM item_instance
WHERE guid = ?`, itemGUID)
	return err
}

func guildBankItemForUpdateTx(ctx context.Context, tx *sql.Tx, guildID uint64, tabID, slotID uint8) (*GuildBankItem, error) {
	var item GuildBankItem
	var charges string
	var enchantments string
	err := tx.QueryRowContext(ctx, `
SELECT
	ii.guid, ii.itemEntry, bi.SlotId, ii.count, ii.flags, ii.randomPropertyId, ii.durability,
	ii.charges, ii.enchantments, COALESCE(ii.text, '')
FROM guild_bank_item bi
INNER JOIN item_instance ii ON ii.guid = bi.item_guid
WHERE bi.guildid = ? AND bi.TabId = ? AND bi.SlotId = ?
FOR UPDATE`, guildID, tabID, slotID).Scan(
		&item.ItemGUID, &item.Entry, &item.Slot, &item.Count, &item.Flags, &item.RandomPropertyID, &item.Durability,
		&charges, &enchantments, &item.Text,
	)
	if err != nil {
		return nil, err
	}
	item.Charges = firstItemCharge(charges)
	item.EnchantmentID, item.SocketEnchants = guildBankEnchants(enchantments)
	return &item, nil
}

func guildBankMoveItemsForUpdateTx(ctx context.Context, tx *sql.Tx, guildID uint64, sourceTabID, sourceSlotID, destinationTabID, destinationSlotID uint8) (*GuildBankItem, *GuildBankItem, error) {
	rows, err := tx.QueryContext(ctx, `
SELECT
	bi.TabId, bi.SlotId,
	ii.guid, ii.itemEntry, ii.count, ii.flags, ii.randomPropertyId, ii.durability,
	ii.charges, ii.enchantments, COALESCE(ii.text, '')
FROM guild_bank_item bi
INNER JOIN item_instance ii ON ii.guid = bi.item_guid
WHERE bi.guildid = ? AND ((bi.TabId = ? AND bi.SlotId = ?) OR (bi.TabId = ? AND bi.SlotId = ?))
ORDER BY bi.TabId ASC, bi.SlotId ASC
FOR UPDATE`, guildID, sourceTabID, sourceSlotID, destinationTabID, destinationSlotID)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	var source *GuildBankItem
	var dest *GuildBankItem
	for rows.Next() {
		var tabID uint8
		var item GuildBankItem
		var charges string
		var enchantments string
		if err = rows.Scan(
			&tabID, &item.Slot,
			&item.ItemGUID, &item.Entry, &item.Count, &item.Flags, &item.RandomPropertyID, &item.Durability,
			&charges, &enchantments, &item.Text,
		); err != nil {
			return nil, nil, err
		}
		item.Charges = firstItemCharge(charges)
		item.EnchantmentID, item.SocketEnchants = guildBankEnchants(enchantments)
		if tabID == sourceTabID && item.Slot == sourceSlotID {
			source = &item
		} else if tabID == destinationTabID && item.Slot == destinationSlotID {
			dest = &item
		}
	}
	if err = rows.Err(); err != nil {
		return nil, nil, err
	}
	if source == nil {
		return nil, nil, sql.ErrNoRows
	}
	return source, dest, nil
}

func incrementGuildBankWithdrawTx(ctx context.Context, tx *sql.Tx, guildID uint64, memberGUID uint64, tabID uint8, amount uint32) error {
	if tabID > GuildBankMaxTabs {
		return ErrGuildBankInvalidTab
	}

	column := "money"
	if tabID < GuildBankMaxTabs {
		column = "tab" + strconv.Itoa(int(tabID))
	}

	_, err := tx.ExecContext(ctx, `
INSERT INTO guild_member_withdraw (guid, tab0, tab1, tab2, tab3, tab4, tab5, money)
VALUES (?, 0, 0, 0, 0, 0, 0, 0)
ON DUPLICATE KEY UPDATE guid = VALUES(guid)`, memberGUID)
	if err != nil {
		return err
	}

	limit, used, err := guildBankWithdrawLimitForUpdateTx(ctx, tx, guildID, memberGUID, tabID, column)
	if err != nil {
		return err
	}
	if limit != 0xFFFFFFFF && (used >= limit || amount > limit-used) {
		return ErrGuildBankWithdrawLimit
	}

	_, err = tx.ExecContext(ctx, "UPDATE guild_member_withdraw SET "+column+" = "+column+" + ? WHERE guid = ?", amount, memberGUID)
	return err
}

func decrementGuildBankWithdrawTx(ctx context.Context, tx *sql.Tx, memberGUID uint64, tabID uint8, amount uint32) error {
	if tabID > GuildBankMaxTabs {
		return ErrGuildBankInvalidTab
	}

	column := "money"
	if tabID < GuildBankMaxTabs {
		column = "tab" + strconv.Itoa(int(tabID))
	}

	_, err := tx.ExecContext(ctx, "UPDATE guild_member_withdraw SET "+column+" = CASE WHEN "+column+" > ? THEN "+column+" - ? ELSE 0 END WHERE guid = ?", amount, amount, memberGUID)
	return err
}

func guildBankWithdrawLimitForUpdateTx(ctx context.Context, tx *sql.Tx, guildID uint64, memberGUID uint64, tabID uint8, column string) (uint32, uint32, error) {
	if tabID == GuildBankMaxTabs {
		var limit uint32
		var used uint32
		err := tx.QueryRowContext(ctx, `
SELECT gr.BankMoneyPerDay, COALESCE(w.money, 0)
FROM guild_member gm
INNER JOIN guild_rank gr ON gr.guildid = gm.guildid AND gr.rid = gm.rank
LEFT JOIN guild_member_withdraw w ON w.guid = gm.guid
WHERE gm.guildid = ? AND gm.guid = ?
FOR UPDATE`, guildID, memberGUID).Scan(&limit, &used)
		return limit, used, err
	}

	var flags uint32
	var limit uint32
	var used uint32
	err := tx.QueryRowContext(ctx, `
SELECT gbr.gbright, gbr.SlotPerDay, COALESCE(w.`+column+`, 0)
FROM guild_member gm
INNER JOIN guild_bank_right gbr ON gbr.guildid = gm.guildid AND gbr.rid = gm.rank AND gbr.TabId = ?
LEFT JOIN guild_member_withdraw w ON w.guid = gm.guid
WHERE gm.guildid = ? AND gm.guid = ?
FOR UPDATE`, tabID, guildID, memberGUID).Scan(&flags, &limit, &used)
	if err != nil {
		return 0, 0, err
	}
	if flags&GuildBankRightViewTab == 0 {
		return 0, 0, ErrGuildBankWithdrawLimit
	}
	return limit, used, nil
}

func insertGuildBankLogTx(ctx context.Context, tx *sql.Tx, guildID uint64, eventType GuildBankEventLogType, tabID uint8, playerGUID uint64, itemOrMoney uint32, itemStackCount uint16, destTabID uint8) error {
	_, err := insertGuildBankLogReturningGUIDTx(ctx, tx, guildID, eventType, tabID, playerGUID, itemOrMoney, itemStackCount, destTabID)
	return err
}

func insertGuildBankLogReturningGUIDTx(ctx context.Context, tx *sql.Tx, guildID uint64, eventType GuildBankEventLogType, tabID uint8, playerGUID uint64, itemOrMoney uint32, itemStackCount uint16, destTabID uint8) (uint32, error) {
	if err := lockGuildBankLogTx(ctx, tx, guildID); err != nil {
		return 0, err
	}

	var logGUID uint32
	err := tx.QueryRowContext(ctx, `
SELECT COALESCE(MAX(LogGuid), 0) + 1
FROM guild_bank_eventlog
WHERE guildid = ? AND TabId = ?`, guildID, tabID).Scan(&logGUID)
	if err != nil {
		return 0, err
	}

	_, err = tx.ExecContext(ctx, `
INSERT INTO guild_bank_eventlog (guildid, LogGuid, TabId, EventType, PlayerGuid, ItemOrMoney, ItemStackCount, DestTabId, TimeStamp)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		guildID, logGUID, tabID, eventType, playerGUID, itemOrMoney, itemStackCount, destTabID, time.Now().Unix(),
	)
	if err != nil {
		return 0, err
	}
	return logGUID, nil
}

func deleteGuildBankLogTx(ctx context.Context, tx *sql.Tx, guildID uint64, logGUID uint32, eventType GuildBankEventLogType, tabID uint8, playerGUID uint64, itemOrMoney uint32, itemStackCount uint16, destTabID uint8) (bool, error) {
	if logGUID == 0 {
		return false, nil
	}
	if err := lockGuildBankLogTx(ctx, tx, guildID); err != nil {
		return false, err
	}

	res, err := tx.ExecContext(ctx, `
DELETE FROM guild_bank_eventlog
WHERE guildid = ? AND LogGuid = ? AND TabId = ? AND EventType = ? AND PlayerGuid = ? AND ItemOrMoney = ? AND ItemStackCount = ? AND DestTabId = ?`,
		guildID, logGUID, tabID, eventType, playerGUID, itemOrMoney, itemStackCount, destTabID)
	if err != nil {
		return false, err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	return affected > 0, nil
}

func lockGuildBankLogTx(ctx context.Context, tx *sql.Tx, guildID uint64) error {
	var found uint64
	return tx.QueryRowContext(ctx, `
SELECT guildid
FROM guild
WHERE guildid = ?
FOR UPDATE`, guildID).Scan(&found)
}
