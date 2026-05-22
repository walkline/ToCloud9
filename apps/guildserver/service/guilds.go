package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/walkline/ToCloud9/apps/guildserver"
	"github.com/walkline/ToCloud9/apps/guildserver/repo"
	"github.com/walkline/ToCloud9/shared/events"
	"github.com/walkline/ToCloud9/shared/wow"
)

var (
	ErrNotEnoughRight  = errors.New("not enough rights")
	ErrGuildNotFound   = errors.New("guild not found")
	ErrLeaderCantLeave = errors.New("leader can't leave")
	ErrGuildNotAllied  = errors.New("guild target is not allied")
)

const guildPetitionType = 9

type GuildPetitionOfferStatus uint8

const (
	GuildPetitionOfferOK GuildPetitionOfferStatus = iota
	GuildPetitionOfferNotFound
	GuildPetitionOfferNotOwner
	GuildPetitionOfferTargetNotFound
	GuildPetitionOfferTargetAlreadyInGuild
	GuildPetitionOfferTargetAlreadyInvited
	GuildPetitionOfferFailed
)

type GuildPetitionSignStatus uint8

const (
	GuildPetitionSignOK GuildPetitionSignStatus = iota
	GuildPetitionSignAlreadySigned
	GuildPetitionSignAlreadyInGuild
	GuildPetitionSignCantSignOwn
	GuildPetitionSignNotServer
	GuildPetitionSignNotFound
	GuildPetitionSignFull
	GuildPetitionSignFailed
	GuildPetitionSignAlreadyInvited
)

type GuildBankStatus uint8

const (
	GuildBankStatusOK GuildBankStatus = iota
	GuildBankStatusFailed
	GuildBankStatusGuildNotFound
	GuildBankStatusNotInGuild
	GuildBankStatusNotEnoughRights
	GuildBankStatusInvalidTab
	GuildBankStatusInvalidSlot
	GuildBankStatusNotEnoughMoney
	GuildBankStatusBankFull
	GuildBankStatusWithdrawLimit
	GuildBankStatusItemNotFound
)

// InviteAcceptedParams represents parameters for InviteAcceptedParams.InviteAccepted function.
type InviteAcceptedParams struct {
	CharGUID    uint64
	CharName    string
	CharRace    uint8
	CharClass   uint8
	CharLvl     uint8
	CharGender  uint8
	CharAreaID  uint32
	CharAccount uint64

	AllowCrossFaction bool
}

// GuildRank represents guild rank.
type GuildRank struct {
	RankID        uint8
	Name          string
	Rights        uint32
	MoneyPerDay   uint32
	BankTabRights [repo.GuildBankMaxTabs]repo.GuildBankTabRight
}

// GuildService is service to handle guilds.
type GuildService interface {
	// GuildByRealmAndID returns guild by realmID and guildID.
	GuildByRealmAndID(ctx context.Context, realmID uint32, guildID uint64) (*repo.Guild, error)

	GetGuildBank(ctx context.Context, realmID uint32, guildID, memberGUID uint64, tabID uint8, fullUpdate bool) (*repo.GuildBank, int32, GuildBankStatus, error)
	GetGuildBankLog(ctx context.Context, realmID uint32, guildID, memberGUID uint64, tabID uint8) ([]repo.GuildBankLogEntry, GuildBankStatus, error)
	GetGuildBankTabText(ctx context.Context, realmID uint32, guildID, memberGUID uint64, tabID uint8) (string, GuildBankStatus, error)
	UpdateGuildBankTab(ctx context.Context, realmID uint32, guildID, memberGUID uint64, tabID uint8, name, icon string) (GuildBankStatus, error)
	SetGuildBankTabText(ctx context.Context, realmID uint32, guildID, memberGUID uint64, tabID uint8, text string) (GuildBankStatus, error)
	BuyGuildBankTab(ctx context.Context, realmID uint32, guildID, memberGUID uint64, tabID uint8, cost uint32) (GuildBankStatus, error)
	DepositGuildBankMoney(ctx context.Context, realmID uint32, guildID, memberGUID uint64, amount uint32) (GuildBankStatus, error)
	WithdrawGuildBankMoney(ctx context.Context, realmID uint32, guildID, memberGUID uint64, amount uint32, repair bool) (uint32, GuildBankStatus, error)
	RollbackGuildBankMoneyWithdraw(ctx context.Context, realmID uint32, guildID, memberGUID uint64, amount uint32, repair bool, logGUID uint32) (GuildBankStatus, error)
	DepositGuildBankItem(ctx context.Context, realmID uint32, guildID, memberGUID uint64, tabID, slotID uint8, item repo.GuildBankItem) (GuildBankStatus, error)
	WithdrawGuildBankItem(ctx context.Context, realmID uint32, guildID, memberGUID uint64, tabID, slotID uint8, count uint32) (*repo.GuildBankItem, uint32, GuildBankStatus, error)
	RollbackGuildBankItemWithdraw(ctx context.Context, realmID uint32, guildID, memberGUID uint64, tabID, slotID uint8, item repo.GuildBankItem, logGUID uint32) ([]uint8, GuildBankStatus, error)
	MoveGuildBankItem(ctx context.Context, realmID uint32, guildID, memberGUID uint64, sourceTabID, sourceSlotID, destinationTabID, destinationSlotID uint8, count uint32) ([]uint8, GuildBankStatus, error)

	// InviteMember creates invite to the guild.
	InviteMember(ctx context.Context, realmID uint32, inviterGUID uint64, inviteeGUID uint64, inviteeName string, inviteeRace uint8, allowCrossFaction bool) error

	// InviteAccepted handles guild invite accept users action. Returns guild id of the new member.
	InviteAccepted(ctx context.Context, realmID uint32, params InviteAcceptedParams) (uint64, error)

	// Leave handles player leave command.
	Leave(ctx context.Context, realmID uint32, charGUID uint64) error

	// Kick handles kick of the guild member.
	Kick(ctx context.Context, realmID uint32, kicker, target uint64) error

	// SetMessageOfTheDay sets message of the day for a guild.
	SetMessageOfTheDay(ctx context.Context, realmID uint32, updaterGUID uint64, message string) error

	// SetGuildInfo sets info text for the guild.
	SetGuildInfo(ctx context.Context, realmID uint32, updaterGUID uint64, info string) error

	// SetMemberPublicNote sets public note for the guild member.
	SetMemberPublicNote(ctx context.Context, realmID uint32, updaterGUID, targetGUID uint64, note string) error

	// SetMemberOfficerNote sets officer note for the guild member.
	SetMemberOfficerNote(ctx context.Context, realmID uint32, updaterGUID, targetGUID uint64, note string) error

	// UpdateGuildRank updates rank of the guild.
	UpdateGuildRank(ctx context.Context, realmID uint32, updaterGUID uint64, rank GuildRank) error

	// AddGuildRank creates new guild rank in the updaters gild with given name.
	AddGuildRank(ctx context.Context, realmID uint32, updaterGUID uint64, rankName string) error

	// DeleteLastGuildRank deletes the last guild rank from updaters guild.
	DeleteLastGuildRank(ctx context.Context, realmID uint32, updaterGUID uint64) error

	// PromoteMember promotes guild member to the 1 rank higher.
	PromoteMember(ctx context.Context, realmID uint32, updaterGUID, targetGUID uint64) error

	// DemoteMember demotes guild member to the 1 rank lower.
	DemoteMember(ctx context.Context, realmID uint32, updaterGUID, targetGUID uint64) error

	// SendGuildMessage sends guild message to the online player.
	SendGuildMessage(ctx context.Context, realmID uint32, senderGUID uint64, message string, lang uint32, isOfficers bool, senderChatTag uint8) error

	// GuildPetitionByGUID returns a native AzerothCore guild petition by item GUID.
	GuildPetitionByGUID(ctx context.Context, realmID uint32, petitionGUID uint64) (*repo.GuildPetition, error)

	// OfferGuildPetition routes a native guild petition offer to an online same-realm target.
	OfferGuildPetition(ctx context.Context, realmID uint32, ownerGUID, targetGUID uint64, targetName string, petitionGUID uint64) (GuildPetitionOfferStatus, error)

	// SignGuildPetition validates and persists a native guild petition signature.
	SignGuildPetition(ctx context.Context, realmID uint32, signerGUID uint64, signerName string, signerAccountID uint32, signerGuildID uint32, petitionGUID uint64) (GuildPetitionSignStatus, error)

	events.GWGuildCreatedHandler
}

