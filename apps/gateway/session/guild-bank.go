package session

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	root "github.com/walkline/ToCloud9/apps/gateway"
	eBroadcaster "github.com/walkline/ToCloud9/apps/gateway/events-broadcaster"
	"github.com/walkline/ToCloud9/apps/gateway/packet"
	pbGuild "github.com/walkline/ToCloud9/gen/guilds/pb"
	pbGameServ "github.com/walkline/ToCloud9/gen/worldserver/pb"
	"github.com/walkline/ToCloud9/shared/events"
)

const (
	gameObjectTypeGuildBank = 34

	guildBankMaxTabs  = 6
	guildBankSlotAuto = 0xFF

	guildCommandViewTab  = 21 // GUILD_COMMAND_VIEW_TAB
	guildCommandMoveItem = 22 // GUILD_COMMAND_MOVE_ITEM

	guildErrInternal       = 1  // ERR_GUILD_INTERNAL
	guildErrWithdrawLimit  = 25 // ERR_GUILD_WITHDRAW_LIMIT
	guildErrNotEnoughMoney = 26 // ERR_GUILD_NOT_ENOUGH_MONEY
	guildErrBankFull       = 28 // ERR_GUILD_BANK_FULL
	guildErrItemNotFound   = 29 // ERR_GUILD_ITEM_NOT_FOUND

	equipErrInventoryFull = 50 // EQUIP_ERR_INVENTORY_FULL

	// Guild bank event log types (AC GuildBankEventLogTypes).
	guildBankLogDepositItem  = 1
	guildBankLogWithdrawItem = 2
	guildBankLogMoveItem     = 3
	guildBankLogMoveItem2    = 7
)

// canUseGuildBank validates the proximity to the guild bank chest the client
// claims to interact with.
func (s *GameSession) canUseGuildBank(ctx context.Context, bankerGUID uint64) bool {
	resp, err := s.gameServerGRPCClient.CanPlayerInteractWithGameObject(ctx, &pbGameServ.CanPlayerInteractWithGameObjectRequest{
		Api:            root.Ver,
		PlayerGuid:     s.character.GUID,
		GameObjectGuid: bankerGUID,
		GameObjectType: gameObjectTypeGuildBank,
	})
	if err != nil {
		return false
	}
	return resp.CanInteract
}

func (s *GameSession) guildBankState(ctx context.Context) (*pbGuild.GetBankStateResponse, error) {
	return s.guildServiceClient.GetBankState(ctx, &pbGuild.GetBankStateParams{
		Api:        root.Ver,
		RealmID:    root.RealmID,
		GuildID:    uint64(s.character.GuildID),
		PlayerGUID: s.character.GUID,
	})
}

// sendGuildBankList sends SMSG_GUILD_BANK_LIST for one tab. When fullUpdate is
// true the packet is a complete refresh of the tab and the item list carries
// every occupied slot, so the client clears and re-renders the tab.
//
// The 3.3.5 client REQUIRES the tab-info block (names/icons) whenever
// fullUpdate is set on tab 0: presence of that block is therefore tied
// strictly to (fullUpdate && tab==0). Sending fullUpdate without it — as an
// earlier version did on the post-change refresh — shifts the whole packet,
// so the client reads the item bytes as tab names (garbage tab title) and
// never renders the items.
func (s *GameSession) sendGuildBankList(ctx context.Context, tab uint8, fullUpdate bool) error {
	state, err := s.guildBankState(ctx)
	if err != nil {
		return err
	}

	if int(tab) >= len(state.Tabs) && tab != 0 {
		return nil
	}

	var items []*pbGuild.GuildBankItem
	if int(tab) < len(state.Tabs) {
		tabResp, err := s.guildServiceClient.GetBankTab(ctx, &pbGuild.GetBankTabParams{
			Api:        root.Ver,
			RealmID:    root.RealmID,
			GuildID:    uint64(s.character.GuildID),
			PlayerGUID: s.character.GUID,
			Tab:        uint32(tab),
		})
		if err != nil {
			// No view right just means an empty item list; other errors abort.
			if status.Code(err) != codes.PermissionDenied {
				return err
			}
		} else {
			items = tabResp.Items
		}
	}

	var remaining int32
	if int(tab) < len(state.Tabs) {
		remaining = int32(state.Tabs[tab].RemainingSlots)
	}

	fu := uint8(0)
	if fullUpdate {
		fu = 1
	}

	resp := packet.NewWriterWithSize(packet.SMsgGuildBankList, 0)
	resp.Uint64(state.Money)
	resp.Uint8(tab)
	resp.Int32(remaining)
	resp.Uint8(fu)
	if fullUpdate && tab == 0 {
		resp.Uint8(uint8(len(state.Tabs)))
		for _, t := range state.Tabs {
			resp.String(t.Name)
			resp.String(t.Icon)
		}
	}

	resp.Uint8(uint8(len(items)))
	for _, item := range items {
		writeBankItem(resp, uint8(item.Slot), item)
	}

	s.gameSocket.Send(resp)
	return nil
}

