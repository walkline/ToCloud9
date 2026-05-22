package service

import (
	"context"
	"sync"
	"time"

	"github.com/walkline/ToCloud9/apps/guildserver/repo"
	"github.com/walkline/ToCloud9/shared/events"
)

// guildsInMemCache is in memory implementation of GuildsCache.
type guildsInMemCache struct {
	r repo.GuildsRepo

	// cacheMutex guards cache and guildMembersCache maps.
	cacheMutex sync.RWMutex

	// cache usage example:
	//		guild := cache[realmID][guildID]
	cache map[uint32]map[uint64]*repo.Guild

	// guildMembersCache usage example:
	//		member := guildMembersCache[realmID][characterID]
	guildMembersCache map[uint32]map[uint64]*repo.GuildMember

	lifecycleEventTimes map[uint32]map[uint64]uint64
}

// NewGuildsInMemCache returns in memory guilds cache.
func NewGuildsInMemCache(r repo.GuildsRepo) GuildsCache {
	return &guildsInMemCache{
		r:                   r,
		cache:               map[uint32]map[uint64]*repo.Guild{},
		guildMembersCache:   map[uint32]map[uint64]*repo.GuildMember{},
		lifecycleEventTimes: map[uint32]map[uint64]uint64{},
	}
}

// LoadAllForRealm loads all guilds for realm.
// Can be time-consuming, better to use it on startup to warmup cache.
func (g *guildsInMemCache) LoadAllForRealm(ctx context.Context, realmID uint32) (map[uint64]*repo.Guild, error) {
	return g.r.LoadAllForRealm(ctx, realmID)
}

// GuildByRealmAndID loads guild by realm and id.
func (g *guildsInMemCache) GuildByRealmAndID(_ context.Context, realmID uint32, guildID uint64) (*repo.Guild, error) {
	g.cacheMutex.RLock()
	guild := g.cache[realmID][guildID]
	g.cacheMutex.RUnlock()
	return guild, nil
}

// AddGuildInvite links user invite to a specific guild. Uncached.
func (g *guildsInMemCache) AddGuildInvite(ctx context.Context, realmID uint32, charGUID, guildID uint64) error {
	return g.r.AddGuildInvite(ctx, realmID, charGUID, guildID)
}

// GuildIDByCharInvite returns guild id by invited character. Uncached.
func (g *guildsInMemCache) GuildIDByCharInvite(ctx context.Context, realmID uint32, charGUID uint64) (uint64, error) {
	return g.r.GuildIDByCharInvite(ctx, realmID, charGUID)
}

// RemoveGuildInviteForCharacter removes guild invite by character.
func (g *guildsInMemCache) RemoveGuildInviteForCharacter(ctx context.Context, realmID uint32, charGUID uint64) error {
	return g.r.RemoveGuildInviteForCharacter(ctx, realmID, charGUID)
}

// GuildIDByRealmAndMemberGUID returns guild id by guild member guid.
func (g *guildsInMemCache) GuildIDByRealmAndMemberGUID(_ context.Context, realmID uint32, memberGUID uint64) (uint64, error) {
	g.cacheMutex.RLock()
	member := g.guildMembersCache[realmID][memberGUID]
	g.cacheMutex.RUnlock()
	if member == nil {
		return 0, nil
	}

	return member.GuildID, nil
}

func (g *guildsInMemCache) IgnoredByGuildMembers(ctx context.Context, realmID uint32, senderGUID uint64, receiverGUIDs []uint64) (map[uint64]bool, error) {
	return g.r.IgnoredByGuildMembers(ctx, realmID, senderGUID, receiverGUIDs)
}

// AddGuildMember adds guild member to the guild.
func (g *guildsInMemCache) AddGuildMember(ctx context.Context, realmID uint32, member repo.GuildMember) error {
	if err := g.r.AddGuildMember(ctx, realmID, member); err != nil {
		return err
	}

	g.cacheMutex.Lock()
	if g.guildMembersCache[realmID] == nil {
		g.guildMembersCache[realmID] = map[uint64]*repo.GuildMember{}
	}
	if g.cache[realmID] == nil {
		g.cache[realmID] = map[uint64]*repo.Guild{}
	}
	g.guildMembersCache[realmID][member.PlayerGUID] = &member
	if g.cache[realmID][member.GuildID] != nil {
		g.cache[realmID][member.GuildID].GuildMembers = append(g.cache[realmID][member.GuildID].GuildMembers, &member)
	}
	g.cacheMutex.Unlock()

	return nil
}

