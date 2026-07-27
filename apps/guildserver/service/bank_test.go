package service

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/stretchr/testify/mock"
	"github.com/walkline/ToCloud9/apps/guildserver/repo"

	eventsMocks "github.com/walkline/ToCloud9/shared/events/mocks"
)

// fakeBankRepo is a hand-rolled GuildBankRepo recording calls.
type fakeBankRepo struct {
	repo.GuildBankRepo

	money       uint64
	tabs        []repo.BankTab
	rights      map[uint8][repo.GuildBankMaxTabs]repo.BankTabRights
	withdrawals repo.BankWithdrawals

	depositedItem  *repo.BankItem
	depositLogged  bool
	withdrawnLimit uint32
	movedSplit     uint32
	moveCalled     bool
}

func (f *fakeBankRepo) BankMoney(_ context.Context, _ uint32, _ uint64) (uint64, error) {
	return f.money, nil
}

func (f *fakeBankRepo) BankTabs(_ context.Context, _ uint32, _ uint64) ([]repo.BankTab, error) {
	return f.tabs, nil
}

func (f *fakeBankRepo) RankTabRights(_ context.Context, _ uint32, _ uint64) (map[uint8][repo.GuildBankMaxTabs]repo.BankTabRights, error) {
	return f.rights, nil
}

func (f *fakeBankRepo) MemberWithdrawals(_ context.Context, _ uint32, _ uint64) (repo.BankWithdrawals, error) {
	return f.withdrawals, nil
}

func (f *fakeBankRepo) DepositMoney(_ context.Context, _ uint32, _, _, amount uint64) (uint64, error) {
	f.money += amount
	return f.money, nil
}

func (f *fakeBankRepo) WithdrawMoney(_ context.Context, _ uint32, _, _, amount uint64, dailyLimit uint32) (uint64, error) {
	f.withdrawnLimit = dailyLimit
	if f.money < amount {
		return 0, repo.ErrBankNotEnoughMoney
	}
	f.money -= amount
	return f.money, nil
}

func (f *fakeBankRepo) DepositItem(_ context.Context, _ uint32, _, _ uint64, _, slot uint8, item repo.BankItem, logEvent bool) (uint8, error) {
	f.depositedItem = &item
	f.depositLogged = logEvent
	if slot == repo.BankSlotAuto {
		return 0, nil
	}
	return slot, nil
}

func (f *fakeBankRepo) WithdrawItem(_ context.Context, _ uint32, _, _ uint64, _, _ uint8, dailyLimit uint32) (repo.BankItem, error) {
	f.withdrawnLimit = dailyLimit
	return repo.BankItem{ItemGUID: 111, Entry: 2589, Count: 20}, nil
}

func (f *fakeBankRepo) MoveItem(_ context.Context, _ uint32, _, _ uint64, _, _, _, _ uint8, splitCount uint32) error {
	f.moveCalled = true
	f.movedSplit = splitCount
	return nil
}

// fakeGuildsRepo serves one static guild.
type fakeGuildsRepo struct {
	repo.GuildsRepo
	guild *repo.Guild
}

func (f *fakeGuildsRepo) GuildByRealmAndID(_ context.Context, _ uint32, _ uint64) (*repo.Guild, error) {
	return f.guild, nil
}

const (
	testLeaderGUID  = uint64(1)
	testOfficerGUID = uint64(2)
	testMemberGUID  = uint64(3)
)

func bankTestGuild() *repo.Guild {
	return &repo.Guild{
		RealmID:    1,
		ID:         7,
		Name:       "Test Guild",
		LeaderGUID: testLeaderGUID,
		GuildRanks: []repo.GuildRank{
			{Rank: 0, Name: "GM", Rights: repo.RightAll, MoneyPerDay: 0},
			{Rank: 1, Name: "Officer", Rights: repo.RightAll, MoneyPerDay: 5000},
			{Rank: 3, Name: "Member", Rights: repo.RightEmpty, MoneyPerDay: 0},
		},
		GuildMembers: []*repo.GuildMember{
			{PlayerGUID: testLeaderGUID, Rank: 0},
			{PlayerGUID: testOfficerGUID, Rank: 1},
			{PlayerGUID: testMemberGUID, Rank: 3},
		},
	}
}

func bankTestService(bankRepo *fakeBankRepo) GuildBankService {
	producer := &eventsMocks.GuildServiceProducer{}
	producer.On("BankMoneyUpdated", mock.Anything).Return(nil)
	producer.On("BankTabUpdated", mock.Anything).Return(nil)
	producer.On("BankTabsChanged", mock.Anything).Return(nil)
	producer.On("BankTextUpdated", mock.Anything).Return(nil)
	return NewGuildBankService(bankRepo, &fakeGuildsRepo{guild: bankTestGuild()}, producer)
}