// writeBankItem appends one SMSG_GUILD_BANK_LIST item entry. A nil item writes
// an empty slot (ItemID 0), which the client uses to clear a slot on a partial
// update — a full update omits empty slots, so a withdrawn item is never
// cleared that way while the tab is already open.
func writeBankItem(resp *packet.Writer, slot uint8, item *pbGuild.GuildBankItem) {
	resp.Uint8(slot)
	if item == nil || item.Entry == 0 {
		resp.Uint32(0)
		return
	}
	resp.Uint32(item.Entry)
	resp.Int32(int32(item.Flags))
	resp.Int32(item.RandomPropertyID)
	if item.RandomPropertyID != 0 {
		// Suffix factor is not tracked by the cluster bank yet.
		resp.Int32(0)
	}
	resp.Int32(int32(item.Count))
	resp.Int32(int32(item.EnchantmentID))
	resp.Uint8(uint8(item.Charges))

	var gems []struct {
		idx uint8
		id  uint32
	}
	for i, enchID := range item.SocketEnchantIDs {
		if enchID != 0 {
			gems = append(gems, struct {
				idx uint8
				id  uint32
			}{uint8(i), enchID})
		}
	}
	resp.Uint8(uint8(len(gems)))
	for _, gem := range gems {
		resp.Uint8(gem.idx)
		resp.Int32(int32(gem.id))
	}
}

// sendGuildBankSlots sends a partial SMSG_GUILD_BANK_LIST (fullUpdate=0) for the
// given slots of a tab, each with its current content (empty when nothing sits
// there). This is how AC refreshes a slot after a deposit/withdraw/move: unlike
// a full update, it clears an emptied slot, so a withdrawn item disappears from
// an already-open bank window.
func (s *GameSession) sendGuildBankSlots(ctx context.Context, tab uint8, slots ...uint8) error {
	if len(slots) == 0 {
		return nil
	}

	state, err := s.guildBankState(ctx)
	if err != nil {
		return err
	}
	if int(tab) >= len(state.Tabs) {
		return nil
	}

	tabResp, err := s.guildServiceClient.GetBankTab(ctx, &pbGuild.GetBankTabParams{
		Api:        root.Ver,
		RealmID:    root.RealmID,
		GuildID:    uint64(s.character.GuildID),
		PlayerGUID: s.character.GUID,
		Tab:        uint32(tab),
	})
	if err != nil {
		return err
	}
	bySlot := make(map[uint8]*pbGuild.GuildBankItem, len(tabResp.Items))
	for _, item := range tabResp.Items {
		bySlot[uint8(item.Slot)] = item
	}

	resp := packet.NewWriterWithSize(packet.SMsgGuildBankList, 0)
	resp.Uint64(state.Money)
	resp.Uint8(tab)
	resp.Int32(int32(state.Tabs[tab].RemainingSlots))
	resp.Uint8(0) // partial update, no tab-info block
	resp.Uint8(uint8(len(slots)))
	for _, slot := range slots {
		writeBankItem(resp, slot, bySlot[slot])
	}

	s.gameSocket.Send(resp)
	return nil
}

