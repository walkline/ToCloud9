package session

import (
	"context"
	"fmt"

	root "github.com/walkline/ToCloud9/apps/gateway"
	"github.com/walkline/ToCloud9/apps/gateway/packet"
	pbGuild "github.com/walkline/ToCloud9/gen/guilds/pb"
	pbWorld "github.com/walkline/ToCloud9/gen/worldserver/pb"
	"github.com/walkline/ToCloud9/shared/wow/guid"
)

const (
	guildBankMaxSlots = 98

	guildCommandViewTab  int32 = 21
	guildCommandMoveItem int32 = 22

	guildCommandResultInternal         int32 = 1
	guildCommandResultPermissions      int32 = 8
	guildCommandResultPlayerNotInGuild int32 = 9
	guildCommandResultWithdrawLimit    int32 = 25
	guildCommandResultNotEnoughMoney   int32 = 26
	guildCommandResultBankFull         int32 = 28
	guildCommandResultItemNotFound     int32 = 29

	guildBankTabCost0 uint32 = 1000000
	guildBankTabCost1 uint32 = 2500000
	guildBankTabCost2 uint32 = 5000000
	guildBankTabCost3 uint32 = 10000000
	guildBankTabCost4 uint32 = 25000000
	guildBankTabCost5 uint32 = 50000000
)

var guildBankTabCosts = [guildBankMaxTabs]uint32{
	guildBankTabCost0,
	guildBankTabCost1,
	guildBankTabCost2,
	guildBankTabCost3,
	guildBankTabCost4,
	guildBankTabCost5,
}

func (s *GameSession) HandleGuildBankerActivate(ctx context.Context, p *packet.Packet) error {
	reader := p.Reader()
	banker := reader.Uint64()
	fullUpdate := reader.Uint8() != 0
	if reader.Error() != nil {
		return reader.Error()
	}
	if ok, err := s.canInteractWithGuildBank(ctx, banker); err != nil || !ok {
		return err
	}
	return s.sendGuildBankList(ctx, 0, fullUpdate, nil)
}

func (s *GameSession) HandleGuildBankQueryTab(ctx context.Context, p *packet.Packet) error {
	reader := p.Reader()
	banker := reader.Uint64()
	tabID := reader.Uint8()
	fullUpdate := reader.Uint8() != 0
	if reader.Error() != nil {
		return reader.Error()
	}
	if ok, err := s.canInteractWithGuildBank(ctx, banker); err != nil || !ok {
		return err
	}
	return s.sendGuildBankList(ctx, tabID, fullUpdate, nil)
}

func (s *GameSession) HandleGuildBankLogQuery(ctx context.Context, p *packet.Packet) error {
	reader := p.Reader()
	tabID := reader.Uint8()
	if reader.Error() != nil {
		return reader.Error()
	}
	if s.character == nil || s.character.GuildID == 0 {
		return nil
	}

	resp, err := s.guildServiceClient.GetGuildBankLog(ctx, &pbGuild.GetGuildBankLogParams{
		Api:        root.Ver,
		RealmID:    root.RealmID,
		GuildID:    uint64(s.character.GuildID),
		MemberGUID: s.character.GUID,
		TabID:      uint32(tabID),
	})
	if err != nil {
		return err
	}
	if !s.guildBankStatusOK(resp.Status, guildCommandViewTab) {
		return nil
	}

	w := packet.NewWriterWithSize(packet.MsgGuildBankLogQuery, 0)
	w.Uint8(tabID)
	w.Uint8(uint8(len(resp.Entries)))
	for _, entry := range resp.Entries {
		w.Uint8(uint8(int8(entry.EntryType)))
		w.Uint64(guid.NewFromCounter(guid.Player, guid.LowType(entry.PlayerGUID)).GetRawValue())
		switch entry.EntryType {
		case int32(repoGuildBankLogDepositItem), int32(repoGuildBankLogWithdrawItem):
			w.Uint32(uint32(entry.ItemID))
			w.Uint32(uint32(entry.Count))
		case int32(repoGuildBankLogMoveItem), int32(repoGuildBankLogMoveItem2):
			w.Uint32(uint32(entry.ItemID))
			w.Uint32(uint32(entry.Count))
			w.Uint8(uint8(entry.OtherTab))
		default:
			w.Uint32(entry.Money)
		}
		w.Uint32(entry.TimeOffset)
	}
	s.gameSocket.Send(w)
	return nil
}

