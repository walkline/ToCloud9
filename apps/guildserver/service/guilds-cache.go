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

	// GuildMembershipSource part of the interface since cached membership can be stale.
	GuildMembershipSource
}

// GuildMembershipSource provides guild membership from the source of truth,
// bypassing any caching layer.
type GuildMembershipSource interface {
	// GuildIDByRealmAndMemberGUIDFromSource returns guild id by guild member guid
	// from the underlying storage.
	GuildIDByRealmAndMemberGUIDFromSource(ctx context.Context, realmID uint32, memberGUID uint64) (uint64, error)
}