func (s *GameSession) HandleGuildBankerActivate(ctx context.Context, p *packet.Packet) error {
	r := p.Reader()
	banker := r.Uint64()
	fullUpdate := r.Uint8() != 0

	if !s.canUseGuildBank(ctx, banker) {
		return nil
	}
	if s.character.GuildID == 0 {
		s.sendGuildCommandResult(guildCommandViewTab, "", guildErrPlayerNotInGuild)
		return nil
	}

	s.guildBankOpen = true

	return s.sendGuildBankList(ctx, 0, fullUpdate)
}

func (s *GameSession) HandleGuildBankQueryTab(ctx context.Context, p *packet.Packet) error {
	r := p.Reader()
	banker := r.Uint64()
	tab := r.Uint8()
	fullUpdate := r.Uint8() != 0

	if !s.canUseGuildBank(ctx, banker) || s.character.GuildID == 0 {
		return nil
	}

	return s.sendGuildBankList(ctx, tab, fullUpdate)
}

func (s *GameSession) HandleGuildBankDepositMoney(ctx context.Context, p *packet.Packet) error {
	r := p.Reader()
	banker := r.Uint64()
	amount := r.Uint32()

	if amount == 0 || !s.canUseGuildBank(ctx, banker) || s.character.GuildID == 0 {
		return nil
	}

	// Debit first; the worldserver refuses when the player lacks the money.
	if _, err := s.gameServerGRPCClient.ModifyMoneyForPlayer(ctx, &pbGameServ.ModifyMoneyForPlayerRequest{
		Api:        root.Ver,
		PlayerGuid: s.character.GUID,
		Value:      -int32(amount),
	}); err != nil {
		return nil
	}

	_, err := s.guildServiceClient.BankDepositMoney(ctx, &pbGuild.BankMoneyParams{
		Api:        root.Ver,
		RealmID:    root.RealmID,
		GuildID:    uint64(s.character.GuildID),
		PlayerGUID: s.character.GUID,
		Amount:     uint64(amount),
	})
	if err != nil {
		// Refund the debit.
		_, _ = s.gameServerGRPCClient.ModifyMoneyForPlayer(ctx, &pbGameServ.ModifyMoneyForPlayerRequest{
			Api:        root.Ver,
			PlayerGuid: s.character.GUID,
			Value:      int32(amount),
		})
		switch status.Code(err) {
		case codes.PermissionDenied:
			s.sendGuildCommandResult(guildCommandMoveItem, "", guildErrPermissions)
		default:
			// The bank money cap is the only other business failure here.
			s.sendGuildCommandResult(guildCommandMoveItem, "", guildErrBankFull)
		}
		return nil
	}

	return s.sendGuildBankMoneyInfo(ctx)
}

func (s *GameSession) HandleGuildBankWithdrawMoney(ctx context.Context, p *packet.Packet) error {
	r := p.Reader()
	banker := r.Uint64()
	amount := r.Uint32()

	if amount == 0 || !s.canUseGuildBank(ctx, banker) || s.character.GuildID == 0 {
		return nil
	}

	// The service enforces rights, the daily limit and the bank balance.
	_, err := s.guildServiceClient.BankWithdrawMoney(ctx, &pbGuild.BankMoneyParams{
		Api:        root.Ver,
		RealmID:    root.RealmID,
		GuildID:    uint64(s.character.GuildID),
		PlayerGUID: s.character.GUID,
		Amount:     uint64(amount),
	})
	if err != nil {
		switch status.Code(err) {
		case codes.PermissionDenied:
			s.sendGuildCommandResult(guildCommandMoveItem, "", guildErrPermissions)
		case codes.FailedPrecondition:
			s.sendGuildCommandResult(guildCommandMoveItem, "", guildErrNotEnoughMoney)
		}
		return nil
	}

	if _, err = s.gameServerGRPCClient.ModifyMoneyForPlayer(ctx, &pbGameServ.ModifyMoneyForPlayerRequest{
		Api:        root.Ver,
		PlayerGuid: s.character.GUID,
		Value:      int32(amount),
	}); err != nil {
		// Player money cap or vanished player: put the money back in the bank.
		_, _ = s.guildServiceClient.BankDepositMoney(ctx, &pbGuild.BankMoneyParams{
			Api:        root.Ver,
			RealmID:    root.RealmID,
			GuildID:    uint64(s.character.GuildID),
			PlayerGUID: s.character.GUID,
			Amount:     uint64(amount),
		})
		return nil
	}

	return s.sendGuildBankMoneyInfo(ctx)
}

