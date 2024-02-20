package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/walkline/ToCloud9/apps/guildserver"
	"github.com/walkline/ToCloud9/apps/guildserver/repo"
	"github.com/walkline/ToCloud9/shared/events"
)

var (
	ErrNotEnoughRight  = errors.New("not enough rights")
	ErrGuildNotFound   = errors.New("guild not found")
	ErrLeaderCantLeave = errors.New("leader can't leave")
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
}

// GuildRank represents guild rank.
type GuildRank struct {
	RankID      uint8
	Name        string
	Rights      uint32
	MoneyPerDay uint32
}

// GuildService is service to handle guilds.
type GuildService interface {
	// GuildByRealmAndID returns guild by realmID and guildID.
	GuildByRealmAndID(ctx context.Context, realmID uint32, guildID uint64) (*repo.Guild, error)

	// InviteMember creates invite to the guild.
	InviteMember(ctx context.Context, realmID uint32, inviterGUID uint64, inviteeGUID uint64, inviteeName string) error

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
	SendGuildMessage(ctx context.Context, realmID uint32, senderGUID uint64, message string, lang uint32, isOfficers bool) error
}

// guildServiceImpl is implementation of GuildService.
type guildServiceImpl struct {
	guildsRepo     repo.GuildsRepo
	eventsProducer events.GuildServiceProducer
}

// NewGuildService creates GuildService.
func NewGuildService(guildsRepo repo.GuildsRepo, eventsProducer events.GuildServiceProducer) GuildService {
	return &guildServiceImpl{
		guildsRepo:     guildsRepo,
		eventsProducer: eventsProducer,
	}
}

// GuildByRealmAndID returns guild by realmID and guildID.
func (g *guildServiceImpl) GuildByRealmAndID(ctx context.Context, realmID uint32, guildID uint64) (*repo.Guild, error) {
	return g.guildsRepo.GuildByRealmAndID(ctx, realmID, guildID)
}

// InviteMember creates invite to the guild.
func (g *guildServiceImpl) InviteMember(ctx context.Context, realmID uint32, inviterGUID uint64, inviteeGUID uint64, inviteeName string) error {
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

	member := g.guildMemberForMemberGuid(guild, inviterGUID)

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

	rank := g.rankForMember(guild, updaterGuildID)
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
	if memberRank.Rank != uint8(repo.GuildRankGuildMaster) {
		return ErrNotEnoughRight
	}

	err = g.guildsRepo.UpdateGuildRank(ctx, realmID, guild.ID, rank.RankID, rank.Name, rank.Rights, rank.MoneyPerDay)
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
	if memberRank.Rank != uint8(repo.GuildRankGuildMaster) {
		return ErrNotEnoughRight
	}

	if len(guild.GuildRanks) >= repo.GuildRankMax {
		return fmt.Errorf("rank limit rached (%d)", repo.GuildRankMax)
	}

	rankID := g.lowestRankInGuild(guild) + 1
	err = g.guildsRepo.AddGuildRank(ctx, realmID, guild.ID, rankID, rankName, repo.RightEmpty, 0)
	if err != nil {
		return err
	}

	err = g.eventsProducer.RankCreated(&events.GuildEventRankCreatedPayload{
		GenericGuildEvent: *g.buildGenericEventPayload(guild),
		RankID:            rankID,
		RankName:          rankName,
		RanksCount:        uint8(len(guild.GuildRanks)),
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
	if memberRank.Rank != uint8(repo.GuildRankGuildMaster) {
		return ErrNotEnoughRight
	}

	lowestRank := g.lowestRankInGuild(guild)
	var rankToDelete repo.GuildRank
	for _, rank := range guild.GuildRanks {
		if rank.Rank == lowestRank {
			rankToDelete = rank
			break
		}
	}

	err = g.guildsRepo.DeleteLowestGuildRank(ctx, realmID, guild.ID, lowestRank)
	if err != nil {
		return err
	}

	err = g.eventsProducer.RankDeleted(&events.GuildEventRankDeletedPayload{
		GenericGuildEvent: *g.buildGenericEventPayload(guild),
		RankID:            memberRank.Rank,
		RankName:          rankToDelete.Name,
		RanksCount:        uint8(len(guild.GuildRanks)),
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
func (g *guildServiceImpl) SendGuildMessage(ctx context.Context, realmID uint32, senderGUID uint64, message string, lang uint32, isOfficers bool) error {
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

	var receivers []uint64
	for _, member := range guild.GuildMembers {
		if member.Status == repo.GuildMemberStatusOnline && member.PlayerGUID != senderGUID {
			receivers = append(receivers, member.PlayerGUID)
		}
	}

	return g.eventsProducer.NewMessage(&events.GuildEventNewMessagePayload{
		ServiceID:   guildserver.ServiceID,
		RealmID:     realmID,
		GuildID:     guild.ID,
		SenderGUID:  senderGUID,
		SenderName:  sender.Name,
		Language:    lang,
		Msg:         message,
		ForOfficers: isOfficers,
		Receivers:   receivers,
	})
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
	if promote && !rank.HasRight(repo.RightPromote) {
		return ErrNotEnoughRight
	} else if !promote && !rank.HasRight(repo.RightDemote) {
		return ErrNotEnoughRight
	}

	targetRank := g.rankForMember(guild, targetGUID)
	var newRank uint8
	if promote {
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
