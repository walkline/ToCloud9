package service

import (
	"context"

	"github.com/walkline/ToCloud9/apps/charserver/repo"
	"github.com/walkline/ToCloud9/shared/events"
)

type CharacterLoginLockEvents struct{ locks repo.CharacterLoginLocks }

func NewCharacterLoginLockEvents(locks repo.CharacterLoginLocks) *CharacterLoginLockEvents {
	return &CharacterLoginLockEvents{locks: locks}
}

func (h *CharacterLoginLockEvents) HandleCharacterLoggedIn(events.GWEventCharacterLoggedInPayload) error {
	return nil
}

func (h *CharacterLoginLockEvents) HandleCharacterLoggedOut(payload events.GWEventCharacterLoggedOutPayload) error {
	return h.locks.Release(context.Background(), payload.RealmID, payload.AccountID, payload.CharGUID, payload.GatewayID)
}

func (h *CharacterLoginLockEvents) ReleaseGateway(ctx context.Context, realmID uint32, gatewayID string) error {
	return h.locks.ReleaseByGateway(ctx, realmID, gatewayID)
}