// sendGuildBankMoneyInfo answers MSG_GUILD_BANK_MONEY_WITHDRAWN: how much
// gold the member can still withdraw today (-1 = unlimited).
func (s *GameSession) sendGuildBankMoneyInfo(ctx context.Context) error {
	if s.character.GuildID == 0 {
		return nil
	}

	state, err := s.guildBankState(ctx)
	if err != nil {
		return err
	}

	resp := packet.NewWriterWithSize(packet.MsgGuildBankMoneyWithdrawn, 4)
	resp.Int32(int32(state.RemainingMoney))
	s.gameSocket.Send(resp)
	return nil
}

func (s *GameSession) HandleGuildBankSwapItems(ctx context.Context, p *packet.Packet) error {
	r := p.Reader()
	banker := r.Uint64()
	bankOnly := r.Uint8() != 0

	if s.character.GuildID == 0 {
		return nil
	}

	if bankOnly {
		dstTab := r.Uint8()
		dstSlot := r.Uint8()
		_ = r.Uint32() // dst item id
		srcTab := r.Uint8()
		srcSlot := r.Uint8()
		_ = r.Uint32() // src item id
		_ = r.Uint8()  // auto store
		splitCount := r.Uint32()

		if !s.canUseGuildBank(ctx, banker) {
			return nil
		}

		_, err := s.guildServiceClient.BankMoveItem(ctx, &pbGuild.BankMoveItemParams{
			Api:        root.Ver,
			RealmID:    root.RealmID,
			GuildID:    uint64(s.character.GuildID),
			PlayerGUID: s.character.GUID,
			SrcTab:     uint32(srcTab),
			SrcSlot:    uint32(srcSlot),
			DstTab:     uint32(dstTab),
			DstSlot:    uint32(dstSlot),
			SplitCount: splitCount,
		})
		if err != nil {
			s.handleGuildBankItemError(err)
			return nil
		}

		if srcTab == dstTab {
			return s.sendGuildBankSlots(ctx, srcTab, srcSlot, dstSlot)
		}
		if err = s.sendGuildBankSlots(ctx, srcTab, srcSlot); err != nil {
			return err
		}
		return s.sendGuildBankSlots(ctx, dstTab, dstSlot)
	}

	tab := r.Uint8()
	slot := r.Uint8()
	_ = r.Uint32() // item id
	autoStore := r.Uint8() != 0

	var toChar bool
	var bag, bagSlot uint8
	var stackCount uint32
	if autoStore {
		_ = r.Uint32() // auto store count
		_ = r.Uint8()  // to slot
		_ = r.Uint32() // stack count
		toChar = true
	} else {
		bag = r.Uint8()
		bagSlot = r.Uint8()
		toChar = r.Uint8() != 0
		stackCount = r.Uint32()
	}

	if !s.canUseGuildBank(ctx, banker) {
		return nil
	}

	if toChar {
		return s.withdrawGuildBankItem(ctx, tab, slot, stackCount)
	}
	return s.depositGuildBankItem(ctx, tab, slot, bag, bagSlot, stackCount)
}