func (s *GameSession) HandleQueryGuildBankText(ctx context.Context, p *packet.Packet) error {
	reader := p.Reader()
	tabID := reader.Uint8()
	if reader.Error() != nil {
		return reader.Error()
	}
	if s.character == nil || s.character.GuildID == 0 {
		return nil
	}

	resp, err := s.guildServiceClient.GetGuildBankTabText(ctx, &pbGuild.GetGuildBankTabTextParams{
		Api:        root.Ver,
		RealmID:    root.RealmID,
		GuildID:    uint64(s.character.GuildID),
		MemberGUID: s.character.GUID,
		TabID:      uint32(tabID),
	})
	if err != nil {
		return err
	}
	if !s.guildBankStatusOK(resp.Status, guildCommandViewTab) {
		return nil
	}

	w := packet.NewWriterWithSize(packet.MsgQueryGuildBankText, uint32(1+len(resp.Text)+1))
	w.Uint8(tabID)
	w.String(resp.Text)
	s.gameSocket.Send(w)
	return nil
}

func (s *GameSession) HandleSetGuildBankText(ctx context.Context, p *packet.Packet) error {
	reader := p.Reader()
	tabID := reader.Uint8()
	text := reader.String()
	if reader.Error() != nil {
		return reader.Error()
	}
	if s.character == nil || s.character.GuildID == 0 {
		return nil
	}

	resp, err := s.guildServiceClient.SetGuildBankTabText(ctx, &pbGuild.SetGuildBankTabTextParams{
		Api:        root.Ver,
		RealmID:    root.RealmID,
		GuildID:    uint64(s.character.GuildID),
		MemberGUID: s.character.GUID,
		TabID:      uint32(tabID),
		Text:       text,
	})
	if err != nil {
		return err
	}
	s.guildBankStatusOK(resp.Status, guildCommandViewTab)
	return nil
}

func (s *GameSession) HandleGuildBankUpdateTab(ctx context.Context, p *packet.Packet) error {
	reader := p.Reader()
	banker := reader.Uint64()
	tabID := reader.Uint8()
	name := reader.String()
	icon := reader.String()
	if reader.Error() != nil {
		return reader.Error()
	}
	if ok, err := s.canInteractWithGuildBank(ctx, banker); err != nil || !ok {
		return err
	}

	resp, err := s.guildServiceClient.UpdateGuildBankTab(ctx, &pbGuild.UpdateGuildBankTabParams{
		Api:        root.Ver,
		RealmID:    root.RealmID,
		GuildID:    uint64(s.character.GuildID),
		MemberGUID: s.character.GUID,
		TabID:      uint32(tabID),
		Name:       name,
		Icon:       icon,
	})
	if err != nil {
		return err
	}
	if s.guildBankStatusOK(resp.Status, guildCommandViewTab) {
		return s.sendGuildBankList(ctx, 0, true, nil)
	}
	return nil
}

func (s *GameSession) HandleGuildBankBuyTab(ctx context.Context, p *packet.Packet) error {
	reader := p.Reader()
	banker := reader.Uint64()
	tabID := reader.Uint8()
	if reader.Error() != nil {
		return reader.Error()
	}
	if ok, err := s.canInteractWithGuildBank(ctx, banker); err != nil || !ok {
		return err
	}
	if tabID >= guildBankMaxTabs {
		s.sendGuildCommandResult(guildCommandViewTab, "", guildCommandResultInternal)
		return nil
	}

	cost := guildBankTabCosts[tabID]
	money, err := s.gameServerGRPCClient.GetMoneyForPlayer(ctx, &pbWorld.GetMoneyForPlayerRequest{Api: root.Ver, PlayerGuid: s.character.GUID})
	if err != nil {
		return err
	}
	if money.Money < cost {
		s.sendGuildCommandResult(guildCommandViewTab, "", guildCommandResultNotEnoughMoney)
		return nil
	}
	if _, err = s.gameServerGRPCClient.ModifyMoneyForPlayer(ctx, &pbWorld.ModifyMoneyForPlayerRequest{Api: root.Ver, PlayerGuid: s.character.GUID, Value: -int32(cost)}); err != nil {
		return err
	}

	resp, err := s.guildServiceClient.BuyGuildBankTab(ctx, &pbGuild.BuyGuildBankTabParams{
		Api:        root.Ver,
		RealmID:    root.RealmID,
		GuildID:    uint64(s.character.GuildID),
		MemberGUID: s.character.GUID,
		TabID:      uint32(tabID),
	})
	if err != nil {
		return err
	}
	if resp.Status != pbGuild.GuildBankStatus_Ok {
		_, _ = s.gameServerGRPCClient.ModifyMoneyForPlayer(ctx, &pbWorld.ModifyMoneyForPlayerRequest{Api: root.Ver, PlayerGuid: s.character.GUID, Value: int32(cost)})
		s.guildBankStatusOK(resp.Status, guildCommandViewTab)
		return nil
	}
	return s.sendGuildBankList(ctx, 0, true, nil)
}