// RemoveGuildMember removes guild member from the guild.
func (g *guildsInMemCache) RemoveGuildMember(ctx context.Context, realmID uint32, characterGUID uint64) error {
	if err := g.r.RemoveGuildMember(ctx, realmID, characterGUID); err != nil {
		return err
	}

	g.cacheMutex.Lock()
	defer g.cacheMutex.Unlock()

	member := g.guildMembersCache[realmID][characterGUID]
	if member == nil {
		return nil
	}

	delete(g.guildMembersCache[realmID], characterGUID)

	for i, mem := range g.cache[realmID][member.GuildID].GuildMembers {
		if mem.PlayerGUID == characterGUID {
			g.cache[realmID][member.GuildID].GuildMembers = append(
				g.cache[realmID][member.GuildID].GuildMembers[:i],
				g.cache[realmID][member.GuildID].GuildMembers[i+1:]...,
			)
			break
		}
	}

	return nil
}

// SetMessageOfTheDay updates message of the day for the guild.
func (g *guildsInMemCache) SetMessageOfTheDay(ctx context.Context, realmID uint32, guildID uint64, message string) error {
	err := g.r.SetMessageOfTheDay(ctx, realmID, guildID, message)
	if err != nil {
		return err
	}

	g.cacheMutex.Lock()
	defer g.cacheMutex.Unlock()

	guild := g.cache[realmID][guildID]
	if guild == nil {
		return nil
	}

	guild.MessageOfTheDay = message

	return nil
}

// SetMemberPublicNote sets public not for guild member.
func (g *guildsInMemCache) SetMemberPublicNote(ctx context.Context, realmID uint32, memberGUID uint64, note string) error {
	err := g.r.SetMemberPublicNote(ctx, realmID, memberGUID, note)
	if err != nil {
		return err
	}

	g.cacheMutex.Lock()
	defer g.cacheMutex.Unlock()

	member := g.guildMembersCache[realmID][memberGUID]
	if member == nil {
		return nil
	}

	member.PublicNote = note

	return nil
}

// SetMemberOfficerNote sets officer not for guild member.
func (g *guildsInMemCache) SetMemberOfficerNote(ctx context.Context, realmID uint32, memberGUID uint64, note string) error {
	err := g.r.SetMemberOfficerNote(ctx, realmID, memberGUID, note)
	if err != nil {
		return err
	}

	g.cacheMutex.Lock()
	defer g.cacheMutex.Unlock()

	member := g.guildMembersCache[realmID][memberGUID]
	if member == nil {
		return nil
	}

	member.OfficerNote = note

	return nil
}

// SetMemberRank sets rank for the guild member.
func (g *guildsInMemCache) SetMemberRank(ctx context.Context, realmID uint32, memberGUID uint64, rank uint8) error {
	err := g.r.SetMemberRank(ctx, realmID, memberGUID, rank)
	if err != nil {
		return err
	}

	g.cacheMutex.Lock()
	defer g.cacheMutex.Unlock()

	member := g.guildMembersCache[realmID][memberGUID]
	if member == nil {
		return nil
	}

	member.Rank = rank

	return nil
}

// SetGuildInfo updates guild info text of the guild.
func (g *guildsInMemCache) SetGuildInfo(ctx context.Context, realmID uint32, guildID uint64, info string) error {
	err := g.r.SetGuildInfo(ctx, realmID, guildID, info)
	if err != nil {
		return err
	}

	g.cacheMutex.Lock()
	defer g.cacheMutex.Unlock()

	guild := g.cache[realmID][guildID]
	if guild == nil {
		return nil
	}

	guild.Info = info
	return nil
}

// UpdateGuildRank updates guild rank.
func (g *guildsInMemCache) UpdateGuildRank(
	ctx context.Context, realmID uint32, guildID uint64,
	rank uint8, name string, rights, moneyPerDay uint32, bankTabRights [repo.GuildBankMaxTabs]repo.GuildBankTabRight,
) error {
	err := g.r.UpdateGuildRank(ctx, realmID, guildID, rank, name, rights, moneyPerDay, bankTabRights)
	if err != nil {
		return err
	}

	g.cacheMutex.Lock()
	defer g.cacheMutex.Unlock()

	guild := g.cache[realmID][guildID]
	if guild == nil {
		return nil
	}

	for i, guildRank := range guild.GuildRanks {
		if guildRank.Rank == rank {
			guild.GuildRanks[i] = repo.GuildRank{
				GuildID:       guildID,
				Rank:          rank,
				Name:          name,
				Rights:        rights,
				MoneyPerDay:   moneyPerDay,
				BankTabRights: bankTabRights,
			}
			break
		}
	}

	return nil
}