// guildServiceImpl is implementation of GuildService.
type guildServiceImpl struct {
	guildsRepo        repo.GuildsRepo
	guildBankRepo     repo.GuildsRepo
	itemGUIDAllocator ItemGUIDAllocator
	eventsProducer    events.GuildServiceProducer
}

// NewGuildService creates GuildService.
func NewGuildService(guildsRepo repo.GuildsRepo, eventsProducer events.GuildServiceProducer) GuildService {
	return NewGuildServiceWithBankRepo(guildsRepo, guildsRepo, eventsProducer)
}

// NewGuildServiceWithBankRepo creates GuildService with a separate uncached repo for guild bank paths.
func NewGuildServiceWithBankRepo(guildsRepo repo.GuildsRepo, guildBankRepo repo.GuildsRepo, eventsProducer events.GuildServiceProducer) GuildService {
	return NewGuildServiceWithBankRepoAndItemGUIDAllocator(guildsRepo, guildBankRepo, nil, eventsProducer)
}

// NewGuildServiceWithBankRepoAndItemGUIDAllocator creates GuildService with a separate uncached repo and item GUID allocator for guild bank paths.
func NewGuildServiceWithBankRepoAndItemGUIDAllocator(guildsRepo repo.GuildsRepo, guildBankRepo repo.GuildsRepo, itemGUIDAllocator ItemGUIDAllocator, eventsProducer events.GuildServiceProducer) GuildService {
	return &guildServiceImpl{
		guildsRepo:        guildsRepo,
		guildBankRepo:     guildBankRepo,
		itemGUIDAllocator: itemGUIDAllocator,
		eventsProducer:    eventsProducer,
	}
}

func (g *guildServiceImpl) bankRepo() repo.GuildsRepo {
	if g.guildBankRepo != nil {
		return g.guildBankRepo
	}
	return g.guildsRepo
}

// GuildByRealmAndID returns guild by realmID and guildID.
func (g *guildServiceImpl) GuildByRealmAndID(ctx context.Context, realmID uint32, guildID uint64) (*repo.Guild, error) {
	return g.guildsRepo.GuildByRealmAndID(ctx, realmID, guildID)
}

func (g *guildServiceImpl) GetGuildBank(ctx context.Context, realmID uint32, guildID, memberGUID uint64, tabID uint8, fullUpdate bool) (*repo.GuildBank, int32, GuildBankStatus, error) {
	guild, member, rank, status, err := g.guildBankMemberRank(ctx, realmID, guildID, memberGUID)
	if status != GuildBankStatusOK || err != nil {
		return nil, 0, status, err
	}
	if status = g.guildBankCanViewTab(guild, rank, tabID); status != GuildBankStatusOK {
		return nil, 0, status, nil
	}

	bank, err := g.bankRepo().GuildBank(ctx, realmID, guildID, tabID, fullUpdate)
	if err != nil {
		return nil, 0, GuildBankStatusFailed, err
	}
	return bank, g.guildBankRemainingSlots(rank, member, tabID), GuildBankStatusOK, nil
}

func (g *guildServiceImpl) GetGuildBankLog(ctx context.Context, realmID uint32, guildID, memberGUID uint64, tabID uint8) ([]repo.GuildBankLogEntry, GuildBankStatus, error) {
	guild, _, rank, status, err := g.guildBankMemberRank(ctx, realmID, guildID, memberGUID)
	if status != GuildBankStatusOK || err != nil {
		return nil, status, err
	}
	if status = g.guildBankCanViewTab(guild, rank, tabID); status != GuildBankStatusOK {
		return nil, status, nil
	}

	entries, err := g.bankRepo().GuildBankLog(ctx, realmID, guildID, tabID)
	if err != nil {
		return nil, GuildBankStatusFailed, err
	}
	return entries, GuildBankStatusOK, nil
}

func (g *guildServiceImpl) GetGuildBankTabText(ctx context.Context, realmID uint32, guildID, memberGUID uint64, tabID uint8) (string, GuildBankStatus, error) {
	bank, _, status, err := g.GetGuildBank(ctx, realmID, guildID, memberGUID, tabID, false)
	if status != GuildBankStatusOK || err != nil {
		return "", status, err
	}
	for _, tab := range bank.Tabs {
		if tab.TabID == tabID {
			return tab.Text, GuildBankStatusOK, nil
		}
	}
	return "", GuildBankStatusInvalidTab, nil
}

func (g *guildServiceImpl) UpdateGuildBankTab(ctx context.Context, realmID uint32, guildID, memberGUID uint64, tabID uint8, name, icon string) (GuildBankStatus, error) {
	guild, _, rank, status, err := g.guildBankMemberRank(ctx, realmID, guildID, memberGUID)
	if status != GuildBankStatusOK || err != nil {
		return status, err
	}
	if status = g.guildBankCanUpdateText(guild, rank, tabID); status != GuildBankStatusOK {
		return status, nil
	}
	if err = g.bankRepo().SetGuildBankTabInfo(ctx, realmID, guildID, tabID, name, icon); err != nil {
		return repoGuildBankErrorToStatus(err), err
	}
	return GuildBankStatusOK, nil
}

func (g *guildServiceImpl) SetGuildBankTabText(ctx context.Context, realmID uint32, guildID, memberGUID uint64, tabID uint8, text string) (GuildBankStatus, error) {
	guild, _, rank, status, err := g.guildBankMemberRank(ctx, realmID, guildID, memberGUID)
	if status != GuildBankStatusOK || err != nil {
		return status, err
	}
	if status = g.guildBankCanUpdateText(guild, rank, tabID); status != GuildBankStatusOK {
		return status, nil
	}
	if err = g.bankRepo().SetGuildBankTabText(ctx, realmID, guildID, tabID, text); err != nil {
		return repoGuildBankErrorToStatus(err), err
	}
	return GuildBankStatusOK, nil
}

func (g *guildServiceImpl) BuyGuildBankTab(ctx context.Context, realmID uint32, guildID, memberGUID uint64, tabID uint8, cost uint32) (GuildBankStatus, error) {
	guild, _, rank, status, err := g.guildBankMemberRank(ctx, realmID, guildID, memberGUID)
	if status != GuildBankStatusOK || err != nil {
		return status, err
	}
	if rank.Rank != uint8(repo.GuildRankGuildMaster) {
		return GuildBankStatusNotEnoughRights, nil
	}
	if tabID != guild.PurchasedBankTabs || tabID >= repo.GuildBankMaxTabs {
		return GuildBankStatusInvalidTab, nil
	}

	if err = g.bankRepo().BuyGuildBankTab(ctx, realmID, guildID, tabID); err != nil {
		return repoGuildBankErrorToStatus(err), err
	}
	return GuildBankStatusOK, nil
}