// handleGuildBankItemError surfaces item operation failures to the client.
// Every path must answer something: a swallowed error leaves the client
// with a dead drag and no trace anywhere.
func (s *GameSession) handleGuildBankItemError(err error) {
	switch status.Code(err) {
	case codes.PermissionDenied:
		s.sendGuildCommandResult(guildCommandMoveItem, "", guildErrPermissions)
	case codes.NotFound:
		s.sendGuildCommandResult(guildCommandMoveItem, "", guildErrItemNotFound)
	case codes.FailedPrecondition:
		if msg := status.Convert(err).Message(); msg == "stack split is not supported yet" {
			s.SendSysMessage("Splitting stacks in the guild bank is not supported yet.")
			return
		}
		s.sendGuildCommandResult(guildCommandMoveItem, "", guildErrBankFull)
	default:
		s.logger.Warn().Err(err).Msg("guild bank item operation failed")
		s.sendGuildCommandResult(guildCommandMoveItem, "", guildErrInternal)
	}
}

func (s *GameSession) depositGuildBankItem(ctx context.Context, tab, slot, bag, bagSlot uint8, stackCount uint32) error {
	itemResp, err := s.gameServerGRPCClient.GetPlayerItemByPos(ctx, &pbGameServ.GetPlayerItemByPosRequest{
		Api:        root.Ver,
		PlayerGuid: s.character.GUID,
		Bag:        uint32(bag),
		Slot:       uint32(bagSlot),
	})
	if err != nil || itemResp.Item == nil {
		s.sendGuildCommandResult(guildCommandMoveItem, "", guildErrItemNotFound)
		return nil
	}
	item := itemResp.Item

	if !item.IsTradable {
		// Soulbound and otherwise untradable items never enter the bank.
		s.sendGuildCommandResult(guildCommandMoveItem, "", guildErrItemNotFound)
		return nil
	}

	if stackCount > 0 && stackCount < item.Count {
		s.SendSysMessage("Splitting stacks into the guild bank is not supported yet.")
		return nil
	}

	// Refuse the swap gesture instead of silently mixing two flows.
	if slot != guildBankSlotAuto {
		tabResp, err := s.guildServiceClient.GetBankTab(ctx, &pbGuild.GetBankTabParams{
			Api:        root.Ver,
			RealmID:    root.RealmID,
			GuildID:    uint64(s.character.GuildID),
			PlayerGUID: s.character.GUID,
			Tab:        uint32(tab),
		})
		if err != nil {
			s.handleGuildBankItemError(err)
			return nil
		}
		for _, existing := range tabResp.Items {
			if existing.Slot == uint32(slot) {
				s.SendSysMessage("Swapping with a guild bank item is not supported yet - use an empty slot.")
				return nil
			}
		}
	}

	// Detach the item from the player first; the bank row references it.
	removeResp, err := s.gameServerGRPCClient.RemoveItemsWithGuidsFromPlayer(ctx, &pbGameServ.RemoveItemsWithGuidsFromPlayerRequest{
		Api:        root.Ver,
		PlayerGuid: s.character.GUID,
		Guids:      []uint64{item.Guid},
	})
	if err != nil || len(removeResp.UpdatedItemsGuids) == 0 {
		s.sendGuildCommandResult(guildCommandMoveItem, "", guildErrItemNotFound)
		return nil
	}

	depositResp, err := s.guildServiceClient.BankDepositItem(ctx, &pbGuild.BankDepositItemParams{
		Api:        root.Ver,
		RealmID:    root.RealmID,
		GuildID:    uint64(s.character.GuildID),
		PlayerGUID: s.character.GUID,
		Tab:        uint32(tab),
		Slot:       uint32(slot),
		Item: &pbGuild.GuildBankItem{
			ItemGuid:         item.Guid,
			Entry:            item.Entry,
			Count:            item.Count,
			Flags:            item.Flags,
			Durability:       item.Durability,
			RandomPropertyID: int32(item.RandomPropertyID),
			Text:             item.Text,
		},
	})
	if err != nil {
		// Give the item back to the player.
		_, _ = s.gameServerGRPCClient.AddExistingItemToPlayer(ctx, &pbGameServ.AddExistingItemToPlayerRequest{
			Api:        root.Ver,
			PlayerGuid: s.character.GUID,
			Item: &pbGameServ.AddExistingItemToPlayerRequest_Item{
				Guid:             item.Guid,
				Entry:            item.Entry,
				Count:            item.Count,
				Flags:            item.Flags,
				Durability:       item.Durability,
				RandomPropertyID: item.RandomPropertyID,
				Text:             item.Text,
			},
		})
		s.handleGuildBankItemError(err)
		return nil
	}

	return s.sendGuildBankSlots(ctx, tab, uint8(depositResp.Slot))
}