func (s *GameSession) HandleGuildBankDepositMoney(ctx context.Context, p *packet.Packet) error {
	reader := p.Reader()
	banker := reader.Uint64()
	amount := reader.Uint32()
	if reader.Error() != nil {
		return reader.Error()
	}
	if ok, err := s.canInteractWithGuildBank(ctx, banker); err != nil || !ok {
		return err
	}

	money, err := s.gameServerGRPCClient.GetMoneyForPlayer(ctx, &pbWorld.GetMoneyForPlayerRequest{Api: root.Ver, PlayerGuid: s.character.GUID})
	if err != nil {
		return err
	}
	if money.Money < amount {
		s.sendGuildCommandResult(guildCommandMoveItem, "", guildCommandResultNotEnoughMoney)
		return nil
	}
	if _, err = s.gameServerGRPCClient.ModifyMoneyForPlayer(ctx, &pbWorld.ModifyMoneyForPlayerRequest{Api: root.Ver, PlayerGuid: s.character.GUID, Value: -int32(amount)}); err != nil {
		return err
	}

	resp, err := s.guildServiceClient.DepositGuildBankMoney(ctx, &pbGuild.DepositGuildBankMoneyParams{
		Api:        root.Ver,
		RealmID:    root.RealmID,
		GuildID:    uint64(s.character.GuildID),
		MemberGUID: s.character.GUID,
		Amount:     amount,
	})
	if err != nil {
		return err
	}
	if resp.Status != pbGuild.GuildBankStatus_Ok {
		_, _ = s.gameServerGRPCClient.ModifyMoneyForPlayer(ctx, &pbWorld.ModifyMoneyForPlayerRequest{Api: root.Ver, PlayerGuid: s.character.GUID, Value: int32(amount)})
		s.guildBankStatusOK(resp.Status, guildCommandMoveItem)
		return nil
	}
	return s.sendGuildBankList(ctx, 0, false, nil)
}

func (s *GameSession) HandleGuildBankWithdrawMoney(ctx context.Context, p *packet.Packet) error {
	reader := p.Reader()
	banker := reader.Uint64()
	amount := reader.Uint32()
	if reader.Error() != nil {
		return reader.Error()
	}
	if ok, err := s.canInteractWithGuildBank(ctx, banker); err != nil || !ok {
		return err
	}

	resp, err := s.guildServiceClient.WithdrawGuildBankMoney(ctx, &pbGuild.WithdrawGuildBankMoneyParams{
		Api:        root.Ver,
		RealmID:    root.RealmID,
		GuildID:    uint64(s.character.GuildID),
		MemberGUID: s.character.GUID,
		Amount:     amount,
	})
	if err != nil {
		return err
	}
	if !s.guildBankStatusOK(resp.Status, guildCommandMoveItem) {
		return nil
	}
	if _, err = s.gameServerGRPCClient.ModifyMoneyForPlayer(ctx, &pbWorld.ModifyMoneyForPlayerRequest{Api: root.Ver, PlayerGuid: s.character.GUID, Value: int32(amount)}); err != nil {
		if _, rollbackErr := s.guildServiceClient.RollbackGuildBankMoneyWithdraw(ctx, &pbGuild.RollbackGuildBankMoneyWithdrawParams{Api: root.Ver, RealmID: root.RealmID, GuildID: uint64(s.character.GuildID), MemberGUID: s.character.GUID, Amount: amount, LogGUID: resp.LogGUID}); rollbackErr != nil {
			return rollbackErr
		}
		return err
	}
	return s.sendGuildBankList(ctx, 0, false, nil)
}