func (g *guildServiceImpl) DepositGuildBankMoney(ctx context.Context, realmID uint32, guildID, memberGUID uint64, amount uint32) (GuildBankStatus, error) {
	if _, _, _, status, err := g.guildBankMemberRank(ctx, realmID, guildID, memberGUID); status != GuildBankStatusOK || err != nil {
		return status, err
	}
	if err := g.bankRepo().DepositGuildBankMoney(ctx, realmID, guildID, memberGUID, amount); err != nil {
		return repoGuildBankErrorToStatus(err), err
	}
	return GuildBankStatusOK, nil
}

func (g *guildServiceImpl) WithdrawGuildBankMoney(ctx context.Context, realmID uint32, guildID, memberGUID uint64, amount uint32, repair bool) (uint32, GuildBankStatus, error) {
	_, member, rank, status, err := g.guildBankMemberRank(ctx, realmID, guildID, memberGUID)
	if status != GuildBankStatusOK || err != nil {
		return 0, status, err
	}
	if repair {
		if !rank.HasRight(repo.RightWithdrawRepair) {
			return 0, GuildBankStatusNotEnoughRights, nil
		}
	} else if !rank.HasRight(repo.RightWithdrawGold) {
		return 0, GuildBankStatusNotEnoughRights, nil
	}
	if g.guildBankRemainingMoney(rank, member) >= 0 && uint32(g.guildBankRemainingMoney(rank, member)) < amount {
		return 0, GuildBankStatusWithdrawLimit, nil
	}
	logGUID, err := g.bankRepo().WithdrawGuildBankMoney(ctx, realmID, guildID, memberGUID, amount, repair)
	if err != nil {
		return 0, repoGuildBankErrorToStatus(err), err
	}
	return logGUID, GuildBankStatusOK, nil
}

func (g *guildServiceImpl) RollbackGuildBankMoneyWithdraw(ctx context.Context, realmID uint32, guildID, memberGUID uint64, amount uint32, repair bool, logGUID uint32) (GuildBankStatus, error) {
	if _, _, _, status, err := g.guildBankMemberRank(ctx, realmID, guildID, memberGUID); status != GuildBankStatusOK || err != nil {
		return status, err
	}
	if err := g.bankRepo().RollbackGuildBankMoneyWithdraw(ctx, realmID, guildID, memberGUID, amount, repair, logGUID); err != nil {
		return repoGuildBankErrorToStatus(err), err
	}
	return GuildBankStatusOK, nil
}

func (g *guildServiceImpl) DepositGuildBankItem(ctx context.Context, realmID uint32, guildID, memberGUID uint64, tabID, slotID uint8, item repo.GuildBankItem) (GuildBankStatus, error) {
	guild, _, rank, status, err := g.guildBankMemberRank(ctx, realmID, guildID, memberGUID)
	if status != GuildBankStatusOK || err != nil {
		return status, err
	}
	if status = g.guildBankCanDeposit(guild, rank, tabID); status != GuildBankStatusOK {
		return status, nil
	}
	if err = g.bankRepo().DepositGuildBankItem(ctx, realmID, guildID, memberGUID, tabID, slotID, item); err != nil {
		return repoGuildBankErrorToStatus(err), err
	}
	return GuildBankStatusOK, nil
}

func (g *guildServiceImpl) WithdrawGuildBankItem(ctx context.Context, realmID uint32, guildID, memberGUID uint64, tabID, slotID uint8, count uint32) (*repo.GuildBankItem, uint32, GuildBankStatus, error) {
	guild, member, rank, status, err := g.guildBankMemberRank(ctx, realmID, guildID, memberGUID)
	if status != GuildBankStatusOK || err != nil {
		return nil, 0, status, err
	}
	if status = g.guildBankCanWithdraw(guild, member, rank, tabID); status != GuildBankStatusOK {
		return nil, 0, status, nil
	}
	splitItemGUID, err := g.nextGuildBankSplitItemGUID(ctx, realmID, count)
	if err != nil {
		return nil, 0, GuildBankStatusFailed, err
	}
	item, logGUID, err := g.bankRepo().WithdrawGuildBankItem(ctx, realmID, guildID, memberGUID, tabID, slotID, count, splitItemGUID)
	if err != nil {
		return nil, 0, repoGuildBankErrorToStatus(err), err
	}
	return item, logGUID, GuildBankStatusOK, nil
}

func (g *guildServiceImpl) RollbackGuildBankItemWithdraw(ctx context.Context, realmID uint32, guildID, memberGUID uint64, tabID, slotID uint8, item repo.GuildBankItem, logGUID uint32) ([]uint8, GuildBankStatus, error) {
	if _, _, _, status, err := g.guildBankMemberRank(ctx, realmID, guildID, memberGUID); status != GuildBankStatusOK || err != nil {
		return nil, status, err
	}
	changed, err := g.bankRepo().RollbackGuildBankItemWithdraw(ctx, realmID, guildID, memberGUID, tabID, slotID, item, logGUID)
	if err != nil {
		return nil, repoGuildBankErrorToStatus(err), err
	}
	return changed, GuildBankStatusOK, nil
}

func (g *guildServiceImpl) MoveGuildBankItem(ctx context.Context, realmID uint32, guildID, memberGUID uint64, sourceTabID, sourceSlotID, destinationTabID, destinationSlotID uint8, count uint32) ([]uint8, GuildBankStatus, error) {
	guild, member, rank, status, err := g.guildBankMemberRank(ctx, realmID, guildID, memberGUID)
	if status != GuildBankStatusOK || err != nil {
		return nil, status, err
	}
	if sourceTabID != destinationTabID {
		if status = g.guildBankCanWithdraw(guild, member, rank, sourceTabID); status != GuildBankStatusOK {
			return nil, status, nil
		}
		if status = g.guildBankCanDeposit(guild, rank, destinationTabID); status != GuildBankStatusOK {
			return nil, status, nil
		}
	} else if status = g.guildBankCanViewTab(guild, rank, sourceTabID); status != GuildBankStatusOK {
		return nil, status, nil
	}
	splitItemGUID, err := g.nextGuildBankSplitItemGUID(ctx, realmID, count)
	if err != nil {
		return nil, GuildBankStatusFailed, err
	}
	changed, err := g.bankRepo().MoveGuildBankItem(ctx, realmID, guildID, memberGUID, sourceTabID, sourceSlotID, destinationTabID, destinationSlotID, count, splitItemGUID)
	if err != nil {
		return nil, repoGuildBankErrorToStatus(err), err
	}
	return changed, GuildBankStatusOK, nil
}

func (g *guildServiceImpl) nextGuildBankSplitItemGUID(ctx context.Context, realmID uint32, count uint32) (uint64, error) {
	if count == 0 {
		return 0, nil
	}
	if g.itemGUIDAllocator == nil {
		return 0, errGuildBankItemGUIDAllocatorMissing
	}
	return g.itemGUIDAllocator.NextItemGUID(ctx, realmID)
}

