package service

import (
	"context"
	"errors"

	"github.com/walkline/ToCloud9/apps/guildserver"
	"github.com/walkline/ToCloud9/apps/guildserver/repo"
	"github.com/walkline/ToCloud9/shared/events"
)

var (
	// ErrBankWrongCost returned when the gateway paid a different cost than
	// the tab being purchased (concurrent purchase).
	ErrBankWrongCost = errors.New("tab cost mismatch")
)

// GuildBankTabCosts is the price of each bank tab in copper (AC defaults).
var GuildBankTabCosts = [repo.GuildBankMaxTabs]uint32{1000000, 2500000, 5000000, 10000000, 25000000, 50000000}

// BankStateTab is a purchased tab with the rights of the requesting member.
type BankStateTab struct {
	repo.BankTab

	Rights         uint8
	RemainingSlots uint32
}

// BankState is everything the gateway needs to answer the bank related
// queries (MSG_GUILD_PERMISSIONS, MSG_GUILD_BANK_MONEY_WITHDRAWN,
// CMSG_GUILD_BANKER_ACTIVATE) for one member.
type BankState struct {
	Money          uint64
	RankID         uint8
	RankRights     uint32
	MoneyPerDay    uint32
	RemainingMoney uint32
	Tabs           []BankStateTab
	NextTabCost    uint32
}

// GuildBankService handles the guild bank. It is the only writer of the bank
// state; items move between players and the bank through the worldserver
// items API, driven by the gateway.
type GuildBankService interface {
	BankState(ctx context.Context, realmID uint32, guildID, playerGUID uint64) (*BankState, error)
	BankTabItems(ctx context.Context, realmID uint32, guildID, playerGUID uint64, tab uint8) ([]repo.BankItem, error)

	BankDepositMoney(ctx context.Context, realmID uint32, guildID, playerGUID, amount uint64) (newMoney uint64, remaining uint32, err error)
	BankWithdrawMoney(ctx context.Context, realmID uint32, guildID, playerGUID, amount uint64) (newMoney uint64, remaining uint32, err error)

	BankDepositItem(ctx context.Context, realmID uint32, guildID, playerGUID uint64, tab, slot uint8, item repo.BankItem, restore bool) (uint8, error)
	BankWithdrawItem(ctx context.Context, realmID uint32, guildID, playerGUID uint64, tab, slot uint8) (repo.BankItem, uint32, error)
	BankMoveItem(ctx context.Context, realmID uint32, guildID, playerGUID uint64, srcTab, srcSlot, dstTab, dstSlot uint8, splitCount uint32) error

	BankBuyTab(ctx context.Context, realmID uint32, guildID, playerGUID uint64, tab uint8, paidCost uint32) error
	BankSetTabInfo(ctx context.Context, realmID uint32, guildID, playerGUID uint64, tab uint8, name, icon string) error
	BankSetTabText(ctx context.Context, realmID uint32, guildID, playerGUID uint64, tab uint8, text string) error

	BankLog(ctx context.Context, realmID uint32, guildID, playerGUID uint64, tab uint8) ([]repo.BankLogEntry, error)

	// RankBankTabRights returns the per-rank per-tab rights for the roster packet.
	RankBankTabRights(ctx context.Context, realmID uint32, guildID uint64) (map[uint8][repo.GuildBankMaxTabs]repo.BankTabRights, error)
	// SetRankBankTabRights persists the bank part of CMSG_GUILD_RANK for the
	// purchased tabs. The caller (UpdateRank flow) already validated the changer.
	SetRankBankTabRights(ctx context.Context, realmID uint32, guildID uint64, rank uint8, rights []repo.BankTabRights) error
	// SetRankBankTabRightsByChanger resolves the changer's guild and persists
	// the rights there (UpdateRank flow, which has no guild id in its params).
	SetRankBankTabRightsByChanger(ctx context.Context, realmID uint32, changerGUID uint64, rank uint8, rights []repo.BankTabRights) error
}

type guildBankServiceImpl struct {
	bankRepo       repo.GuildBankRepo
	guildsRepo     repo.GuildsRepo
	eventsProducer events.GuildServiceProducer
}

