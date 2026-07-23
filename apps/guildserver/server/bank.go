package server

import (
	"context"
	"errors"

	"github.com/rs/zerolog/log"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/walkline/ToCloud9/apps/guildserver"
	"github.com/walkline/ToCloud9/apps/guildserver/repo"
	"github.com/walkline/ToCloud9/apps/guildserver/service"
	"github.com/walkline/ToCloud9/gen/guilds/pb"
)

// bankStatusError maps bank business errors to grpc status codes. Unexpected
// errors (DB failures and the like) are logged here: the gateway only relays
// clean business codes to the client, so this is where they surface.
func bankStatusError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, service.ErrNotEnoughRight):
		return status.Error(codes.PermissionDenied, err.Error())
	case errors.Is(err, service.ErrGuildNotFound),
		errors.Is(err, repo.ErrBankTabNotFound),
		errors.Is(err, repo.ErrBankSlotEmpty):
		return status.Error(codes.NotFound, err.Error())
	case errors.Is(err, repo.ErrBankSlotOccupied),
		errors.Is(err, repo.ErrBankTabFull),
		errors.Is(err, repo.ErrBankNotEnoughMoney),
		errors.Is(err, repo.ErrBankMoneyLimit),
		errors.Is(err, repo.ErrBankWithdrawLimit),
		errors.Is(err, service.ErrBankWrongCost),
		errors.Is(err, repo.ErrBankSplitUnsupported):
		return status.Error(codes.FailedPrecondition, err.Error())
	}
	log.Warn().Err(err).Msg("guild bank operation failed with unexpected error")
	return status.Error(codes.Internal, "guild bank operation failed")
}

func bankItemToPB(item repo.BankItem) *pb.GuildBankItem {
	return &pb.GuildBankItem{
		Slot:             uint32(item.Slot),
		ItemGuid:         item.ItemGUID,
		Entry:            item.Entry,
		Count:            item.Count,
		Flags:            item.Flags,
		Durability:       item.Durability,
		RandomPropertyID: item.RandomPropertyID,
		EnchantmentID:    item.EnchantmentID,
		Charges:          item.Charges,
		Text:             item.Text,
		SocketEnchantIDs: item.SocketEnchantIDs,
	}
}

func bankItemFromPB(item *pb.GuildBankItem) repo.BankItem {
	if item == nil {
		return repo.BankItem{}
	}
	return repo.BankItem{
		Slot:             uint8(item.Slot),
		ItemGUID:         item.ItemGuid,
		Entry:            item.Entry,
		Count:            item.Count,
		Flags:            item.Flags,
		Durability:       item.Durability,
		RandomPropertyID: item.RandomPropertyID,
		EnchantmentID:    item.EnchantmentID,
		Charges:          item.Charges,
		Text:             item.Text,
	}
}

func (g *GuildServer) GetBankState(ctx context.Context, params *pb.GetBankStateParams) (*pb.GetBankStateResponse, error) {
	state, err := g.bankService.BankState(ctx, params.RealmID, params.GuildID, params.PlayerGUID)
	if err != nil {
		return nil, bankStatusError(err)
	}

	resp := &pb.GetBankStateResponse{
		Api:            guildserver.Ver,
		Money:          state.Money,
		RankID:         uint32(state.RankID),
		RankRights:     state.RankRights,
		MoneyPerDay:    state.MoneyPerDay,
		RemainingMoney: state.RemainingMoney,
		NextTabCost:    state.NextTabCost,
	}
	for _, tab := range state.Tabs {
		resp.Tabs = append(resp.Tabs, &pb.GetBankStateResponse_Tab{
			Name:           tab.Name,
			Icon:           tab.Icon,
			Text:           tab.Text,
			Rights:         uint32(tab.Rights),
			RemainingSlots: tab.RemainingSlots,
		})
	}
	return resp, nil
}

func (g *GuildServer) GetBankTab(ctx context.Context, params *pb.GetBankTabParams) (*pb.GetBankTabResponse, error) {
	items, err := g.bankService.BankTabItems(ctx, params.RealmID, params.GuildID, params.PlayerGUID, uint8(params.Tab))
	if err != nil {
		return nil, bankStatusError(err)
	}

	resp := &pb.GetBankTabResponse{Api: guildserver.Ver}
	for _, item := range items {
		resp.Items = append(resp.Items, bankItemToPB(item))
	}
	return resp, nil
}

func (g *GuildServer) BankDepositMoney(ctx context.Context, params *pb.BankMoneyParams) (*pb.BankMoneyResponse, error) {
	newMoney, remaining, err := g.bankService.BankDepositMoney(ctx, params.RealmID, params.GuildID, params.PlayerGUID, params.Amount)
	if err != nil {
		return nil, bankStatusError(err)
	}
	return &pb.BankMoneyResponse{Api: guildserver.Ver, NewBankMoney: newMoney, RemainingMoney: remaining}, nil
}