func (s *GameSession) withdrawGuildBankItem(ctx context.Context, tab, slot uint8, stackCount uint32) error {
	if stackCount > 0 {
		// The client only fills the split amount on an explicit stack split.
		tabResp, err := s.guildServiceClient.GetBankTab(ctx, &pbGuild.GetBankTabParams{
			Api:        root.Ver,
			RealmID:    root.RealmID,
			GuildID:    uint64(s.character.GuildID),
			PlayerGUID: s.character.GUID,
			Tab:        uint32(tab),
		})
		if err != nil {
			s.handleGuildBankItemError(err)
			return nil
		}
		for _, existing := range tabResp.Items {
			if existing.Slot == uint32(slot) && stackCount < existing.Count {
				s.SendSysMessage("Splitting stacks out of the guild bank is not supported yet.")
				return nil
			}
		}
	}

	withdrawResp, err := s.guildServiceClient.BankWithdrawItem(ctx, &pbGuild.BankWithdrawItemParams{
		Api:        root.Ver,
		RealmID:    root.RealmID,
		GuildID:    uint64(s.character.GuildID),
		PlayerGUID: s.character.GUID,
		Tab:        uint32(tab),
		Slot:       uint32(slot),
	})
	if err != nil {
		switch status.Code(err) {
		case codes.PermissionDenied:
			s.sendGuildCommandResult(guildCommandMoveItem, "", guildErrPermissions)
		case codes.FailedPrecondition:
			s.sendGuildCommandResult(guildCommandMoveItem, "", guildErrWithdrawLimit)
		case codes.NotFound:
			s.sendGuildCommandResult(guildCommandMoveItem, "", guildErrItemNotFound)
		default:
			s.logger.Warn().Err(err).Msg("guild bank withdraw failed")
			s.sendGuildCommandResult(guildCommandMoveItem, "", guildErrInternal)
		}
		return nil
	}
	item := withdrawResp.Item

	addResp, err := s.gameServerGRPCClient.AddExistingItemToPlayer(ctx, &pbGameServ.AddExistingItemToPlayerRequest{
		Api:        root.Ver,
		PlayerGuid: s.character.GUID,
		Item: &pbGameServ.AddExistingItemToPlayerRequest_Item{
			Guid:             item.ItemGuid,
			Entry:            item.Entry,
			Count:            item.Count,
			Flags:            item.Flags,
			Durability:       item.Durability,
			RandomPropertyID: uint32(item.RandomPropertyID),
			Text:             item.Text,
		},
	})
	if err != nil || addResp.Status != pbGameServ.AddExistingItemToPlayerResponse_Success {
		// Put the item back in the bank (restore path skips the rights check).
		_, _ = s.guildServiceClient.BankDepositItem(ctx, &pbGuild.BankDepositItemParams{
			Api:        root.Ver,
			RealmID:    root.RealmID,
			GuildID:    uint64(s.character.GuildID),
			PlayerGUID: s.character.GUID,
			Tab:        uint32(tab),
			Slot:       uint32(slot),
			Item:       item,
			Restore:    true,
		})
		s.sendInventoryFullError()
		return nil
	}

	// The slot is now empty: a partial update clears it client-side (a full
	// update would omit it and leave the withdrawn item on screen).
	return s.sendGuildBankSlots(ctx, tab, slot)
}