// AddGuildRank adds guild rank.
func (g *guildsInMemCache) AddGuildRank(ctx context.Context, realmID uint32, guildID uint64, rank uint8, name string, rights, moneyPerDay uint32) error {
	err := g.r.AddGuildRank(ctx, realmID, guildID, rank, name, rights, moneyPerDay)
	if err != nil {
		return err
	}

	g.cacheMutex.Lock()
	defer g.cacheMutex.Unlock()

	guild := g.cache[realmID][guildID]
	if guild == nil {
		return nil
	}

	guild.GuildRanks = append(guild.GuildRanks, repo.GuildRank{
		GuildID:     guildID,
		Rank:        rank,
		Name:        name,
		Rights:      rights,
		MoneyPerDay: moneyPerDay,
	})

	return nil
}

// DeleteLowestGuildRank deletes lowes guild rank.
func (g *guildsInMemCache) DeleteLowestGuildRank(ctx context.Context, realmID uint32, guildID uint64, rank uint8) error {
	err := g.r.DeleteLowestGuildRank(ctx, realmID, guildID, rank)
	if err != nil {
		return err
	}

	g.cacheMutex.Lock()
	defer g.cacheMutex.Unlock()

	guild := g.cache[realmID][guildID]
	if guild == nil {
		return nil
	}

	if int(rank) > len(guild.GuildRanks) {
		return nil
	}

	guild.GuildRanks = guild.GuildRanks[:rank]

	return nil
}

// GuildPetitionByGUID loads a native petition by item GUID. Uncached.
func (g *guildsInMemCache) GuildPetitionByGUID(ctx context.Context, realmID uint32, petitionGUID uint64) (*repo.GuildPetition, error) {
	return g.r.GuildPetitionByGUID(ctx, realmID, petitionGUID)
}

// AddGuildPetitionSignature persists a native guild petition signature. Uncached.
func (g *guildsInMemCache) AddGuildPetitionSignature(ctx context.Context, realmID uint32, petitionID uint32, petitionGUID, ownerGUID, playerGUID uint64, playerAccount uint32) error {
	return g.r.AddGuildPetitionSignature(ctx, realmID, petitionID, petitionGUID, ownerGUID, playerGUID, playerAccount)
}

func (g *guildsInMemCache) GuildBank(ctx context.Context, realmID uint32, guildID uint64, tabID uint8, fullUpdate bool) (*repo.GuildBank, error) {
	return g.r.GuildBank(ctx, realmID, guildID, tabID, fullUpdate)
}

func (g *guildsInMemCache) GuildBankLog(ctx context.Context, realmID uint32, guildID uint64, tabID uint8) ([]repo.GuildBankLogEntry, error) {
	return g.r.GuildBankLog(ctx, realmID, guildID, tabID)
}

func (g *guildsInMemCache) SetGuildBankTabInfo(ctx context.Context, realmID uint32, guildID uint64, tabID uint8, name, icon string) error {
	return g.r.SetGuildBankTabInfo(ctx, realmID, guildID, tabID, name, icon)
}

func (g *guildsInMemCache) SetGuildBankTabText(ctx context.Context, realmID uint32, guildID uint64, tabID uint8, text string) error {
	return g.r.SetGuildBankTabText(ctx, realmID, guildID, tabID, text)
}

func (g *guildsInMemCache) BuyGuildBankTab(ctx context.Context, realmID uint32, guildID uint64, tabID uint8) error {
	return g.r.BuyGuildBankTab(ctx, realmID, guildID, tabID)
}

func (g *guildsInMemCache) DepositGuildBankMoney(ctx context.Context, realmID uint32, guildID uint64, memberGUID uint64, amount uint32) error {
	return g.r.DepositGuildBankMoney(ctx, realmID, guildID, memberGUID, amount)
}

func (g *guildsInMemCache) WithdrawGuildBankMoney(ctx context.Context, realmID uint32, guildID uint64, memberGUID uint64, amount uint32, repair bool) (uint32, error) {
	return g.r.WithdrawGuildBankMoney(ctx, realmID, guildID, memberGUID, amount, repair)
}