func NewGuildBankService(bankRepo repo.GuildBankRepo, guildsRepo repo.GuildsRepo, eventsProducer events.GuildServiceProducer) GuildBankService {
	return &guildBankServiceImpl{
		bankRepo:       bankRepo,
		guildsRepo:     guildsRepo,
		eventsProducer: eventsProducer,
	}
}

// memberContext resolves the guild and the member with its rank, making sure
// the player belongs to the guild it claims to act on.
func (g *guildBankServiceImpl) memberContext(ctx context.Context, realmID uint32, guildID, playerGUID uint64) (*repo.Guild, *repo.GuildMember, *repo.GuildRank, error) {
	guild, err := g.guildsRepo.GuildByRealmAndID(ctx, realmID, guildID)
	if err != nil {
		return nil, nil, nil, err
	}
	if guild == nil {
		return nil, nil, nil, ErrGuildNotFound
	}

	var member *repo.GuildMember
	for _, m := range guild.GuildMembers {
		if m.PlayerGUID == playerGUID {
			member = m
			break
		}
	}
	if member == nil {
		return nil, nil, nil, ErrGuildNotFound
	}

	for i := range guild.GuildRanks {
		if guild.GuildRanks[i].Rank == member.Rank {
			return guild, member, &guild.GuildRanks[i], nil
		}
	}
	return nil, nil, nil, ErrGuildNotFound
}

func (g *guildBankServiceImpl) isGuildMaster(member *repo.GuildMember) bool {
	return member.Rank == uint8(repo.GuildRankGuildMaster)
}

// tabRights returns the member rights on one tab.
func (g *guildBankServiceImpl) tabRights(member *repo.GuildMember, rightsMap map[uint8][repo.GuildBankMaxTabs]repo.BankTabRights, tab uint8) repo.BankTabRights {
	if g.isGuildMaster(member) {
		return repo.BankTabRights{Rights: repo.BankRightFull, SlotsPerDay: repo.BankWithdrawUnlimited}
	}
	if tab >= repo.GuildBankMaxTabs {
		return repo.BankTabRights{}
	}
	return rightsMap[member.Rank][tab]
}

func (g *guildBankServiceImpl) remainingSlots(tr repo.BankTabRights, withdrawn uint32) uint32 {
	if tr.SlotsPerDay == repo.BankWithdrawUnlimited {
		return repo.BankWithdrawUnlimited
	}
	if tr.Rights&repo.BankRightViewTab == 0 || tr.SlotsPerDay <= withdrawn {
		return 0
	}
	return tr.SlotsPerDay - withdrawn
}

func (g *guildBankServiceImpl) remainingMoney(member *repo.GuildMember, rank *repo.GuildRank, withdrawnMoney uint32) uint32 {
	if g.isGuildMaster(member) {
		return repo.BankWithdrawUnlimited
	}
	// Plain bitmask: HasRight() is only valid for the RightEmpty-embedding
	// flags, which the withdraw rights are not.
	if rank.Rights&(repo.RightWithdrawGold|repo.RightWithdrawRepair) == 0 {
		return 0
	}
	if rank.MoneyPerDay <= withdrawnMoney {
		return 0
	}
	return rank.MoneyPerDay - withdrawnMoney
}

func (g *guildBankServiceImpl) BankState(ctx context.Context, realmID uint32, guildID, playerGUID uint64) (*BankState, error) {
	_, member, rank, err := g.memberContext(ctx, realmID, guildID, playerGUID)
	if err != nil {
		return nil, err
	}

	money, err := g.bankRepo.BankMoney(ctx, realmID, guildID)
	if err != nil {
		return nil, err
	}
	tabs, err := g.bankRepo.BankTabs(ctx, realmID, guildID)
	if err != nil {
		return nil, err
	}
	rightsMap, err := g.bankRepo.RankTabRights(ctx, realmID, guildID)
	if err != nil {
		return nil, err
	}
	withdrawals, err := g.bankRepo.MemberWithdrawals(ctx, realmID, playerGUID)
	if err != nil {
		return nil, err
	}

	state := &BankState{
		Money:          money,
		RankID:         member.Rank,
		RankRights:     rank.Rights,
		MoneyPerDay:    rank.MoneyPerDay,
		RemainingMoney: g.remainingMoney(member, rank, withdrawals.Money),
	}
	if g.isGuildMaster(member) {
		state.MoneyPerDay = repo.BankWithdrawUnlimited
	}
	for _, tab := range tabs {
		tr := g.tabRights(member, rightsMap, tab.TabID)
		state.Tabs = append(state.Tabs, BankStateTab{
			BankTab:        tab,
			Rights:         tr.Rights,
			RemainingSlots: g.remainingSlots(tr, withdrawals.Tabs[tab.TabID]),
		})
	}
	if len(tabs) < repo.GuildBankMaxTabs {
		state.NextTabCost = GuildBankTabCosts[len(tabs)]
	}
	return state, nil
}