func TestBankStateRightsMatrix(t *testing.T) {
	bankRepo := &fakeBankRepo{
		money: 123456,
		tabs:  []repo.BankTab{{TabID: 0, Name: "One"}, {TabID: 1, Name: "Two"}},
		rights: map[uint8][repo.GuildBankMaxTabs]repo.BankTabRights{
			1: {{Rights: repo.BankRightDepositItem, SlotsPerDay: 10}, {Rights: 0, SlotsPerDay: 0}},
		},
		withdrawals: repo.BankWithdrawals{Tabs: [repo.GuildBankMaxTabs]uint32{4}, Money: 1000},
	}
	svc := bankTestService(bankRepo)

	// Guild master: everything unlimited.
	state, err := svc.BankState(context.Background(), 1, 7, testLeaderGUID)
	assert.Nil(t, err)
	assert.Equal(t, repo.BankWithdrawUnlimited, state.RemainingMoney)
	assert.Equal(t, repo.BankWithdrawUnlimited, state.MoneyPerDay)
	assert.Equal(t, uint8(repo.BankRightFull), state.Tabs[0].Rights)
	assert.Equal(t, repo.BankWithdrawUnlimited, state.Tabs[0].RemainingSlots)
	assert.Equal(t, uint32(5000000), state.NextTabCost)

	// Officer: tab rights from the table, counters applied.
	state, err = svc.BankState(context.Background(), 1, 7, testOfficerGUID)
	assert.Nil(t, err)
	assert.Equal(t, uint32(5000-1000), state.RemainingMoney)
	assert.Equal(t, uint8(repo.BankRightDepositItem), state.Tabs[0].Rights)
	assert.Equal(t, uint32(10-4), state.Tabs[0].RemainingSlots)
	assert.Equal(t, uint32(0), state.Tabs[1].RemainingSlots)

	// Member without withdraw right: no gold, no slots.
	state, err = svc.BankState(context.Background(), 1, 7, testMemberGUID)
	assert.Nil(t, err)
	assert.Equal(t, uint32(0), state.RemainingMoney)
	assert.Equal(t, uint32(0), state.Tabs[0].RemainingSlots)
}

func TestBankWithdrawMoneyRequiresRight(t *testing.T) {
	bankRepo := &fakeBankRepo{money: 100000, tabs: []repo.BankTab{{TabID: 0}}}
	svc := bankTestService(bankRepo)

	// Member rank has no withdraw-gold right.
	_, _, err := svc.BankWithdrawMoney(context.Background(), 1, 7, testMemberGUID, 100)
	assert.True(t, errors.Is(err, ErrNotEnoughRight))

	// Officer passes its rank daily limit to the repo guard.
	_, _, err = svc.BankWithdrawMoney(context.Background(), 1, 7, testOfficerGUID, 100)
	assert.Nil(t, err)
	assert.Equal(t, uint32(5000), bankRepo.withdrawnLimit)

	// Guild master is unlimited.
	_, _, err = svc.BankWithdrawMoney(context.Background(), 1, 7, testLeaderGUID, 100)
	assert.Nil(t, err)
	assert.Equal(t, repo.BankWithdrawUnlimited, bankRepo.withdrawnLimit)
}

func TestBankDepositItemRightsAndRestore(t *testing.T) {
	bankRepo := &fakeBankRepo{
		tabs: []repo.BankTab{{TabID: 0}},
		rights: map[uint8][repo.GuildBankMaxTabs]repo.BankTabRights{
			1: {{Rights: repo.BankRightViewTab, SlotsPerDay: 10}},
		},
	}
	svc := bankTestService(bankRepo)
	item := repo.BankItem{ItemGUID: 42, Entry: 2589, Count: 20}

	// View-only rank cannot deposit.
	_, err := svc.BankDepositItem(context.Background(), 1, 7, testOfficerGUID, 0, 3, item, false)
	assert.True(t, errors.Is(err, ErrNotEnoughRight))
	assert.Nil(t, bankRepo.depositedItem)

	// The restore path (failed withdraw compensation) bypasses the check
	// and is not logged as a fresh deposit.
	_, err = svc.BankDepositItem(context.Background(), 1, 7, testOfficerGUID, 0, 3, item, true)
	assert.Nil(t, err)
	assert.NotNil(t, bankRepo.depositedItem)
	assert.False(t, bankRepo.depositLogged)

	// Guild master deposits freely, logged. The wire guid (full 64-bit
	// ObjectGuid, high bits 0x4000...) must be normalized to the counter
	// that guild_bank_item.item_guid (int unsigned) references.
	item.ItemGUID = 0x4000000000000000 | 42
	_, err = svc.BankDepositItem(context.Background(), 1, 7, testLeaderGUID, 0, 3, item, false)
	assert.Nil(t, err)
	assert.True(t, bankRepo.depositLogged)
	assert.Equal(t, uint64(42), bankRepo.depositedItem.ItemGUID)
}

func TestBankMoveItemRights(t *testing.T) {
	bankRepo := &fakeBankRepo{
		tabs: []repo.BankTab{{TabID: 0}, {TabID: 1}},
		rights: map[uint8][repo.GuildBankMaxTabs]repo.BankTabRights{
			1: {{Rights: repo.BankRightViewTab, SlotsPerDay: 10}, {Rights: 0}},
		},
	}
	svc := bankTestService(bankRepo)

	// Same tab: view right is enough.
	err := svc.BankMoveItem(context.Background(), 1, 7, testOfficerGUID, 0, 1, 0, 2, 0)
	assert.Nil(t, err)
	assert.True(t, bankRepo.moveCalled)

	// Cross tab without deposit right on the destination.
	err = svc.BankMoveItem(context.Background(), 1, 7, testOfficerGUID, 0, 1, 1, 2, 0)
	assert.True(t, errors.Is(err, ErrNotEnoughRight))
}

func TestBankBuyTabCostValidation(t *testing.T) {
	bankRepo := &fakeBankRepo{tabs: []repo.BankTab{{TabID: 0}}}
	svc := bankTestService(bankRepo)

	err := svc.BankBuyTab(context.Background(), 1, 7, testMemberGUID, 1, 999)
	assert.True(t, errors.Is(err, ErrBankWrongCost))
}

func TestBankSetTabInfoLeaderOnly(t *testing.T) {
	bankRepo := &fakeBankRepo{tabs: []repo.BankTab{{TabID: 0}}}
	svc := bankTestService(bankRepo)

	err := svc.BankSetTabInfo(context.Background(), 1, 7, testOfficerGUID, 0, "Tab", "Icon")
	assert.True(t, errors.Is(err, ErrNotEnoughRight))
}