// InviteMember creates invite to the guild.
func (g *guildServiceImpl) InviteMember(ctx context.Context, realmID uint32, inviterGUID uint64, inviteeGUID uint64, inviteeName string, inviteeRace uint8, allowCrossFaction bool) error {
	guildID, err := g.guildsRepo.GuildIDByRealmAndMemberGUID(ctx, realmID, inviterGUID)
	if err != nil {
		return fmt.Errorf("can't fetch guild id for member, err: %w", err)
	}

	if guildID == 0 {
		return ErrGuildNotFound
	}

	inviteeGuildID, err := g.guildsRepo.GuildIDByRealmAndMemberGUID(ctx, realmID, inviteeGUID)
	if err != nil {
		return fmt.Errorf("can't fetch guild id for member, err: %w", err)
	}

	if inviteeGuildID != 0 {
		return errors.New("invitee already in guild")
	}

	guild, err := g.guildsRepo.GuildByRealmAndID(ctx, realmID, guildID)
	if err != nil {
		return fmt.Errorf("can't fetch guild by id, err: %w", err)
	}

	if guild == nil {
		return ErrGuildNotFound
	}

	member := g.guildMemberForMemberGuid(guild, inviterGUID)
	if member == nil {
		return fmt.Errorf("can't find guild member %d in guild %d", inviterGUID, guildID)
	}

	if !allowCrossFaction && !guildSameFaction(member.Race, inviteeRace) {
		return ErrGuildNotAllied
	}

	rank := g.rankForMember(guild, inviterGUID)
	if rank == nil {
		return fmt.Errorf("can't find rank for player %d and guild %d", inviterGUID, guildID)
	}

	if !rank.HasRight(repo.RightInvite) {
		return ErrNotEnoughRight
	}

	err = g.guildsRepo.AddGuildInvite(ctx, realmID, inviteeGUID, guildID)
	if err != nil {
		return fmt.Errorf("can't invite with bind palyer to the guild, err: %w", err)
	}

	return g.eventsProducer.InviteCreated(&events.GuildEventInviteCreatedPayload{
		ServiceID:   guildserver.ServiceID,
		RealmID:     realmID,
		GuildID:     guildID,
		GuildName:   guild.Name,
		InviterGUID: inviterGUID,
		InviterName: member.Name,
		InviteeGUID: inviteeGUID,
		InviteeName: inviteeName,
	})
}

// InviteAccepted handles guild invite accept users action.
func (g *guildServiceImpl) InviteAccepted(ctx context.Context, realmID uint32, params InviteAcceptedParams) (uint64, error) {
	guildID, err := g.guildsRepo.GuildIDByCharInvite(ctx, realmID, params.CharGUID)
	if err != nil {
		return 0, err
	}

	if guildID == 0 {
		return 0, errors.New("character doesn't have invites")
	}

	guild, err := g.guildsRepo.GuildByRealmAndID(ctx, realmID, guildID)
	if err != nil {
		return 0, fmt.Errorf("can't fetch guild by id, err: %w", err)
	}

	if guild == nil {
		return 0, ErrGuildNotFound
	}

	if !params.AllowCrossFaction && !guildLeaderSameFaction(guild, params.CharRace) {
		return 0, ErrGuildNotAllied
	}

	err = g.guildsRepo.RemoveGuildInviteForCharacter(ctx, realmID, params.CharGUID)
	if err != nil {
		return 0, err
	}

	err = g.guildsRepo.AddGuildMember(ctx, realmID, repo.GuildMember{
		GuildID:     guildID,
		PlayerGUID:  params.CharGUID,
		Rank:        g.lowestRankInGuild(guild),
		PublicNote:  "",
		OfficerNote: "",
		Name:        params.CharName,
		Race:        params.CharRace,
		Class:       params.CharClass,
		Lvl:         params.CharLvl,
		Gender:      params.CharGender,
		AreaID:      params.CharAreaID,
		Account:     params.CharAccount,
		LogoutTime:  0,
		Status:      repo.GuildMemberStatusOnline,
	})
	if err != nil {
		return 0, err
	}

	guild, err = g.guildsRepo.GuildByRealmAndID(ctx, realmID, guildID)
	if err != nil {
		return 0, fmt.Errorf("can't fetch guild by id after update, err: %w", err)
	}

	err = g.eventsProducer.MemberAdded(&events.GuildEventMemberAddedPayload{
		GenericGuildEvent: *g.buildGenericEventPayload(guild),
		MemberGUID:        params.CharGUID,
		MemberName:        params.CharName,
	})
	if err != nil {
		return 0, err
	}

	return guildID, nil
}

// Leave handles player leave command.
func (g *guildServiceImpl) Leave(ctx context.Context, realmID uint32, charGUID uint64) error {
	guild, err := g.guildByMemberGUID(ctx, realmID, charGUID)
	if err != nil {
		return err
	}

	if guild == nil {
		return ErrGuildNotFound
	}

	rank := g.rankForMember(guild, charGUID)
	if rank.Rank == uint8(repo.GuildRankGuildMaster) {
		return ErrLeaderCantLeave
	}

	member := g.guildMemberForMemberGuid(guild, charGUID)

	genericEventPayload := g.buildGenericEventPayload(guild)

	err = g.guildsRepo.RemoveGuildMember(ctx, realmID, charGUID)
	if err != nil {
		return err
	}

	err = g.eventsProducer.MemberLeft(&events.GuildEventMemberLeftPayload{
		GenericGuildEvent: *genericEventPayload,
		MemberGUID:        member.PlayerGUID,
		MemberName:        member.Name,
	})
	if err != nil {
		return err
	}

	return nil
}

// Kick handles kick of the guild member.
func (g *guildServiceImpl) Kick(ctx context.Context, realmID uint32, kicker, target uint64) error {
	guild, err := g.guildByMemberGUID(ctx, realmID, kicker)
	if err != nil {
		return err
	}

	if guild == nil {
		return ErrGuildNotFound
	}

	targetsGuildID, err := g.guildsRepo.GuildIDByRealmAndMemberGUID(ctx, realmID, target)
	if err != nil {
		return err
	}

	if guild.ID != targetsGuildID {
		return ErrNotEnoughRight
	}

	rankOfKicker := g.rankForMember(guild, kicker)
	rankOfTarget := g.rankForMember(guild, target)
	kickerMember := g.guildMemberForMemberGuid(guild, kicker)
	targetMember := g.guildMemberForMemberGuid(guild, target)

	if !rankOfKicker.HasRight(repo.RightRemove) {
		return ErrNotEnoughRight
	}

	if rankOfKicker.Rank >= rankOfTarget.Rank {
		return ErrNotEnoughRight
	}

	genericEventPayload := g.buildGenericEventPayload(guild)

	err = g.guildsRepo.RemoveGuildMember(ctx, realmID, target)
	if err != nil {
		return err
	}

	err = g.eventsProducer.MemberKicked(&events.GuildEventMemberKickedPayload{
		GenericGuildEvent: *genericEventPayload,
		MemberGUID:        targetMember.PlayerGUID,
		MemberName:        targetMember.Name,
		KickerGUID:        kickerMember.PlayerGUID,
		KickerName:        kickerMember.Name,
	})
	if err != nil {
		return err
	}

	return nil
}

// SetMessageOfTheDay sets message of the day for a guild.
func (g *guildServiceImpl) SetMessageOfTheDay(ctx context.Context, realmID uint32, updaterGUID uint64, message string) error {
	guild, err := g.guildByMemberGUID(ctx, realmID, updaterGUID)
	if err != nil {
		return err
	}

	if guild == nil {
		return ErrGuildNotFound
	}

	rank := g.rankForMember(guild, updaterGUID)
	if !rank.HasRight(repo.RightSetMessageOfTheDay) {
		return ErrNotEnoughRight
	}

	err = g.guildsRepo.SetMessageOfTheDay(ctx, realmID, guild.ID, message)
	if err != nil {
		return err
	}

	err = g.eventsProducer.MOTDUpdated(&events.GuildEventMOTDUpdatedPayload{
		GenericGuildEvent:  *g.buildGenericEventPayload(guild),
		NewMessageOfTheDay: message,
	})
	if err != nil {
		return err
	}

	return nil
}