func (g *GuildServer) BankWithdrawMoney(ctx context.Context, params *pb.BankMoneyParams) (*pb.BankMoneyResponse, error) {
	newMoney, remaining, err := g.bankService.BankWithdrawMoney(ctx, params.RealmID, params.GuildID, params.PlayerGUID, params.Amount)
	if err != nil {
		return nil, bankStatusError(err)
	}
	return &pb.BankMoneyResponse{Api: guildserver.Ver, NewBankMoney: newMoney, RemainingMoney: remaining}, nil
}

func (g *GuildServer) BankDepositItem(ctx context.Context, params *pb.BankDepositItemParams) (*pb.BankDepositItemResponse, error) {
	slot, err := g.bankService.BankDepositItem(
		ctx, params.RealmID, params.GuildID, params.PlayerGUID,
		uint8(params.Tab), uint8(params.Slot), bankItemFromPB(params.Item), params.Restore,
	)
	if err != nil {
		return nil, bankStatusError(err)
	}
	return &pb.BankDepositItemResponse{Api: guildserver.Ver, Slot: uint32(slot)}, nil
}

func (g *GuildServer) BankWithdrawItem(ctx context.Context, params *pb.BankWithdrawItemParams) (*pb.BankWithdrawItemResponse, error) {
	item, remaining, err := g.bankService.BankWithdrawItem(ctx, params.RealmID, params.GuildID, params.PlayerGUID, uint8(params.Tab), uint8(params.Slot))
	if err != nil {
		return nil, bankStatusError(err)
	}
	return &pb.BankWithdrawItemResponse{Api: guildserver.Ver, Item: bankItemToPB(item), RemainingSlots: remaining}, nil
}

func (g *GuildServer) BankMoveItem(ctx context.Context, params *pb.BankMoveItemParams) (*pb.BankMoveItemResponse, error) {
	err := g.bankService.BankMoveItem(
		ctx, params.RealmID, params.GuildID, params.PlayerGUID,
		uint8(params.SrcTab), uint8(params.SrcSlot), uint8(params.DstTab), uint8(params.DstSlot), params.SplitCount,
	)
	if err != nil {
		return nil, bankStatusError(err)
	}
	return &pb.BankMoveItemResponse{Api: guildserver.Ver}, nil
}

func (g *GuildServer) BankBuyTab(ctx context.Context, params *pb.BankBuyTabParams) (*pb.BankBuyTabResponse, error) {
	err := g.bankService.BankBuyTab(ctx, params.RealmID, params.GuildID, params.PlayerGUID, uint8(params.Tab), params.PaidCost)
	if err != nil {
		return nil, bankStatusError(err)
	}
	return &pb.BankBuyTabResponse{Api: guildserver.Ver}, nil
}

func (g *GuildServer) BankSetTabInfo(ctx context.Context, params *pb.BankSetTabInfoParams) (*pb.BankSetTabInfoResponse, error) {
	err := g.bankService.BankSetTabInfo(ctx, params.RealmID, params.GuildID, params.PlayerGUID, uint8(params.Tab), params.Name, params.Icon)
	if err != nil {
		return nil, bankStatusError(err)
	}
	return &pb.BankSetTabInfoResponse{Api: guildserver.Ver}, nil
}

func (g *GuildServer) BankSetTabText(ctx context.Context, params *pb.BankSetTabTextParams) (*pb.BankSetTabTextResponse, error) {
	err := g.bankService.BankSetTabText(ctx, params.RealmID, params.GuildID, params.PlayerGUID, uint8(params.Tab), params.Text)
	if err != nil {
		return nil, bankStatusError(err)
	}
	return &pb.BankSetTabTextResponse{Api: guildserver.Ver}, nil
}

func (g *GuildServer) GetBankLog(ctx context.Context, params *pb.GetBankLogParams) (*pb.GetBankLogResponse, error) {
	entries, err := g.bankService.BankLog(ctx, params.RealmID, params.GuildID, params.PlayerGUID, uint8(params.Tab))
	if err != nil {
		return nil, bankStatusError(err)
	}

	resp := &pb.GetBankLogResponse{Api: guildserver.Ver}
	for _, e := range entries {
		resp.Entries = append(resp.Entries, &pb.GetBankLogResponse_Entry{
			EventType:  uint32(e.EventType),
			PlayerGUID: e.PlayerGUID,
			ItemEntry:  e.ItemEntry,
			Count:      e.Count,
			DestTab:    uint32(e.DestTab),
			Money:      e.Money,
			Timestamp:  e.Timestamp,
		})
	}
	return resp, nil
}