func (g *guildBankServiceImpl) BankTabItems(ctx context.Context, realmID uint32, guildID, playerGUID uint64, tab uint8) ([]repo.BankItem, error) {
	_, member, _, err := g.memberContext(ctx, realmID, guildID, playerGUID)
	if err != nil {
		return nil, err
	}

	rightsMap, err := g.bankRepo.RankTabRights(ctx, realmID, guildID)
	if err != nil {
		return nil, err
	}
	if g.tabRights(member, rightsMap, tab).Rights&repo.BankRightViewTab == 0 {
		return nil, ErrNotEnoughRight
	}

	return g.bankRepo.BankTabItems(ctx, realmID, guildID, tab)
}

func (g *guildBankServiceImpl) BankDepositMoney(ctx context.Context, realmID uint32, guildID, playerGUID, amount uint64) (uint64, uint32, error) {
	guild, member, rank, err := g.memberContext(ctx, realmID, guildID, playerGUID)
	if err != nil {
		return 0, 0, err
	}

	newMoney, err := g.bankRepo.DepositMoney(ctx, realmID, guildID, playerGUID, amount)
	if err != nil {
		return 0, 0, err
	}

	g.publishMoney(guild, newMoney)

	withdrawals, err := g.bankRepo.MemberWithdrawals(ctx, realmID, playerGUID)
	if err != nil {
		return newMoney, 0, err
	}
	return newMoney, g.remainingMoney(member, rank, withdrawals.Money), nil
}

func (g *guildBankServiceImpl) BankWithdrawMoney(ctx context.Context, realmID uint32, guildID, playerGUID, amount uint64) (uint64, uint32, error) {
	guild, member, rank, err := g.memberContext(ctx, realmID, guildID, playerGUID)
	if err != nil {
		return 0, 0, err
	}

	if !g.isGuildMaster(member) && rank.Rights&repo.RightWithdrawGold == 0 {
		return 0, 0, ErrNotEnoughRight
	}

	dailyLimit := rank.MoneyPerDay
	if g.isGuildMaster(member) {
		dailyLimit = repo.BankWithdrawUnlimited
	}

	newMoney, err := g.bankRepo.WithdrawMoney(ctx, realmID, guildID, playerGUID, amount, dailyLimit)
	if err != nil {
		return 0, 0, err
	}

	g.publishMoney(guild, newMoney)

	withdrawals, err := g.bankRepo.MemberWithdrawals(ctx, realmID, playerGUID)
	if err != nil {
		return newMoney, 0, err
	}
	return newMoney, g.remainingMoney(member, rank, withdrawals.Money), nil
}

func (g *guildBankServiceImpl) BankDepositItem(ctx context.Context, realmID uint32, guildID, playerGUID uint64, tab, slot uint8, item repo.BankItem, restore bool) (uint8, error) {
	guild, member, _, err := g.memberContext(ctx, realmID, guildID, playerGUID)
	if err != nil {
		return 0, err
	}

	if !restore {
		rightsMap, err := g.bankRepo.RankTabRights(ctx, realmID, guildID)
		if err != nil {
			return 0, err
		}
		if g.tabRights(member, rightsMap, tab).Rights&repo.BankRightDepositItem != repo.BankRightDepositItem {
			return 0, ErrNotEnoughRight
		}
	}

	// The wire carries the full 64-bit ObjectGuid; guild_bank_item.item_guid
	// stores the 32-bit counter, like item_instance.guid it references.
	item.ItemGUID &= 0xFFFFFFFF

	placedSlot, err := g.bankRepo.DepositItem(ctx, realmID, guildID, playerGUID, tab, slot, item, !restore)
	if err != nil {
		return 0, err
	}

	g.publishTab(guild, tab)
	return placedSlot, nil
}