// SetGuildInfo sets info text for the guild.
func (g *guildServiceImpl) SetGuildInfo(ctx context.Context, realmID uint32, updaterGUID uint64, info string) error {
	guild, err := g.guildByMemberGUID(ctx, realmID, updaterGUID)
	if err != nil {
		return err
	}

	if guild == nil {
		return ErrGuildNotFound
	}

	rank := g.rankForMember(guild, updaterGUID)
	if !rank.HasRight(repo.RightModifyGuildInfo) {
		return ErrNotEnoughRight
	}

	err = g.guildsRepo.SetGuildInfo(ctx, realmID, guild.ID, info)
	if err != nil {
		return err
	}

	err = g.eventsProducer.GuildInfoUpdated(&events.GuildEventGuildInfoUpdatedPayload{
		GenericGuildEvent: *g.buildGenericEventPayload(guild),
		NewGuildInfo:      info,
	})
	if err != nil {
		return err
	}

	return nil
}

// SetMemberPublicNote sets public note for the guild member.
func (g *guildServiceImpl) SetMemberPublicNote(ctx context.Context, realmID uint32, updaterGUID, targetGUID uint64, note string) error {
	updaterGuildID, err := g.guildsRepo.GuildIDByRealmAndMemberGUID(ctx, realmID, updaterGUID)
	if err != nil {
		return err
	}

	if updaterGuildID == 0 {
		return ErrGuildNotFound
	}

	targetGuildID, err := g.guildsRepo.GuildIDByRealmAndMemberGUID(ctx, realmID, targetGUID)
	if err != nil {
		return err
	}

	if targetGuildID == 0 {
		return ErrGuildNotFound
	}

	if targetGuildID != updaterGuildID {
		return ErrNotEnoughRight
	}

	guild, err := g.guildsRepo.GuildByRealmAndID(ctx, realmID, updaterGuildID)
	if err != nil {
		return err
	}

	rank := g.rankForMember(guild, updaterGUID)
	if !rank.HasRight(repo.RightEditPublicNote) {
		return ErrNotEnoughRight
	}

	err = g.guildsRepo.SetMemberPublicNote(ctx, realmID, targetGUID, note)
	if err != nil {
		return err
	}

	target := g.guildMemberForMemberGuid(guild, targetGUID)
	updater := g.guildMemberForMemberGuid(guild, updaterGUID)

	err = g.eventsProducer.MemberNoteUpdated(&events.GuildEventMembersNoteUpdatedPayload{
		GenericGuildEvent: *g.buildGenericEventPayload(guild),
		MemberGUID:        targetGUID,
		MemberName:        target.Name,
		UpdaterGUID:       updaterGUID,
		UpdaterName:       updater.Name,
		Note:              note,
	})
	if err != nil {
		return err
	}

	return nil
}

// SetMemberOfficerNote sets officer note for the guild member.
func (g *guildServiceImpl) SetMemberOfficerNote(ctx context.Context, realmID uint32, updaterGUID, targetGUID uint64, note string) error {
	updaterGuildID, err := g.guildsRepo.GuildIDByRealmAndMemberGUID(ctx, realmID, updaterGUID)
	if err != nil {
		return err
	}

	if updaterGuildID == 0 {
		return ErrGuildNotFound
	}

	targetGuildID, err := g.guildsRepo.GuildIDByRealmAndMemberGUID(ctx, realmID, targetGUID)
	if err != nil {
		return err
	}

	if targetGuildID == 0 {
		return ErrGuildNotFound
	}

	if targetGuildID != updaterGuildID {
		return ErrNotEnoughRight
	}

	guild, err := g.guildsRepo.GuildByRealmAndID(ctx, realmID, updaterGuildID)
	if err != nil {
		return err
	}

	rank := g.rankForMember(guild, updaterGUID)
	if !rank.HasRight(repo.RightEditOfficersNote) {
		return ErrNotEnoughRight
	}

	err = g.guildsRepo.SetMemberOfficerNote(ctx, realmID, targetGUID, note)
	if err != nil {
		return err
	}

	target := g.guildMemberForMemberGuid(guild, targetGUID)
	updater := g.guildMemberForMemberGuid(guild, updaterGUID)

	err = g.eventsProducer.MemberOfficerNoteUpdated(&events.GuildEventMembersOfficerNoteUpdatedPayload{
		GenericGuildEvent: *g.buildGenericEventPayload(guild),
		MemberGUID:        targetGUID,
		MemberName:        target.Name,
		UpdaterGUID:       updaterGUID,
		UpdaterName:       updater.Name,
		Note:              note,
	})
	if err != nil {
		return err
	}

	return nil
}

// UpdateGuildRank updates rank of the guild.
func (g *guildServiceImpl) UpdateGuildRank(ctx context.Context, realmID uint32, updaterGUID uint64, rank GuildRank) error {
	guild, err := g.guildByMemberGUID(ctx, realmID, updaterGUID)
	if err != nil {
		return err
	}

	if guild == nil {
		return ErrGuildNotFound
	}

	memberRank := g.rankForMember(guild, updaterGUID)
	if memberRank == nil {
		return ErrGuildNotFound
	}
	if memberRank.Rank != uint8(repo.GuildRankGuildMaster) {
		return ErrNotEnoughRight
	}

	err = g.guildsRepo.UpdateGuildRank(ctx, realmID, guild.ID, rank.RankID, rank.Name, rank.Rights, rank.MoneyPerDay, rank.BankTabRights)
	if err != nil {
		return err
	}

	err = g.eventsProducer.RankUpdated(&events.GuildEventRankUpdatedPayload{
		GenericGuildEvent: *g.buildGenericEventPayload(guild),
		RankID:            rank.RankID,
		RankName:          rank.Name,
		RankRights:        rank.Rights,
		RankMoneyPerDay:   rank.MoneyPerDay,
		RanksCount:        uint8(len(guild.GuildRanks)),
	})
	if err != nil {
		return err
	}

	return nil
}

// AddGuildRank creates new guild rank in the updaters gild with given name.
func (g *guildServiceImpl) AddGuildRank(ctx context.Context, realmID uint32, updaterGUID uint64, rankName string) error {
	guild, err := g.guildByMemberGUID(ctx, realmID, updaterGUID)
	if err != nil {
		return err
	}

	if guild == nil {
		return ErrGuildNotFound
	}

	memberRank := g.rankForMember(guild, updaterGUID)
	if memberRank == nil {
		return ErrGuildNotFound
	}
	if memberRank.Rank != uint8(repo.GuildRankGuildMaster) {
		return ErrNotEnoughRight
	}

	if len(guild.GuildRanks) >= repo.GuildRankMax {
		return fmt.Errorf("rank limit rached (%d)", repo.GuildRankMax)
	}

	rankID := g.lowestRankInGuild(guild) + 1
	ranksCount := uint8(len(guild.GuildRanks) + 1)
	err = g.guildsRepo.AddGuildRank(ctx, realmID, guild.ID, rankID, rankName, repo.RightChatListen|repo.RightChatSpeak, 0)
	if err != nil {
		return err
	}

	err = g.eventsProducer.RankCreated(&events.GuildEventRankCreatedPayload{
		GenericGuildEvent: *g.buildGenericEventPayload(guild),
		RankID:            rankID,
		RankName:          rankName,
		RanksCount:        ranksCount,
	})
	if err != nil {
		return err
	}

	return nil
}