func (g *guildsInMemCache) RollbackGuildBankMoneyWithdraw(ctx context.Context, realmID uint32, guildID uint64, memberGUID uint64, amount uint32, repair bool, logGUID uint32) error {
	return g.r.RollbackGuildBankMoneyWithdraw(ctx, realmID, guildID, memberGUID, amount, repair, logGUID)
}

func (g *guildsInMemCache) DepositGuildBankItem(ctx context.Context, realmID uint32, guildID uint64, memberGUID uint64, tabID, slotID uint8, item repo.GuildBankItem) error {
	return g.r.DepositGuildBankItem(ctx, realmID, guildID, memberGUID, tabID, slotID, item)
}

func (g *guildsInMemCache) WithdrawGuildBankItem(ctx context.Context, realmID uint32, guildID uint64, memberGUID uint64, tabID, slotID uint8, count uint32, splitItemGUID uint64) (*repo.GuildBankItem, uint32, error) {
	return g.r.WithdrawGuildBankItem(ctx, realmID, guildID, memberGUID, tabID, slotID, count, splitItemGUID)
}

func (g *guildsInMemCache) RollbackGuildBankItemWithdraw(ctx context.Context, realmID uint32, guildID uint64, memberGUID uint64, tabID, slotID uint8, item repo.GuildBankItem, logGUID uint32) ([]uint8, error) {
	return g.r.RollbackGuildBankItemWithdraw(ctx, realmID, guildID, memberGUID, tabID, slotID, item, logGUID)
}

func (g *guildsInMemCache) MoveGuildBankItem(ctx context.Context, realmID uint32, guildID uint64, memberGUID uint64, sourceTabID, sourceSlotID, destinationTabID, destinationSlotID uint8, count uint32, splitItemGUID uint64) ([]uint8, error) {
	return g.r.MoveGuildBankItem(ctx, realmID, guildID, memberGUID, sourceTabID, sourceSlotID, destinationTabID, destinationSlotID, count, splitItemGUID)
}

// Warmup called on startup to warmup cache if possible.
func (g *guildsInMemCache) Warmup(ctx context.Context, realmID uint32) error {
	g.cacheMutex.Lock()
	defer g.cacheMutex.Unlock()

	guilds, err := g.r.LoadAllForRealm(ctx, realmID)
	if err != nil {
		return err
	}

	g.cache[realmID] = guilds

	g.guildMembersCache[realmID] = map[uint64]*repo.GuildMember{}
	for _, guild := range guilds {
		for i := range guild.GuildMembers {
			g.guildMembersCache[realmID][guild.GuildMembers[i].PlayerGUID] = guild.GuildMembers[i]
		}
	}

	return nil
}

// RefreshGuildByMemberGUID reloads one guild from backing storage by member guid.
func (g *guildsInMemCache) RefreshGuildByMemberGUID(ctx context.Context, realmID uint32, memberGUID uint64) (*repo.Guild, error) {
	guildID, err := g.r.GuildIDByRealmAndMemberGUID(ctx, realmID, memberGUID)
	if err != nil {
		return nil, err
	}
	if guildID == 0 {
		return nil, nil
	}

	guild, err := g.r.GuildByRealmAndID(ctx, realmID, guildID)
	if err != nil {
		return nil, err
	}
	if guild == nil {
		return nil, nil
	}

	g.cacheMutex.Lock()
	defer g.cacheMutex.Unlock()

	if g.cache[realmID] == nil {
		g.cache[realmID] = map[uint64]*repo.Guild{}
	}
	if g.guildMembersCache[realmID] == nil {
		g.guildMembersCache[realmID] = map[uint64]*repo.GuildMember{}
	}

	if old := g.cache[realmID][guild.ID]; old != nil {
		for _, member := range old.GuildMembers {
			delete(g.guildMembersCache[realmID], member.PlayerGUID)
		}
	}

	g.cache[realmID][guild.ID] = guild
	for _, member := range guild.GuildMembers {
		g.guildMembersCache[realmID][member.PlayerGUID] = member
	}

	return guild, nil
}