func (g *guildBankServiceImpl) BankWithdrawItem(ctx context.Context, realmID uint32, guildID, playerGUID uint64, tab, slot uint8) (repo.BankItem, uint32, error) {
	guild, member, _, err := g.memberContext(ctx, realmID, guildID, playerGUID)
	if err != nil {
		return repo.BankItem{}, 0, err
	}

	rightsMap, err := g.bankRepo.RankTabRights(ctx, realmID, guildID)
	if err != nil {
		return repo.BankItem{}, 0, err
	}
	tr := g.tabRights(member, rightsMap, tab)
	if tr.Rights&repo.BankRightViewTab == 0 {
		return repo.BankItem{}, 0, ErrNotEnoughRight
	}

	item, err := g.bankRepo.WithdrawItem(ctx, realmID, guildID, playerGUID, tab, slot, tr.SlotsPerDay)
	if err != nil {
		return repo.BankItem{}, 0, err
	}

	g.publishTab(guild, tab)

	withdrawals, err := g.bankRepo.MemberWithdrawals(ctx, realmID, playerGUID)
	if err != nil {
		return item, 0, err
	}
	return item, g.remainingSlots(tr, withdrawals.Tabs[tab]), nil
}

func (g *guildBankServiceImpl) BankMoveItem(ctx context.Context, realmID uint32, guildID, playerGUID uint64, srcTab, srcSlot, dstTab, dstSlot uint8, splitCount uint32) error {
	guild, member, _, err := g.memberContext(ctx, realmID, guildID, playerGUID)
	if err != nil {
		return err
	}

	rightsMap, err := g.bankRepo.RankTabRights(ctx, realmID, guildID)
	if err != nil {
		return err
	}
	if g.tabRights(member, rightsMap, srcTab).Rights&repo.BankRightViewTab == 0 {
		return ErrNotEnoughRight
	}
	if srcTab != dstTab &&
		g.tabRights(member, rightsMap, dstTab).Rights&repo.BankRightDepositItem != repo.BankRightDepositItem {
		return ErrNotEnoughRight
	}

	if err = g.bankRepo.MoveItem(ctx, realmID, guildID, playerGUID, srcTab, srcSlot, dstTab, dstSlot, splitCount); err != nil {
		return err
	}

	g.publishTab(guild, srcTab)
	if dstTab != srcTab {
		g.publishTab(guild, dstTab)
	}
	return nil
}

func (g *guildBankServiceImpl) BankBuyTab(ctx context.Context, realmID uint32, guildID, playerGUID uint64, tab uint8, paidCost uint32) error {
	guild, _, _, err := g.memberContext(ctx, realmID, guildID, playerGUID)
	if err != nil {
		return err
	}

	if tab >= repo.GuildBankMaxTabs || GuildBankTabCosts[tab] != paidCost {
		return ErrBankWrongCost
	}

	if err = g.bankRepo.CreateTab(ctx, realmID, guildID, playerGUID, tab, uint64(paidCost)); err != nil {
		return err
	}

	g.publishTabsChanged(guild)
	return nil
}

func (g *guildBankServiceImpl) BankSetTabInfo(ctx context.Context, realmID uint32, guildID, playerGUID uint64, tab uint8, name, icon string) error {
	guild, _, _, err := g.memberContext(ctx, realmID, guildID, playerGUID)
	if err != nil {
		return err
	}

	// Only the guild master can rename tabs (client UI parity).
	if guild.LeaderGUID != playerGUID {
		return ErrNotEnoughRight
	}

	if err = g.bankRepo.SetTabInfo(ctx, realmID, guildID, tab, name, icon); err != nil {
		return err
	}

	g.publishTabsChanged(guild)
	return nil
}

func (g *guildBankServiceImpl) BankSetTabText(ctx context.Context, realmID uint32, guildID, playerGUID uint64, tab uint8, text string) error {
	guild, member, _, err := g.memberContext(ctx, realmID, guildID, playerGUID)
	if err != nil {
		return err
	}

	rightsMap, err := g.bankRepo.RankTabRights(ctx, realmID, guildID)
	if err != nil {
		return err
	}
	if g.tabRights(member, rightsMap, tab).Rights&repo.BankRightUpdateText == 0 {
		return ErrNotEnoughRight
	}

	if err = g.bankRepo.SetTabText(ctx, realmID, guildID, tab, text); err != nil {
		return err
	}

	g.publishText(guild, tab)
	return nil
}