// DeleteLastGuildRank deletes the last guild rank from updaters guild.
func (g *guildServiceImpl) DeleteLastGuildRank(ctx context.Context, realmID uint32, updaterGUID uint64) error {
	guild, err := g.guildByMemberGUID(ctx, realmID, updaterGUID)
	if err != nil {
		return err
	}

	if guild == nil {
		return ErrGuildNotFound
	}

	memberRank := g.rankForMember(guild, updaterGUID)
	if memberRank == nil {
		return ErrGuildNotFound
	}
	if memberRank.Rank != uint8(repo.GuildRankGuildMaster) {
		return ErrNotEnoughRight
	}
	if len(guild.GuildRanks) <= repo.GuildRankMinCount {
		return nil
	}

	lowestRank := g.lowestRankInGuild(guild)
	var rankToDelete *repo.GuildRank
	for _, rank := range guild.GuildRanks {
		if rank.Rank == lowestRank {
			rankCopy := rank
			rankToDelete = &rankCopy
			break
		}
	}
	if rankToDelete == nil {
		return ErrGuildNotFound
	}
	ranksCount := uint8(len(guild.GuildRanks) - 1)

	err = g.guildsRepo.DeleteLowestGuildRank(ctx, realmID, guild.ID, lowestRank)
	if err != nil {
		return err
	}

	err = g.eventsProducer.RankDeleted(&events.GuildEventRankDeletedPayload{
		GenericGuildEvent: *g.buildGenericEventPayload(guild),
		RankID:            lowestRank,
		RankName:          rankToDelete.Name,
		RanksCount:        ranksCount,
	})
	if err != nil {
		return err
	}

	return nil
}

// PromoteMember promotes guild member to the 1 rank higher.
func (g *guildServiceImpl) PromoteMember(ctx context.Context, realmID uint32, updaterGUID, targetGUID uint64) error {
	return g.updateRank(ctx, realmID, updaterGUID, targetGUID, true)
}

// DemoteMember demotes guild member to the 1 lower higher.
func (g *guildServiceImpl) DemoteMember(ctx context.Context, realmID uint32, updaterGUID, targetGUID uint64) error {
	return g.updateRank(ctx, realmID, updaterGUID, targetGUID, false)
}

// SendGuildMessage sends guild message to the online player.
func (g *guildServiceImpl) SendGuildMessage(ctx context.Context, realmID uint32, senderGUID uint64, message string, lang uint32, isOfficers bool, senderChatTag uint8) error {
	guild, err := g.guildByMemberGUID(ctx, realmID, senderGUID)
	if err != nil {
		return err
	}

	if guild == nil {
		return ErrGuildNotFound
	}

	sender := g.guildMemberForMemberGuid(guild, senderGUID)
	if sender == nil {
		return ErrGuildNotFound
	}
	senderRank := g.rankForMember(guild, senderGUID)
	if senderRank == nil {
		return ErrGuildNotFound
	}
	if isOfficers {
		if !senderRank.HasRight(repo.RightOfficerChatSpeak) {
			return ErrNotEnoughRight
		}
	} else if !senderRank.HasRight(repo.RightChatSpeak) {
		return ErrNotEnoughRight
	}

	var receivers []uint64
	for _, member := range guild.GuildMembers {
		if member.Status != repo.GuildMemberStatusOnline || member.PlayerGUID == senderGUID {
			continue
		}
		memberRank := g.rankWithID(guild, member.Rank)
		if memberRank == nil {
			continue
		}
		if isOfficers {
			if memberRank.HasRight(repo.RightOfficerChatListen) {
				receivers = append(receivers, member.PlayerGUID)
			}
			continue
		}
		if memberRank.HasRight(repo.RightChatListen) {
			receivers = append(receivers, member.PlayerGUID)
		}
	}
	ignoredReceivers, err := g.guildsRepo.IgnoredByGuildMembers(ctx, realmID, senderGUID, receivers)
	if err != nil {
		return err
	}
	filteredReceivers := receivers[:0]
	for _, receiverGUID := range receivers {
		if !ignoredReceivers[receiverGUID] {
			filteredReceivers = append(filteredReceivers, receiverGUID)
		}
	}
	receivers = filteredReceivers

	return g.eventsProducer.NewMessage(&events.GuildEventNewMessagePayload{
		ServiceID:     guildserver.ServiceID,
		RealmID:       realmID,
		GuildID:       guild.ID,
		SenderGUID:    senderGUID,
		SenderName:    sender.Name,
		SenderChatTag: senderChatTag,
		Language:      lang,
		Msg:           message,
		ForOfficers:   isOfficers,
		Receivers:     receivers,
	})
}

// GuildPetitionByGUID returns a native AzerothCore guild petition by item GUID.
func (g *guildServiceImpl) GuildPetitionByGUID(ctx context.Context, realmID uint32, petitionGUID uint64) (*repo.GuildPetition, error) {
	petition, err := g.guildsRepo.GuildPetitionByGUID(ctx, realmID, petitionGUID)
	if err != nil {
		return nil, err
	}
	if petition == nil || petition.Type != guildPetitionType {
		return nil, nil
	}

	return petition, nil
}

// OfferGuildPetition routes a native guild petition offer to an online same-realm target.
func (g *guildServiceImpl) OfferGuildPetition(ctx context.Context, realmID uint32, ownerGUID, targetGUID uint64, targetName string, petitionGUID uint64) (GuildPetitionOfferStatus, error) {
	petition, err := g.GuildPetitionByGUID(ctx, realmID, petitionGUID)
	if err != nil {
		return GuildPetitionOfferFailed, err
	}
	if petition == nil {
		return GuildPetitionOfferNotFound, nil
	}
	if petition.OwnerGUID != ownerGUID {
		return GuildPetitionOfferNotOwner, nil
	}

	targetGuildID, err := g.guildsRepo.GuildIDByRealmAndMemberGUID(ctx, realmID, targetGUID)
	if err != nil {
		return GuildPetitionOfferFailed, err
	}
	if targetGuildID != 0 {
		return GuildPetitionOfferTargetAlreadyInGuild, nil
	}

	targetInviteID, err := g.guildsRepo.GuildIDByCharInvite(ctx, realmID, targetGUID)
	if err != nil {
		return GuildPetitionOfferFailed, err
	}
	if targetInviteID != 0 {
		return GuildPetitionOfferTargetAlreadyInvited, nil
	}

	err = g.eventsProducer.PetitionOffered(&events.GuildEventPetitionOfferedPayload{
		ServiceID:     guildserver.ServiceID,
		RealmID:       realmID,
		PetitionGUID:  petition.PetitionGUID,
		PetitionID:    petition.PetitionID,
		OwnerGUID:     petition.OwnerGUID,
		TargetGUID:    targetGUID,
		TargetName:    targetName,
		GuildName:     petition.Name,
		RequiredSigns: uint32(petition.Type),
		Signatures:    petitionSignaturesToEvents(petition.Signatures),
	})
	if err != nil {
		return GuildPetitionOfferFailed, err
	}

	return GuildPetitionOfferOK, nil
}