// sendInventoryFullError shows the standard "Inventory is full." client error.
func (s *GameSession) sendInventoryFullError() {
	resp := packet.NewWriterWithSize(packet.SMsgInventoryChangeFailure, 18)
	resp.Uint8(equipErrInventoryFull)
	resp.Uint64(0)
	resp.Uint64(0)
	resp.Uint8(0)
	s.gameSocket.Send(resp)
}

func (s *GameSession) HandleGuildBankBuyTab(ctx context.Context, p *packet.Packet) error {
	r := p.Reader()
	banker := r.Uint64()
	tab := r.Uint8()

	if !s.canUseGuildBank(ctx, banker) || s.character.GuildID == 0 {
		return nil
	}

	state, err := s.guildBankState(ctx)
	if err != nil {
		return err
	}
	if state.NextTabCost == 0 || int(tab) != len(state.Tabs) {
		return nil
	}

	// Debit first; the worldserver refuses when the player lacks the money.
	if _, err = s.gameServerGRPCClient.ModifyMoneyForPlayer(ctx, &pbGameServ.ModifyMoneyForPlayerRequest{
		Api:        root.Ver,
		PlayerGuid: s.character.GUID,
		Value:      -int32(state.NextTabCost),
	}); err != nil {
		return nil
	}

	_, err = s.guildServiceClient.BankBuyTab(ctx, &pbGuild.BankBuyTabParams{
		Api:        root.Ver,
		RealmID:    root.RealmID,
		GuildID:    uint64(s.character.GuildID),
		PlayerGUID: s.character.GUID,
		Tab:        uint32(tab),
		PaidCost:   state.NextTabCost,
	})
	if err != nil {
		_, _ = s.gameServerGRPCClient.ModifyMoneyForPlayer(ctx, &pbGameServ.ModifyMoneyForPlayerRequest{
			Api:        root.Ver,
			PlayerGuid: s.character.GUID,
			Value:      int32(state.NextTabCost),
		})
		return nil
	}

	// Same trick as the core: push permissions so the client updates the tabs.
	if err = s.sendGuildPermissions(ctx); err != nil {
		return err
	}
	return s.sendGuildBankList(ctx, 0, true)
}

func (s *GameSession) HandleGuildBankUpdateTab(ctx context.Context, p *packet.Packet) error {
	r := p.Reader()
	banker := r.Uint64()
	tab := r.Uint8()
	name := r.String()
	icon := r.String()

	if name == "" || icon == "" {
		return nil
	}
	if !s.canUseGuildBank(ctx, banker) || s.character.GuildID == 0 {
		return nil
	}

	_, err := s.guildServiceClient.BankSetTabInfo(ctx, &pbGuild.BankSetTabInfoParams{
		Api:        root.Ver,
		RealmID:    root.RealmID,
		GuildID:    uint64(s.character.GuildID),
		PlayerGUID: s.character.GUID,
		Tab:        uint32(tab),
		Name:       name,
		Icon:       icon,
	})
	if err != nil {
		if status.Code(err) == codes.PermissionDenied {
			s.sendGuildCommandResult(guildCommandViewTab, "", guildErrPermissions)
		}
		return nil
	}

	return s.sendGuildBankList(ctx, 0, true)
}