func (s *GameSession) HandleGuildBankSwapItems(ctx context.Context, p *packet.Packet) error {
	reader := p.Reader()
	banker := reader.Uint64()
	bankOnly := reader.Uint8() != 0
	if reader.Error() != nil {
		return reader.Error()
	}
	if ok, err := s.canInteractWithGuildBank(ctx, banker); err != nil || !ok {
		return err
	}

	if bankOnly {
		destTab := reader.Uint8()
		destSlot := reader.Uint8()
		reader.Uint32()
		srcTab := reader.Uint8()
		srcSlot := reader.Uint8()
		reader.Uint32()
		reader.Uint8()
		count := reader.Uint32()
		if reader.Error() != nil {
			return reader.Error()
		}
		resp, err := s.guildServiceClient.MoveGuildBankItem(ctx, &pbGuild.MoveGuildBankItemParams{
			Api:               root.Ver,
			RealmID:           root.RealmID,
			GuildID:           uint64(s.character.GuildID),
			MemberGUID:        s.character.GUID,
			SourceTabID:       uint32(srcTab),
			SourceSlotID:      uint32(srcSlot),
			DestinationTabID:  uint32(destTab),
			DestinationSlotID: uint32(destSlot),
			Count:             count,
		})
		if err != nil {
			return err
		}
		if !s.guildBankStatusOK(resp.Status, guildCommandMoveItem) {
			return nil
		}
		if err = s.sendGuildBankList(ctx, srcTab, false, nil); err != nil {
			return err
		}
		if destTab != srcTab {
			return s.sendGuildBankList(ctx, destTab, false, nil)
		}
		return nil
	}

	bankTab := reader.Uint8()
	bankSlot := reader.Uint8()
	reader.Uint32()
	autoStore := reader.Uint8() != 0
	var playerBag uint8 = 255
	var playerSlot uint8 = 255
	toChar := true
	var stackCount uint32
	if autoStore {
		reader.Uint32()
		reader.Uint8()
		stackCount = reader.Uint32()
	} else {
		playerBag = reader.Uint8()
		playerSlot = reader.Uint8()
		toChar = reader.Uint8() != 0
		stackCount = reader.Uint32()
	}
	if reader.Error() != nil {
		return reader.Error()
	}

	if toChar {
		return s.withdrawGuildBankItem(ctx, bankTab, bankSlot, stackCount, !autoStore, playerBag, playerSlot)
	}
	return s.depositGuildBankItem(ctx, bankTab, bankSlot, stackCount, playerBag, playerSlot)
}

func (s *GameSession) depositGuildBankItem(ctx context.Context, tabID, slotID uint8, count uint32, playerBag, playerSlot uint8) error {
	taken, err := s.gameServerGRPCClient.TakePlayerItemByPos(ctx, &pbWorld.TakePlayerItemByPosRequest{
		Api:                root.Ver,
		PlayerGuid:         s.character.GUID,
		BagSlot:            uint32(playerBag),
		Slot:               uint32(playerSlot),
		Count:              count,
		AssignToPlayerGuid: 0,
	})
	if err != nil {
		return err
	}
	if taken.Status != pbWorld.TakePlayerItemByPosResponse_Success || taken.Item == nil {
		s.sendGuildCommandResult(guildCommandMoveItem, "", guildCommandResultItemNotFound)
		return nil
	}

	resp, err := s.guildServiceClient.DepositGuildBankItem(ctx, &pbGuild.DepositGuildBankItemParams{
		Api:        root.Ver,
		RealmID:    root.RealmID,
		GuildID:    uint64(s.character.GuildID),
		MemberGUID: s.character.GUID,
		TabID:      uint32(tabID),
		SlotID:     uint32(slotID),
		Item:       worldItemToGuildBankItem(taken.Item, slotID),
	})
	if err != nil {
		return err
	}
	if resp.Status != pbGuild.GuildBankStatus_Ok {
		_, _ = s.gameServerGRPCClient.AddExistingItemToPlayer(ctx, &pbWorld.AddExistingItemToPlayerRequest{
			Api:        root.Ver,
			PlayerGuid: s.character.GUID,
			Item:       worldTakenItemToAddItem(taken.Item),
		})
		s.guildBankStatusOK(resp.Status, guildCommandMoveItem)
		return nil
	}
	return s.sendGuildBankList(ctx, tabID, false, resp.ChangedSlots)
}

