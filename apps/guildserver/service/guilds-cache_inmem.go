package service

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/walkline/ToCloud9/apps/guildserver/repo"
	"github.com/walkline/ToCloud9/shared/events"
)

// guildSoftHealInterval is an optional slow rehydrate for out-of-band MySQL
// mutations (GM tools). Peer multi-replica coherence uses dirty flags instead.
const guildSoftHealInterval = 30 * time.Second

// guildsInMemCache is in memory implementation of GuildsCache.
type guildsInMemCache struct {
	r repo.GuildsRepo

	// localServiceID filters peer invalidation self-echo (set via SetLocalServiceID).
	localServiceID string

	// cacheMutex guards cache, guildMembersCache, onlineChars, lastRefresh, dirty, refreshFlight.
	cacheMutex sync.RWMutex

	// cache usage example:
	//		guild := cache[realmID][guildID]
	cache map[uint32]map[uint64]*repo.Guild

	// guildMembersCache usage example:
	//		member := guildMembersCache[realmID][characterID]
	guildMembersCache map[uint32]map[uint64]*repo.GuildMember

	// onlineChars tracks logged-in characters independently of guild membership.
	// The world doesn't flush characters.online in cluster mode and characters can
	// join a guild without going through this service, so the roster status has to
	// come from the gateway login/logout events, including for future members.
	onlineChars map[uint32]map[uint64]struct{}

	// lastRefresh tracks the last *successful* re-hydration time per guild.
	lastRefresh map[uint32]map[uint64]time.Time

	// dirty marks guilds that peers invalidated; next read reloads from MySQL
	// (no eager load on the NATS callback).
	dirty map[uint32]map[uint64]struct{}

	// refreshFlight coalesces concurrent reloads for the same guild (singleflight).
	// Waiters share one MySQL load and its success/error — force callers never
	// false-succeed while a concurrent load is still in flight.
	refreshFlight map[uint32]map[uint64]*guildRefreshFlight
}

type guildRefreshFlight struct {
	done chan struct{} // closed when load finished
	err  error
}

// NewGuildsInMemCache returns in memory guilds cache.
func NewGuildsInMemCache(r repo.GuildsRepo) GuildsCache {
	return &guildsInMemCache{
		r:                 r,
		cache:             map[uint32]map[uint64]*repo.Guild{},
		guildMembersCache: map[uint32]map[uint64]*repo.GuildMember{},
		onlineChars:       map[uint32]map[uint64]struct{}{},
		lastRefresh:       map[uint32]map[uint64]time.Time{},
		dirty:             map[uint32]map[uint64]struct{}{},
		refreshFlight:     map[uint32]map[uint64]*guildRefreshFlight{},
	}
}

// SetLocalServiceID sets the instance id used to ignore self-published invalidations.
func (g *guildsInMemCache) SetLocalServiceID(id string) {
	g.localServiceID = id
}

// LoadAllForRealm loads all guilds for realm.
// Can be time-consuming, better to use it on startup to warmup cache.
func (g *guildsInMemCache) LoadAllForRealm(ctx context.Context, realmID uint32) (map[uint64]*repo.Guild, error) {
	return g.r.LoadAllForRealm(ctx, realmID)
}

// GuildByRealmAndID returns a guild from cache, reloading only when missing,
// peer-dirty, or due for a slow soft-heal of out-of-band DB edits.
func (g *guildsInMemCache) GuildByRealmAndID(ctx context.Context, realmID uint32, guildID uint64) (*repo.Guild, error) {
	g.cacheMutex.RLock()
	guild := g.cache[realmID][guildID]
	_, isDirty := g.dirty[realmID][guildID]
	last := g.lastRefresh[realmID][guildID]
	g.cacheMutex.RUnlock()

	needLoad := guild == nil || isDirty
	softHeal := !needLoad && !last.IsZero() && time.Since(last) > guildSoftHealInterval
	if needLoad || softHeal {
		// Dirty/missing always force; soft-heal uses the same flight but is not
		// required for multi-replica peer coherence.
		_ = g.refreshGuildFromSource(ctx, realmID, guildID, needLoad)
	}

	g.cacheMutex.RLock()
	guild = g.cache[realmID][guildID]
	g.cacheMutex.RUnlock()
	return guild, nil
}

// MemberAuthzForGuild delegates to the repo (uncached point query for write authz).
func (g *guildsInMemCache) MemberAuthzForGuild(ctx context.Context, realmID uint32, guildID, playerGUID uint64) (*repo.MemberAuthz, error) {
	return g.r.MemberAuthzForGuild(ctx, realmID, guildID, playerGUID)
}