// SignGuildPetition validates and persists a native guild petition signature.
func (g *guildServiceImpl) SignGuildPetition(ctx context.Context, realmID uint32, signerGUID uint64, signerName string, signerAccountID uint32, signerGuildID uint32, petitionGUID uint64) (GuildPetitionSignStatus, error) {
	petition, err := g.GuildPetitionByGUID(ctx, realmID, petitionGUID)
	if err != nil {
		return GuildPetitionSignFailed, err
	}
	if petition == nil {
		return GuildPetitionSignNotFound, nil
	}
	if petition.OwnerGUID == signerGUID {
		return GuildPetitionSignCantSignOwn, nil
	}

	if signerGuildID != 0 {
		return GuildPetitionSignAlreadyInGuild, nil
	}
	signerCurrentGuildID, err := g.guildsRepo.GuildIDByRealmAndMemberGUID(ctx, realmID, signerGUID)
	if err != nil {
		return GuildPetitionSignFailed, err
	}
	if signerCurrentGuildID != 0 {
		return GuildPetitionSignAlreadyInGuild, nil
	}

	signerInviteID, err := g.guildsRepo.GuildIDByCharInvite(ctx, realmID, signerGUID)
	if err != nil {
		return GuildPetitionSignFailed, err
	}
	if signerInviteID != 0 {
		return GuildPetitionSignAlreadyInvited, nil
	}

	for _, signature := range petition.Signatures {
		if signature.PlayerGUID == signerGUID || signature.PlayerAccount == signerAccountID {
			return g.publishGuildPetitionSigned(realmID, petition, signerGUID, signerName, GuildPetitionSignAlreadySigned)
		}
	}

	if len(petition.Signatures)+1 > int(petition.Type) {
		return GuildPetitionSignFull, nil
	}

	err = g.guildsRepo.AddGuildPetitionSignature(ctx, realmID, petition.PetitionID, petition.PetitionGUID, petition.OwnerGUID, signerGUID, signerAccountID)
	if err != nil {
		return GuildPetitionSignFailed, err
	}

	return g.publishGuildPetitionSigned(realmID, petition, signerGUID, signerName, GuildPetitionSignOK)
}

func (g *guildServiceImpl) publishGuildPetitionSigned(realmID uint32, petition *repo.GuildPetition, signerGUID uint64, signerName string, status GuildPetitionSignStatus) (GuildPetitionSignStatus, error) {
	err := g.eventsProducer.PetitionSigned(&events.GuildEventPetitionSignedPayload{
		ServiceID:     guildserver.ServiceID,
		RealmID:       realmID,
		PetitionGUID:  petition.PetitionGUID,
		OwnerGUID:     petition.OwnerGUID,
		SignerGUID:    signerGUID,
		SignerName:    signerName,
		NativeStatus:  uint32(status),
		RequiredSigns: uint32(petition.Type),
	})
	if err != nil {
		return GuildPetitionSignFailed, err
	}

	return status, nil
}

func petitionSignaturesToEvents(signatures []repo.GuildPetitionSignature) []events.GuildPetitionSignature {
	result := make([]events.GuildPetitionSignature, len(signatures))
	for i := range signatures {
		result[i] = events.GuildPetitionSignature{
			PlayerGUID:    signatures[i].PlayerGUID,
			PlayerAccount: signatures[i].PlayerAccount,
		}
	}
	return result
}

// updateRank handles promote and demote actions.
func (g *guildServiceImpl) updateRank(ctx context.Context, realmID uint32, updaterGUID, targetGUID uint64, promote bool) error {
	guild, err := g.guildByMemberGUID(ctx, realmID, updaterGUID)
	if err != nil {
		return err
	}

	if guild == nil {
		return ErrGuildNotFound
	}

	targetGuildGUID, err := g.guildsRepo.GuildIDByRealmAndMemberGUID(ctx, realmID, targetGUID)
	if err != nil {
		return err
	}

	if targetGuildGUID == 0 {
		return ErrGuildNotFound
	}

	if targetGuildGUID != guild.ID {
		return ErrNotEnoughRight
	}

	rank := g.rankForMember(guild, updaterGUID)
	if rank == nil {
		return ErrGuildNotFound
	}
	if promote && !rank.HasRight(repo.RightPromote) {
		return ErrNotEnoughRight
	} else if !promote && !rank.HasRight(repo.RightDemote) {
		return ErrNotEnoughRight
	}

	targetRank := g.rankForMember(guild, targetGUID)
	if targetRank == nil {
		return ErrGuildNotFound
	}
	var newRank uint8
	if promote {
		if targetRank.Rank == uint8(repo.GuildRankGuildMaster) {
			return ErrNotEnoughRight
		}
		newRank = targetRank.Rank - 1

		if rank.Rank >= newRank {
			return ErrNotEnoughRight
		}
	} else {
		newRank = targetRank.Rank + 1
		if rank.Rank >= targetRank.Rank {
			return ErrNotEnoughRight
		}

		if newRank > g.lowestRankInGuild(guild) {
			return errors.New("already lowest rank")
		}
	}

	newRankObj := g.rankWithID(guild, newRank)
	updater := g.guildMemberForMemberGuid(guild, updaterGUID)
	target := g.guildMemberForMemberGuid(guild, targetGUID)
	if newRankObj == nil || updater == nil || target == nil {
		return ErrGuildNotFound
	}

	err = g.guildsRepo.SetMemberRank(ctx, realmID, targetGUID, newRank)
	if err != nil {
		return err
	}

	if promote {
		err = g.eventsProducer.MemberPromote(&events.GuildEventMemberPromotePayload{
			GenericGuildEvent: *g.buildGenericEventPayload(guild),
			RankID:            newRankObj.Rank,
			RankName:          newRankObj.Name,
			PromoterGUID:      updater.PlayerGUID,
			PromoterName:      updater.Name,
			MemberGUID:        target.PlayerGUID,
			MemberName:        target.Name,
		})
	} else {
		err = g.eventsProducer.MemberDemote(&events.GuildEventMemberDemotePayload{
			GenericGuildEvent: *g.buildGenericEventPayload(guild),
			RankID:            newRankObj.Rank,
			RankName:          newRankObj.Name,
			DemoterGUID:       updater.PlayerGUID,
			DemoterName:       updater.Name,
			MemberGUID:        target.PlayerGUID,
			MemberName:        target.Name,
		})
	}
	if err != nil {
		return err
	}

	return nil
}

// HandleGuildCreated refreshes newly created guild state after gateway observes worldserver success.
func (g *guildServiceImpl) HandleGuildCreated(payload events.GWEventGuildCreatedPayload) error {
	refresher, ok := g.guildsRepo.(interface {
		RefreshGuildByMemberGUID(ctx context.Context, realmID uint32, memberGUID uint64) (*repo.Guild, error)
	})
	if !ok {
		return fmt.Errorf("guild repo does not support refresh")
	}

	guild, err := refresher.RefreshGuildByMemberGUID(context.Background(), payload.RealmID, payload.LeaderGUID)
	if err != nil {
		return err
	}
	if guild == nil {
		return ErrGuildNotFound
	}

	genericPayload := *g.buildGenericEventPayload(guild)
	for _, member := range guild.GuildMembers {
		err = g.eventsProducer.MemberAdded(&events.GuildEventMemberAddedPayload{
			GenericGuildEvent: genericPayload,
			MemberGUID:        member.PlayerGUID,
			MemberName:        member.Name,
		})
		if err != nil {
			return err
		}
	}

	return nil
}

// rankForMember returns rank in guild with given member guid.
func (g *guildServiceImpl) rankForMember(guild *repo.Guild, memberGUID uint64) *repo.GuildRank {
	member := g.guildMemberForMemberGuid(guild, memberGUID)
	if member == nil {
		return nil
	}

	for _, rank := range guild.GuildRanks {
		if rank.Rank == member.Rank {
			return &rank
		}
	}
	return nil
}

// rankWithID returns rank in guild with given id.
func (g *guildServiceImpl) rankWithID(guild *repo.Guild, id uint8) *repo.GuildRank {
	for _, rank := range guild.GuildRanks {
		if rank.Rank == id {
			return &rank
		}
	}
	return nil
}

// guildMemberForMemberGuid returns guildMember by member guid.
func (g *guildServiceImpl) guildMemberForMemberGuid(guild *repo.Guild, memberGUID uint64) *repo.GuildMember {
	for _, member := range guild.GuildMembers {
		if member.PlayerGUID == memberGUID {
			return member
		}
	}
	return nil
}