func (g *guildsInMemCache) refreshGuildByID(ctx context.Context, realmID uint32, guildID uint64) {
	guild, err := g.r.GuildByRealmAndID(ctx, realmID, guildID)
	if err != nil || guild == nil {
		return
	}

	g.cacheMutex.Lock()
	defer g.cacheMutex.Unlock()

	if g.cache[realmID] == nil {
		g.cache[realmID] = map[uint64]*repo.Guild{}
	}
	if g.guildMembersCache[realmID] == nil {
		g.guildMembersCache[realmID] = map[uint64]*repo.GuildMember{}
	}

	if old := g.cache[realmID][guildID]; old != nil {
		for _, member := range old.GuildMembers {
			delete(g.guildMembersCache[realmID], member.PlayerGUID)
		}
	}

	g.cache[realmID][guildID] = guild
	for _, member := range guild.GuildMembers {
		g.guildMembersCache[realmID][member.PlayerGUID] = member
	}
}

// HandleCharacterLoggedIn updates cache with player logged in.
func (g *guildsInMemCache) HandleCharacterLoggedIn(payload events.GWEventCharacterLoggedInPayload) error {
	g.cacheMutex.Lock()
	if !g.shouldApplyLifecycleEventLocked(payload.RealmID, payload.CharGUID, payload.EventTimeUnixNano) {
		g.cacheMutex.Unlock()
		return nil
	}
	member := g.guildMembersCache[payload.RealmID][payload.CharGUID]
	if member != nil {
		member.Status = repo.GuildMemberStatusOnline
	}
	g.rememberLifecycleEventTimeLocked(payload.RealmID, payload.CharGUID, payload.EventTimeUnixNano)
	g.cacheMutex.Unlock()
	return nil
}

// HandleCharacterLoggedOut updates cache with player logged out.
func (g *guildsInMemCache) HandleCharacterLoggedOut(payload events.GWEventCharacterLoggedOutPayload) error {
	g.cacheMutex.Lock()
	if !g.shouldApplyLifecycleEventLocked(payload.RealmID, payload.CharGUID, payload.EventTimeUnixNano) {
		g.cacheMutex.Unlock()
		return nil
	}
	member := g.guildMembersCache[payload.RealmID][payload.CharGUID]
	if member != nil {
		member.Status = repo.GuildMemberStatusOffline
		member.LogoutTime = time.Now().Unix()
	}
	g.rememberLifecycleEventTimeLocked(payload.RealmID, payload.CharGUID, payload.EventTimeUnixNano)
	g.cacheMutex.Unlock()
	return nil
}

// HandleCharactersUpdates updates cache with pack of characters updates.
func (g *guildsInMemCache) HandleCharactersUpdates(payload events.GWEventCharactersUpdatesPayload) error {
	g.cacheMutex.Lock()
	for _, update := range payload.Updates {
		member := g.guildMembersCache[payload.RealmID][update.ID]
		if member != nil {
			eventTimeUnixNano := update.EventTimeUnixNano
			if eventTimeUnixNano == 0 {
				eventTimeUnixNano = payload.EventTimeUnixNano
			}
			if eventTimeUnixNano != 0 && g.lifecycleEventTimeLocked(payload.RealmID, update.ID) > eventTimeUnixNano {
				continue
			}
			applyCharUpdate(member, update)
		}
	}
	g.cacheMutex.Unlock()
	return nil
}

func (g *guildsInMemCache) shouldApplyLifecycleEventLocked(realmID uint32, charGUID uint64, eventTimeUnixNano uint64) bool {
	return eventTimeUnixNano == 0 || g.lifecycleEventTimeLocked(realmID, charGUID) <= eventTimeUnixNano
}

func (g *guildsInMemCache) lifecycleEventTimeLocked(realmID uint32, charGUID uint64) uint64 {
	realmEvents := g.lifecycleEventTimes[realmID]
	if realmEvents == nil {
		return 0
	}
	return realmEvents[charGUID]
}

func (g *guildsInMemCache) rememberLifecycleEventTimeLocked(realmID uint32, charGUID uint64, eventTimeUnixNano uint64) {
	if eventTimeUnixNano == 0 {
		return
	}
	if g.lifecycleEventTimes == nil {
		g.lifecycleEventTimes = map[uint32]map[uint64]uint64{}
	}
	if g.lifecycleEventTimes[realmID] == nil {
		g.lifecycleEventTimes[realmID] = map[uint64]uint64{}
	}
	g.lifecycleEventTimes[realmID][charGUID] = eventTimeUnixNano
}

func applyCharUpdate(member *repo.GuildMember, upd *events.CharacterUpdate) {
	if upd.Area != nil {
		member.AreaID = *upd.Area
	}

	if upd.Lvl != nil {
		member.Lvl = *upd.Lvl
	}
}