// ForceRefreshGuild re-hydrates one guild from MySQL, ignoring soft-heal timing.
func (g *guildsInMemCache) ForceRefreshGuild(ctx context.Context, realmID uint32, guildID uint64) error {
	return g.refreshGuildFromSource(ctx, realmID, guildID, true)
}

// HandleGuildCacheInvalidate marks a guild dirty so the next read reloads from
// MySQL. No eager ForceRefresh — that was the multi-replica thrash source.
// Self-echo (same ServiceID) is ignored — the publisher already write-through updated.
func (g *guildsInMemCache) HandleGuildCacheInvalidate(payload events.GuildCacheInvalidatePayload) error {
	if payload.ServiceID != "" && g.localServiceID != "" && payload.ServiceID == g.localServiceID {
		return nil
	}
	if payload.GuildID == 0 {
		return nil
	}
	g.markDirty(payload.RealmID, payload.GuildID)
	return nil
}

func (g *guildsInMemCache) markDirty(realmID uint32, guildID uint64) {
	g.cacheMutex.Lock()
	defer g.cacheMutex.Unlock()
	if g.dirty[realmID] == nil {
		g.dirty[realmID] = map[uint64]struct{}{}
	}
	g.dirty[realmID][guildID] = struct{}{}
}

// refreshGuildFromSource re-hydrates the cached guild from the repo.
// force=true always loads; force=false is unused for peer path (soft-heal uses force=false
// only when caller already decided soft-heal is due — still singleflight).
// lastRefresh is only advanced after a successful load. Concurrent reloads share one flight.
func (g *guildsInMemCache) refreshGuildFromSource(ctx context.Context, realmID uint32, guildID uint64, force bool) error {
	g.cacheMutex.Lock()
	if !force {
		// Soft-heal only: skip if another load just completed within the interval.
		if time.Since(g.lastRefresh[realmID][guildID]) < guildSoftHealInterval {
			if _, dirty := g.dirty[realmID][guildID]; !dirty {
				g.cacheMutex.Unlock()
				return nil
			}
		}
	}
	if g.refreshFlight[realmID] == nil {
		g.refreshFlight[realmID] = map[uint64]*guildRefreshFlight{}
	}
	if f := g.refreshFlight[realmID][guildID]; f != nil {
		g.cacheMutex.Unlock()
		select {
		case <-f.done:
			return f.err
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	flight := &guildRefreshFlight{done: make(chan struct{})}
	g.refreshFlight[realmID][guildID] = flight
	g.cacheMutex.Unlock()

	guild, err := g.r.GuildByRealmAndID(ctx, realmID, guildID)

	g.cacheMutex.Lock()
	if err == nil {
		if old := g.cache[realmID][guildID]; old != nil {
			for _, member := range old.GuildMembers {
				cached := g.guildMembersCache[realmID][member.PlayerGUID]
				if cached != nil && cached.GuildID == guildID {
					delete(g.guildMembersCache[realmID], member.PlayerGUID)
				}
			}
		}
		if g.lastRefresh[realmID] == nil {
			g.lastRefresh[realmID] = map[uint64]time.Time{}
		}
		g.lastRefresh[realmID][guildID] = time.Now()
		if g.dirty[realmID] != nil {
			delete(g.dirty[realmID], guildID)
		}

		if guild == nil {
			if g.cache[realmID] != nil {
				delete(g.cache[realmID], guildID)
			}
		} else {
			if g.cache[realmID] == nil {
				g.cache[realmID] = map[uint64]*repo.Guild{}
			}
			if g.guildMembersCache[realmID] == nil {
				g.guildMembersCache[realmID] = map[uint64]*repo.GuildMember{}
			}
			for _, member := range guild.GuildMembers {
				if _, online := g.onlineChars[realmID][member.PlayerGUID]; online {
					member.Status = repo.GuildMemberStatusOnline
					member.LogoutTime = 0
				}
				g.guildMembersCache[realmID][member.PlayerGUID] = member
			}
			g.cache[realmID][guildID] = guild
		}
	}
	flight.err = err
	delete(g.refreshFlight[realmID], guildID)
	close(flight.done)
	g.cacheMutex.Unlock()
	return err
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

// GuildIDByRealmAndMemberGUIDFromSource returns guild id by guild member guid from
// the underlying repo. The world can remove guilds and members without going through
// this service (e.g. GM commands handled in-process), so a positive cached membership
// can be stale — when the source disagrees, the cached entry is evicted.
func (g *guildsInMemCache) GuildIDByRealmAndMemberGUIDFromSource(ctx context.Context, realmID uint32, memberGUID uint64) (uint64, error) {
	guildID, err := g.r.GuildIDByRealmAndMemberGUID(ctx, realmID, memberGUID)
	if err != nil {
		return 0, err
	}

	if guildID == 0 {
		g.evictGuildMember(realmID, memberGUID)
	}

	return guildID, nil
}

// evictGuildMember removes the member from the members cache and from the cached guild roster.
func (g *guildsInMemCache) evictGuildMember(realmID uint32, characterGUID uint64) {
	g.cacheMutex.Lock()
	defer g.cacheMutex.Unlock()

	member := g.guildMembersCache[realmID][characterGUID]
	if member == nil {
		return
	}

	delete(g.guildMembersCache[realmID], characterGUID)

	guild := g.cache[realmID][member.GuildID]
	if guild == nil {
		return
	}

	for i, mem := range guild.GuildMembers {
		if mem.PlayerGUID == characterGUID {
			guild.GuildMembers = append(guild.GuildMembers[:i], guild.GuildMembers[i+1:]...)
			break
		}
	}
}

// AcceptGuildInvite consumes invite + inserts member in one MySQL TX, then refreshes cache.
func (g *guildsInMemCache) AcceptGuildInvite(ctx context.Context, realmID uint32, member repo.GuildMember) (uint64, error) {
	guildID, err := g.r.AcceptGuildInvite(ctx, realmID, member)
	if err != nil {
		return 0, err
	}
	// Hydrate roster (and membership index) from SoT after the atomic accept.
	if err := g.ForceRefreshGuild(ctx, realmID, guildID); err != nil {
		log.Warn().Err(err).Uint32("realmID", realmID).Uint64("guildID", guildID).
			Msg("AcceptGuildInvite: cache refresh after TX failed")
	}
	// Ensure the new member is online in the overlay for event fan-out.
	g.cacheMutex.Lock()
	if g.onlineChars[realmID] == nil {
		g.onlineChars[realmID] = map[uint64]struct{}{}
	}
	g.onlineChars[realmID][member.PlayerGUID] = struct{}{}
	if m := g.guildMembersCache[realmID][member.PlayerGUID]; m != nil {
		m.Status = repo.GuildMemberStatusOnline
		m.LogoutTime = 0
	}
	// Also index by low guid if the DB stores counters only.
	low := uint64(uint32(member.PlayerGUID))
	if low != member.PlayerGUID {
		g.onlineChars[realmID][low] = struct{}{}
		if m := g.guildMembersCache[realmID][low]; m != nil {
			m.Status = repo.GuildMemberStatusOnline
			m.LogoutTime = 0
		}
	}
	g.cacheMutex.Unlock()
	return guildID, nil
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
	// Peer-created guilds (or cold cache) may not have the guild entry yet.
	// Always index membership so local auth works; rehydrate guild best-effort.
	memberCopy := member
	g.guildMembersCache[realmID][member.PlayerGUID] = &memberCopy
	guild := g.cache[realmID][member.GuildID]
	if guild == nil {
		g.cacheMutex.Unlock()
		if err := g.ForceRefreshGuild(ctx, realmID, member.GuildID); err != nil {
			// MySQL member row already exists; do not fail the mutator so peers
			// still receive cache invalidation from the coherence decorator.
			log.Warn().Err(err).Uint32("realmID", realmID).Uint64("guildID", member.GuildID).
				Msg("AddGuildMember: force refresh after insert failed")
		}
		return nil
	}
	guild.GuildMembers = append(guild.GuildMembers, &memberCopy)
	g.cacheMutex.Unlock()

	return nil
}

// RemoveGuildMember removes guild member from the guild.
func (g *guildsInMemCache) RemoveGuildMember(ctx context.Context, realmID uint32, characterGUID uint64) error {
	if err := g.r.RemoveGuildMember(ctx, realmID, characterGUID); err != nil {
		return err
	}

	g.evictGuildMember(realmID, characterGUID)

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

// SetGuildEmblem updates the guild tabard emblem and the in-memory cache.
func (g *guildsInMemCache) SetGuildEmblem(ctx context.Context, realmID uint32, guildID uint64, emblem repo.GuildEmblem) error {
	err := g.r.SetGuildEmblem(ctx, realmID, guildID, emblem)
	if err != nil {
		return err
	}

	g.cacheMutex.Lock()
	defer g.cacheMutex.Unlock()

	guild := g.cache[realmID][guildID]
	if guild == nil {
		return nil
	}

	guild.Emblem = emblem
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
	guilds, err := g.r.LoadAllForRealm(ctx, realmID)
	if err != nil {
		return err
	}

	g.cacheMutex.Lock()
	defer g.cacheMutex.Unlock()

	g.cache[realmID] = guilds
	g.guildMembersCache[realmID] = map[uint64]*repo.GuildMember{}
	if g.lastRefresh[realmID] == nil {
		g.lastRefresh[realmID] = map[uint64]time.Time{}
	}
	now := time.Now()
	for _, guild := range guilds {
		g.lastRefresh[realmID][guild.ID] = now
		for i := range guild.GuildMembers {
			g.guildMembersCache[realmID][guild.GuildMembers[i].PlayerGUID] = guild.GuildMembers[i]
		}
	}

	return nil
}

// SeedOnlineChars marks the given characters as online, overlaying the already
// cached rosters. Meant for startup: login events observed before this process
// started are gone, so the online state is recovered from the characters service.
func (g *guildsInMemCache) SeedOnlineChars(realmID uint32, charGUIDs []uint64) {
	g.cacheMutex.Lock()
	defer g.cacheMutex.Unlock()

	if g.onlineChars[realmID] == nil {
		g.onlineChars[realmID] = map[uint64]struct{}{}
	}
	for _, guid := range charGUIDs {
		g.onlineChars[realmID][guid] = struct{}{}
		if member := g.guildMembersCache[realmID][guid]; member != nil {
			member.Status = repo.GuildMemberStatusOnline
			member.LogoutTime = 0
		}
	}
}

// HandleCharacterLoggedIn updates cache with player logged in.
// The character is tracked even when it isn't a guild member yet, so a later
// roster refresh can mark it online if it joined a guild in-process.
func (g *guildsInMemCache) HandleCharacterLoggedIn(payload events.GWEventCharacterLoggedInPayload) error {
	g.cacheMutex.Lock()
	if g.onlineChars[payload.RealmID] == nil {
		g.onlineChars[payload.RealmID] = map[uint64]struct{}{}
	}
	g.onlineChars[payload.RealmID][payload.CharGUID] = struct{}{}
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
	delete(g.onlineChars[payload.RealmID], payload.CharGUID)
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

// CreateGuild creates the guild in the underlying repo and caches it hydrated
// (the reload joins member details from the characters table).
func (g *guildsInMemCache) CreateGuild(ctx context.Context, realmID uint32, name string, leaderGUID uint64, ranks []repo.GuildRank, memberGUIDs []uint64) (uint64, error) {
	id, err := g.r.CreateGuild(ctx, realmID, name, leaderGUID, ranks, memberGUIDs)
	if err != nil {
		return 0, err
	}

	guild, err := g.r.GuildByRealmAndID(ctx, realmID, id)
	if err != nil {
		return id, fmt.Errorf("guild %d created but not cached, err: %w", id, err)
	}

	g.cacheMutex.Lock()
	if g.cache[realmID] == nil {
		g.cache[realmID] = map[uint64]*repo.Guild{}
	}
	if g.guildMembersCache[realmID] == nil {
		g.guildMembersCache[realmID] = map[uint64]*repo.GuildMember{}
	}
	g.cache[realmID][id] = guild
	if g.onlineChars[realmID] == nil {
		g.onlineChars[realmID] = map[uint64]struct{}{}
	}
	for _, member := range guild.GuildMembers {
		// Founding members just joined (leader turn-in + live signatories).
		// characters.online is often stale in cluster, so hydration would mark
		// them offline and MembersOnline / join events would miss them.
		member.Status = repo.GuildMemberStatusOnline
		g.onlineChars[realmID][member.PlayerGUID] = struct{}{}
		g.guildMembersCache[realmID][member.PlayerGUID] = member
	}
	if g.lastRefresh[realmID] == nil {
		g.lastRefresh[realmID] = map[uint64]time.Time{}
	}
	g.lastRefresh[realmID][id] = time.Now()
	if g.dirty[realmID] != nil {
		delete(g.dirty[realmID], id)
	}
	g.cacheMutex.Unlock()

	return id, nil
}
