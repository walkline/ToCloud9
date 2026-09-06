package service

import (
	"context"

	"github.com/walkline/ToCloud9/apps/guildserver/repo"
	"github.com/walkline/ToCloud9/shared/events"
)

// GuildsCache is cached proxy of guilds repo.
type GuildsCache interface {

	// GuildsRepo Since cache is also a proxy we need to have same interface.
	repo.GuildsRepo

	// GWCharacterLoggedInHandler updates cache with player logged in.
	events.GWCharacterLoggedInHandler
	// GWCharacterLoggedOutHandler updates cache with player logged out.
	events.GWCharacterLoggedOutHandler
	// GWCharactersUpdatesHandler updates cache with pack of characters updates.
	events.GWCharactersUpdatesHandler

	// Warmup called on startup to warmup cache if possible.
	Warmup(ctx context.Context, realmID uint32) error

	// SeedOnlineChars marks the given characters as online, overlaying the
	// already cached rosters. Meant to recover the online state on startup.
	SeedOnlineChars(realmID uint32, charGUIDs []uint64)

	// GuildMembershipSource part of the interface since cached membership can be stale.
	GuildMembershipSource

	// GuildCachePeerSync applies peer-replica invalidations and strong reloads.
	GuildCachePeerSync
}

// GuildMembershipSource provides guild membership from the source of truth,
// bypassing any caching layer.
type GuildMembershipSource interface {
	// GuildIDByRealmAndMemberGUIDFromSource returns guild id by guild member guid
	// from the underlying storage.
	GuildIDByRealmAndMemberGUIDFromSource(ctx context.Context, realmID uint32, memberGUID uint64) (uint64, error)
}

// GuildCachePeerSync is the peer-coherence surface for multi-replica guildserver.
// Kept separate from GuildsRepo so bank/UI producers do not depend on NATS.
type GuildCachePeerSync interface {
	// ForceRefreshGuild re-hydrates one guild from MySQL, bypassing the
	// normal refresh throttle (used after peer invalidation and for bank authz).
	// Returns an error if the underlying load fails (callers may fail closed).
	ForceRefreshGuild(ctx context.Context, realmID uint32, guildID uint64) error
	// HandleGuildCacheInvalidate implements events.GuildCacheInvalidateHandler.
	HandleGuildCacheInvalidate(payload events.GuildCacheInvalidatePayload) error
	// SetLocalServiceID configures self-echo filtering for peer invalidations.
	SetLocalServiceID(id string)
}
