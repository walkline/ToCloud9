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

	// LBCharacterLoggedInHandler updates cache with player logged in.
	events.LBCharacterLoggedInHandler
	// LBCharacterLoggedOutHandler updates cache with player logged out.
	events.LBCharacterLoggedOutHandler
	// LBCharactersUpdatesHandler updates cache with pack of characters updates.
	events.LBCharactersUpdatesHandler

	// Warmup called on startup to warmup cache if possible.
	Warmup(ctx context.Context, realmID uint32) error
}
