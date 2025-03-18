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
}

// NewGuildsInMemCache returns in memory guilds cache.
func NewGuildsInMemCache(r repo.GuildsRepo) GuildsCache {
	return &guildsInMemCache{
		r:                 r,
		cache:             map[uint32]map[uint64]*repo.Guild{},
		guildMembersCache: map[uint32]map[uint64]*repo.GuildMember{},
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

// AddGuildMember adds guild member to the guild.
func (g *guildsInMemCache) AddGuildMember(ctx context.Context, realmID uint32, member repo.GuildMember) error {
	if err := g.r.AddGuildMember(ctx, realmID, member); err != nil {
		return err
	}

	g.cacheMutex.Lock()
	g.guildMembersCache[realmID][member.PlayerGUID] = &member
	g.cache[realmID][member.GuildID].GuildMembers = append(g.cache[realmID][member.GuildID].GuildMembers, &member)
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
	rank uint8, name string, rights, moneyPerDay uint32,
) error {
	err := g.r.UpdateGuildRank(ctx, realmID, guildID, rank, name, rights, moneyPerDay)
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
				GuildID:     guildID,
				Rank:        rank,
				Name:        name,
				Rights:      rights,
				MoneyPerDay: moneyPerDay,
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

// HandleCharacterLoggedIn updates cache with player logged in.
func (g *guildsInMemCache) HandleCharacterLoggedIn(payload events.GWEventCharacterLoggedInPayload) error {
	g.cacheMutex.Lock()
	member := g.guildMembersCache[payload.RealmID][payload.CharGUID]
	if member != nil {
		member.Status = repo.GuildMemberStatusOnline
	}
	g.cacheMutex.Unlock()
	return nil
}

// HandleCharacterLoggedOut updates cache with player logged out.
func (g *guildsInMemCache) HandleCharacterLoggedOut(payload events.GWEventCharacterLoggedOutPayload) error {
	g.cacheMutex.Lock()
	member := g.guildMembersCache[payload.RealmID][payload.CharGUID]
	if member != nil {
		member.Status = repo.GuildMemberStatusOffline
		member.LogoutTime = time.Now().Unix()
	}
	g.cacheMutex.Unlock()
	return nil
}

// HandleCharactersUpdates updates cache with pack of characters updates.
func (g *guildsInMemCache) HandleCharactersUpdates(payload events.GWEventCharactersUpdatesPayload) error {
	g.cacheMutex.Lock()
	for _, update := range payload.Updates {
		member := g.guildMembersCache[payload.RealmID][update.ID]
		if member != nil {
			applyCharUpdate(member, update)
		}
	}
	g.cacheMutex.Unlock()
	return nil
}

func applyCharUpdate(member *repo.GuildMember, upd *events.CharacterUpdate) {
	if upd.Area != nil {
		member.AreaID = *upd.Area
	}

	if upd.Lvl != nil {
		member.Lvl = *upd.Lvl
	}
}