func (s *GameSession) withdrawGuildBankItem(ctx context.Context, tabID, slotID uint8, count uint32, storeAtPos bool, playerBag, playerSlot uint8) error {
	resp, err := s.guildServiceClient.WithdrawGuildBankItem(ctx, &pbGuild.WithdrawGuildBankItemParams{
		Api:        root.Ver,
		RealmID:    root.RealmID,
		GuildID:    uint64(s.character.GuildID),
		MemberGUID: s.character.GUID,
		TabID:      uint32(tabID),
		SlotID:     uint32(slotID),
		Count:      count,
	})
	if err != nil {
		return err
	}
	if !s.guildBankStatusOK(resp.Status, guildCommandMoveItem) || resp.Item == nil {
		return nil
	}

	addResp, err := s.gameServerGRPCClient.AddExistingItemToPlayer(ctx, &pbWorld.AddExistingItemToPlayerRequest{
		Api:        root.Ver,
		PlayerGuid: s.character.GUID,
		Item: &pbWorld.AddExistingItemToPlayerRequest_Item{
			Guid:             resp.Item.ItemGUID,
			Entry:            resp.Item.Entry,
			Count:            resp.Item.Count,
			Flags:            resp.Item.Flags,
			Durability:       resp.Item.Durability,
			RandomPropertyID: resp.Item.RandomPropertyID,
			Text:             resp.Item.Text,
		},
		StoreAtPos: storeAtPos,
		BagSlot:    uint32(playerBag),
		Slot:       uint32(playerSlot),
	})
	if err != nil || addResp.Status != pbWorld.AddExistingItemToPlayerResponse_Success {
		rollbackResp, rollbackErr := s.guildServiceClient.RollbackGuildBankItemWithdraw(ctx, &pbGuild.RollbackGuildBankItemWithdrawParams{
			Api:        root.Ver,
			RealmID:    root.RealmID,
			GuildID:    uint64(s.character.GuildID),
			MemberGUID: s.character.GUID,
			TabID:      uint32(tabID),
			SlotID:     uint32(slotID),
			Item:       resp.Item,
			LogGUID:    resp.LogGUID,
		})
		if rollbackErr != nil {
			return rollbackErr
		}
		if rollbackResp != nil && rollbackResp.Status == pbGuild.GuildBankStatus_Ok {
			_ = s.sendGuildBankList(ctx, tabID, false, rollbackResp.ChangedSlots)
		}
		if err != nil {
			return err
		}
		s.sendGuildCommandResult(guildCommandMoveItem, "", guildCommandResultBankFull)
		return nil
	}
	return s.sendGuildBankList(ctx, tabID, false, resp.ChangedSlots)
}

func (s *GameSession) sendGuildBankList(ctx context.Context, tabID uint8, fullUpdate bool, changedSlots []uint32) error {
	if s.character == nil || s.character.GuildID == 0 {
		return nil
	}
	resp, err := s.guildServiceClient.GetGuildBank(ctx, &pbGuild.GetGuildBankParams{
		Api:        root.Ver,
		RealmID:    root.RealmID,
		GuildID:    uint64(s.character.GuildID),
		MemberGUID: s.character.GUID,
		TabID:      uint32(tabID),
		FullUpdate: fullUpdate,
	})
	if err != nil {
		return err
	}
	if !s.guildBankStatusOK(resp.Status, guildCommandViewTab) {
		return nil
	}
	s.gameSocket.SendPacket(buildGuildBankListPacket(resp, changedSlots))
	return nil
}