func (s *GameSession) HandleGuildBankLogQuery(ctx context.Context, p *packet.Packet) error {
	tab := p.Reader().Uint8()

	if s.character.GuildID == 0 {
		return nil
	}

	logResp, err := s.guildServiceClient.GetBankLog(ctx, &pbGuild.GetBankLogParams{
		Api:        root.Ver,
		RealmID:    root.RealmID,
		GuildID:    uint64(s.character.GuildID),
		PlayerGUID: s.character.GUID,
		Tab:        uint32(tab),
	})
	if err != nil {
		return nil
	}

	now := time.Now().Unix()

	resp := packet.NewWriterWithSize(packet.MsgGuildBankLogQuery, 0)
	resp.Uint8(tab)
	resp.Uint8(uint8(len(logResp.Entries)))
	for _, e := range logResp.Entries {
		resp.Uint8(uint8(e.EventType))
		resp.Uint64(e.PlayerGUID)
		switch e.EventType {
		case guildBankLogDepositItem, guildBankLogWithdrawItem:
			resp.Uint32(e.ItemEntry)
			resp.Uint32(e.Count)
		case guildBankLogMoveItem, guildBankLogMoveItem2:
			resp.Uint32(e.ItemEntry)
			resp.Uint32(e.Count)
			resp.Uint8(uint8(e.DestTab))
		default:
			resp.Uint32(uint32(e.Money))
		}
		resp.Uint32(uint32(now - e.Timestamp))
	}
	s.gameSocket.Send(resp)
	return nil
}

func (s *GameSession) HandleQueryGuildBankText(ctx context.Context, p *packet.Packet) error {
	tab := p.Reader().Uint8()

	if s.character.GuildID == 0 {
		return nil
	}

	state, err := s.guildBankState(ctx)
	if err != nil {
		return err
	}
	if int(tab) >= len(state.Tabs) {
		return nil
	}

	resp := packet.NewWriterWithSize(packet.MsgQueryGuildBankText, 0)
	resp.Uint8(tab)
	resp.String(state.Tabs[tab].Text)
	s.gameSocket.Send(resp)
	return nil
}

func (s *GameSession) HandleSetGuildBankText(ctx context.Context, p *packet.Packet) error {
	r := p.Reader()
	tab := r.Uint8()
	text := r.String()

	if s.character.GuildID == 0 {
		return nil
	}

	_, err := s.guildServiceClient.BankSetTabText(ctx, &pbGuild.BankSetTabTextParams{
		Api:        root.Ver,
		RealmID:    root.RealmID,
		GuildID:    uint64(s.character.GuildID),
		PlayerGUID: s.character.GUID,
		Tab:        uint32(tab),
		Text:       text,
	})
	if err != nil && status.Code(err) == codes.PermissionDenied {
		s.sendGuildCommandResult(guildCommandViewTab, "", guildErrPermissions)
	}
	return nil
}

// Bank events pushed by the guild service.

func (s *GameSession) HandleEventGuildBankMoneyUpdated(_ context.Context, e *eBroadcaster.Event) error {
	eventData := e.Payload.(*events.GuildEventBankMoneyUpdatedPayload)

	// GE_BANK_MONEY_SET with the money as big-endian hex (AC wire format).
	s.gameSocket.Send(buildGuildEventPacket(
		GuildEventTypeBankMoneySet, 0,
		fmt.Sprintf("%016X", eventData.Money),
	))

	return nil
}

func (s *GameSession) HandleEventGuildBankTabUpdated(ctx context.Context, e *eBroadcaster.Event) error {
	eventData := e.Payload.(*events.GuildEventBankTabUpdatedPayload)

	if !s.guildBankOpen {
		return nil
	}
	return s.sendGuildBankList(ctx, eventData.TabID, true)
}

func (s *GameSession) HandleEventGuildBankTabsChanged(ctx context.Context, e *eBroadcaster.Event) error {
	s.gameSocket.Send(buildGuildEventPacket(GuildEventTypeTabPurchased, 0))

	if !s.guildBankOpen {
		return nil
	}
	return s.sendGuildBankList(ctx, 0, true)
}

func (s *GameSession) HandleEventGuildBankTextUpdated(_ context.Context, e *eBroadcaster.Event) error {
	eventData := e.Payload.(*events.GuildEventBankTextUpdatedPayload)

	s.gameSocket.Send(buildGuildEventPacket(
		GuildEventTypeBankTextChanged, 0,
		strconv.Itoa(int(eventData.TabID)),
	))

	return nil
}