func (g *guildBankServiceImpl) BankLog(ctx context.Context, realmID uint32, guildID, playerGUID uint64, tab uint8) ([]repo.BankLogEntry, error) {
	_, member, _, err := g.memberContext(ctx, realmID, guildID, playerGUID)
	if err != nil {
		return nil, err
	}

	if tab < repo.GuildBankMaxTabs {
		rightsMap, err := g.bankRepo.RankTabRights(ctx, realmID, guildID)
		if err != nil {
			return nil, err
		}
		if g.tabRights(member, rightsMap, tab).Rights&repo.BankRightViewTab == 0 {
			return nil, ErrNotEnoughRight
		}
	}

	return g.bankRepo.BankLog(ctx, realmID, guildID, tab)
}

func (g *guildBankServiceImpl) RankBankTabRights(ctx context.Context, realmID uint32, guildID uint64) (map[uint8][repo.GuildBankMaxTabs]repo.BankTabRights, error) {
	return g.bankRepo.RankTabRights(ctx, realmID, guildID)
}

func (g *guildBankServiceImpl) SetRankBankTabRights(ctx context.Context, realmID uint32, guildID uint64, rank uint8, rights []repo.BankTabRights) error {
	tabs, err := g.bankRepo.BankTabs(ctx, realmID, guildID)
	if err != nil {
		return err
	}
	if len(rights) > len(tabs) {
		rights = rights[:len(tabs)]
	}
	if len(rights) == 0 {
		return nil
	}
	return g.bankRepo.SetRankTabRights(ctx, realmID, guildID, rank, rights)
}

func (g *guildBankServiceImpl) SetRankBankTabRightsByChanger(ctx context.Context, realmID uint32, changerGUID uint64, rank uint8, rights []repo.BankTabRights) error {
	guildID, err := g.guildsRepo.GuildIDByRealmAndMemberGUID(ctx, realmID, changerGUID)
	if err != nil {
		return err
	}
	if guildID == 0 {
		return ErrGuildNotFound
	}
	return g.SetRankBankTabRights(ctx, realmID, guildID, rank, rights)
}

func (g *guildBankServiceImpl) genericEvent(guild *repo.Guild) events.GenericGuildEvent {
	membersOnline := make([]uint64, 0, len(guild.GuildMembers))
	for _, member := range guild.GuildMembers {
		if member.Status == repo.GuildMemberStatusOnline {
			membersOnline = append(membersOnline, member.PlayerGUID)
		}
	}
	return events.GenericGuildEvent{
		ServiceID:     guildserver.ServiceID,
		RealmID:       guild.RealmID,
		GuildID:       guild.ID,
		GuildName:     guild.Name,
		MembersOnline: membersOnline,
	}
}

func (g *guildBankServiceImpl) publishMoney(guild *repo.Guild, money uint64) {
	_ = g.eventsProducer.BankMoneyUpdated(&events.GuildEventBankMoneyUpdatedPayload{
		GenericGuildEvent: g.genericEvent(guild),
		Money:             money,
	})
}

func (g *guildBankServiceImpl) publishTab(guild *repo.Guild, tab uint8) {
	_ = g.eventsProducer.BankTabUpdated(&events.GuildEventBankTabUpdatedPayload{
		GenericGuildEvent: g.genericEvent(guild),
		TabID:             tab,
	})
}

func (g *guildBankServiceImpl) publishTabsChanged(guild *repo.Guild) {
	_ = g.eventsProducer.BankTabsChanged(&events.GuildEventBankTabsChangedPayload{
		GenericGuildEvent: g.genericEvent(guild),
	})
}

func (g *guildBankServiceImpl) publishText(guild *repo.Guild, tab uint8) {
	_ = g.eventsProducer.BankTextUpdated(&events.GuildEventBankTextUpdatedPayload{
		GenericGuildEvent: g.genericEvent(guild),
		TabID:             tab,
	})
}