func buildGuildBankListPacket(resp *pbGuild.GetGuildBankResponse, changedSlots []uint32) *packet.Packet {
	w := packet.NewWriterWithSize(packet.SMsgGuildBankList, 0)
	w.Uint64(resp.Money)
	w.Uint8(uint8(resp.TabID))
	w.Int32(resp.WithdrawalsRemaining)
	w.Uint8(boolToUint8(resp.FullUpdate))

	if resp.FullUpdate && resp.TabID == 0 {
		w.Uint8(uint8(len(resp.Tabs)))
		for _, tab := range resp.Tabs {
			w.String(tab.Name)
			w.String(tab.Icon)
		}
	}

	items := resp.Items
	if len(changedSlots) > 0 {
		itemBySlot := map[uint32]*pbGuild.GuildBankItem{}
		for _, item := range resp.Items {
			itemBySlot[item.Slot] = item
		}
		items = make([]*pbGuild.GuildBankItem, 0, len(changedSlots))
		for _, slot := range changedSlots {
			if item := itemBySlot[slot]; item != nil {
				items = append(items, item)
			} else {
				items = append(items, &pbGuild.GuildBankItem{Slot: slot})
			}
		}
	}

	w.Uint8(uint8(len(items)))
	for _, item := range items {
		w.Uint8(uint8(item.Slot))
		w.Uint32(item.Entry)
		if item.Entry == 0 {
			continue
		}
		w.Int32(int32(item.Flags))
		w.Int32(item.RandomPropertyID)
		if item.RandomPropertyID != 0 {
			w.Int32(item.RandomPropertySeed)
		}
		w.Int32(int32(item.Count))
		w.Int32(int32(item.EnchantmentID))
		w.Uint8(uint8(item.Charges))
		w.Uint8(uint8(len(item.SocketEnchants)))
		for _, socket := range item.SocketEnchants {
			w.Uint8(uint8(socket.SocketIndex))
			w.Int32(int32(socket.SocketEnchantID))
		}
	}
	return w.ToPacket()
}

func (s *GameSession) canInteractWithGuildBank(ctx context.Context, object uint64) (bool, error) {
	if s.character == nil || s.character.GuildID == 0 {
		s.sendGuildCommandResult(guildCommandViewTab, "", guildCommandResultPlayerNotInGuild)
		return false, nil
	}
	if guid.New(object).GetHigh() != guid.GameObject {
		return false, fmt.Errorf("player '%d' tried to interact with non-gameobject guild bank '%d'", s.character.GUID, object)
	}
	const gameObjectTypeGuildBank = 34
	resp, err := s.gameServerGRPCClient.CanPlayerInteractWithGameObject(ctx, &pbWorld.CanPlayerInteractWithGameObjectRequest{
		Api:            root.Ver,
		PlayerGuid:     s.character.GUID,
		GameObjectGuid: object,
		GameObjectType: gameObjectTypeGuildBank,
	})
	if err != nil {
		return false, err
	}
	return resp.CanInteract, nil
}

func (s *GameSession) guildBankStatusOK(status pbGuild.GuildBankStatus_Status, command int32) bool {
	if status == pbGuild.GuildBankStatus_Ok {
		return true
	}
	s.sendGuildCommandResult(command, "", guildBankStatusToNativeResult(status))
	return false
}

func guildBankStatusToNativeResult(status pbGuild.GuildBankStatus_Status) int32 {
	switch status {
	case pbGuild.GuildBankStatus_NotInGuild, pbGuild.GuildBankStatus_GuildNotFound:
		return guildCommandResultPlayerNotInGuild
	case pbGuild.GuildBankStatus_NotEnoughRights:
		return guildCommandResultPermissions
	case pbGuild.GuildBankStatus_NotEnoughMoney:
		return guildCommandResultNotEnoughMoney
	case pbGuild.GuildBankStatus_BankFull:
		return guildCommandResultBankFull
	case pbGuild.GuildBankStatus_WithdrawLimit:
		return guildCommandResultWithdrawLimit
	case pbGuild.GuildBankStatus_ItemNotFound:
		return guildCommandResultItemNotFound
	default:
		return guildCommandResultInternal
	}
}

func worldItemToGuildBankItem(item *pbWorld.TakePlayerItemByPosResponse_Item, slotID uint8) *pbGuild.GuildBankItem {
	return &pbGuild.GuildBankItem{
		ItemGUID:         item.Guid,
		Entry:            item.Entry,
		Slot:             uint32(slotID),
		Count:            item.Count,
		Flags:            item.Flags,
		Durability:       item.Durability,
		RandomPropertyID: item.RandomPropertyID,
		Text:             item.Text,
	}
}

func worldTakenItemToAddItem(item *pbWorld.TakePlayerItemByPosResponse_Item) *pbWorld.AddExistingItemToPlayerRequest_Item {
	return &pbWorld.AddExistingItemToPlayerRequest_Item{
		Guid:             item.Guid,
		Entry:            item.Entry,
		Count:            item.Count,
		Flags:            item.Flags,
		Durability:       item.Durability,
		RandomPropertyID: item.RandomPropertyID,
		Text:             item.Text,
	}
}

const (
	repoGuildBankLogDepositItem  = 1
	repoGuildBankLogWithdrawItem = 2
	repoGuildBankLogMoveItem     = 3
	repoGuildBankLogMoveItem2    = 7
)