func guildLeaderSameFaction(guild *repo.Guild, race uint8) bool {
	if guild == nil {
		return false
	}

	for _, member := range guild.GuildMembers {
		if member.PlayerGUID == guild.LeaderGUID {
			return guildSameFaction(member.Race, race)
		}
	}

	return false
}

func guildSameFaction(leftRace, rightRace uint8) bool {
	leftTeam, ok := guildRaceTeam(leftRace)
	if !ok {
		return false
	}
	rightTeam, ok := guildRaceTeam(rightRace)
	if !ok {
		return false
	}
	return leftTeam == rightTeam
}

func guildRaceTeam(race uint8) (wow.Team, bool) {
	if int(race) >= len(wow.DefaultRaces) {
		return 0, false
	}
	raceInfo := wow.DefaultRaces[race]
	if raceInfo.ID == 0 {
		return 0, false
	}
	return raceInfo.Team, true
}

func (g *guildServiceImpl) guildBankMemberRank(ctx context.Context, realmID uint32, guildID, memberGUID uint64) (*repo.Guild, *repo.GuildMember, *repo.GuildRank, GuildBankStatus, error) {
	guild, err := g.bankRepo().GuildByRealmAndID(ctx, realmID, guildID)
	if err != nil {
		return nil, nil, nil, GuildBankStatusFailed, err
	}
	if guild == nil {
		return nil, nil, nil, GuildBankStatusGuildNotFound, nil
	}

	member := g.guildMemberForMemberGuid(guild, memberGUID)
	if member == nil {
		return guild, nil, nil, GuildBankStatusNotInGuild, nil
	}
	rank := g.rankForMember(guild, memberGUID)
	if rank == nil {
		return guild, member, nil, GuildBankStatusNotEnoughRights, nil
	}

	return guild, member, rank, GuildBankStatusOK, nil
}

func (g *guildServiceImpl) guildBankCanViewTab(guild *repo.Guild, rank *repo.GuildRank, tabID uint8) GuildBankStatus {
	if tabID >= repo.GuildBankMaxTabs || tabID >= guild.PurchasedBankTabs {
		return GuildBankStatusInvalidTab
	}
	right := g.guildBankTabRight(rank, tabID)
	if right == nil || right.Flags&repo.GuildBankRightViewTab == 0 {
		return GuildBankStatusNotEnoughRights
	}
	return GuildBankStatusOK
}

func (g *guildServiceImpl) guildBankCanDeposit(guild *repo.Guild, rank *repo.GuildRank, tabID uint8) GuildBankStatus {
	if tabID >= repo.GuildBankMaxTabs || tabID >= guild.PurchasedBankTabs {
		return GuildBankStatusInvalidTab
	}
	right := g.guildBankTabRight(rank, tabID)
	if right == nil || right.Flags&repo.GuildBankRightDepositItem != repo.GuildBankRightDepositItem {
		return GuildBankStatusNotEnoughRights
	}
	return GuildBankStatusOK
}

func (g *guildServiceImpl) guildBankCanWithdraw(guild *repo.Guild, member *repo.GuildMember, rank *repo.GuildRank, tabID uint8) GuildBankStatus {
	if status := g.guildBankCanViewTab(guild, rank, tabID); status != GuildBankStatusOK {
		return status
	}
	if g.guildBankRemainingSlots(rank, member, tabID) == 0 {
		return GuildBankStatusWithdrawLimit
	}
	return GuildBankStatusOK
}

func (g *guildServiceImpl) guildBankCanUpdateText(guild *repo.Guild, rank *repo.GuildRank, tabID uint8) GuildBankStatus {
	if tabID >= repo.GuildBankMaxTabs || tabID >= guild.PurchasedBankTabs {
		return GuildBankStatusInvalidTab
	}
	right := g.guildBankTabRight(rank, tabID)
	if right == nil || right.Flags&repo.GuildBankRightUpdateText == 0 {
		return GuildBankStatusNotEnoughRights
	}
	return GuildBankStatusOK
}

func (g *guildServiceImpl) guildBankTabRight(rank *repo.GuildRank, tabID uint8) *repo.GuildBankTabRight {
	if rank == nil || tabID >= repo.GuildBankMaxTabs {
		return nil
	}
	right := rank.BankTabRights[tabID]
	return &right
}

func (g *guildServiceImpl) guildBankRemainingSlots(rank *repo.GuildRank, member *repo.GuildMember, tabID uint8) int32 {
	if rank == nil || member == nil || tabID >= repo.GuildBankMaxTabs {
		return 0
	}
	right := rank.BankTabRights[tabID]
	return guildBankRemainingLimit(right.WithdrawItemLimit, member.BankWithdraw[tabID])
}

func (g *guildServiceImpl) guildBankRemainingMoney(rank *repo.GuildRank, member *repo.GuildMember) int32 {
	if rank == nil || member == nil {
		return 0
	}
	return guildBankRemainingLimit(rank.MoneyPerDay, member.BankWithdraw[repo.GuildBankMaxTabs])
}

func guildBankRemainingLimit(limit uint32, used uint32) int32 {
	if limit == 0xFFFFFFFF {
		return -1
	}
	if used >= limit {
		return 0
	}
	return int32(limit - used)
}

func repoGuildBankErrorToStatus(err error) GuildBankStatus {
	switch {
	case err == nil:
		return GuildBankStatusOK
	case errors.Is(err, repo.ErrGuildBankInvalidTab):
		return GuildBankStatusInvalidTab
	case errors.Is(err, repo.ErrGuildBankInvalidSlot):
		return GuildBankStatusInvalidSlot
	case errors.Is(err, repo.ErrGuildBankNotEnoughGold):
		return GuildBankStatusNotEnoughMoney
	case errors.Is(err, repo.ErrGuildBankFull):
		return GuildBankStatusBankFull
	case errors.Is(err, repo.ErrGuildBankWithdrawLimit):
		return GuildBankStatusWithdrawLimit
	case errors.Is(err, repo.ErrGuildBankItemNotFound):
		return GuildBankStatusItemNotFound
	default:
		return GuildBankStatusFailed
	}
}

// lowestRankInGuild returns the lowest rank in given guild. Ranks are inverted, so we are interested in a rank with the highest id.
func (g *guildServiceImpl) lowestRankInGuild(guild *repo.Guild) uint8 {
	highestRank := uint8(0)
	for _, rank := range guild.GuildRanks {
		if rank.Rank > highestRank {
			highestRank = rank.Rank
		}
	}
	return highestRank
}

// guildByMemberGUID returns guild by member guid.
func (g *guildServiceImpl) guildByMemberGUID(ctx context.Context, realmID uint32, memberGUID uint64) (*repo.Guild, error) {
	guildID, err := g.guildsRepo.GuildIDByRealmAndMemberGUID(ctx, realmID, memberGUID)
	if err != nil {
		return nil, err
	}

	if guildID == 0 {
		return nil, nil
	}

	guild, err := g.guildsRepo.GuildByRealmAndID(ctx, realmID, guildID)
	if err != nil {
		return nil, err
	}

	return guild, nil
}

// buildGenericEventPayload builds generic payload for guild event.
func (g *guildServiceImpl) buildGenericEventPayload(guild *repo.Guild) *events.GenericGuildEvent {
	var membersOnline []uint64
	for _, member := range guild.GuildMembers {
		if member.Status == repo.GuildMemberStatusOnline {
			membersOnline = append(membersOnline, member.PlayerGUID)
		}
	}
	return &events.GenericGuildEvent{
		ServiceID:     guildserver.ServiceID,
		RealmID:       guild.RealmID,
		GuildID:       guild.ID,
		GuildName:     guild.Name,
		MembersOnline: membersOnline,
	}
}
