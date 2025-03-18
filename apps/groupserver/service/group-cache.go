package service

import (
	"context"

	"github.com/walkline/ToCloud9/apps/groupserver/repo"
	"github.com/walkline/ToCloud9/shared/events"
)

// GroupsCache is cached proxy of groups repo.
type GroupsCache interface {
	// GroupsRepo Since cache is also a proxy we need to have same interface.
	repo.GroupsRepo

	// GWCharacterLoggedInHandler updates cache with player logged in.
	events.GWCharacterLoggedInHandler
	// GWCharacterLoggedOutHandler updates cache with player logged out.
	events.GWCharacterLoggedOutHandler

	// Warmup called on startup to warmup cache if possible.
	Warmup(ctx context.Context, realmID uint32) error
}
