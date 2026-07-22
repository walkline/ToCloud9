package repo

import "context"

type CharacterLoginLocks interface {
	Acquire(ctx context.Context, realmID, accountID uint32, characterGUID uint64, gatewayID string) (bool, error)
	Release(ctx context.Context, realmID, accountID uint32, characterGUID uint64, gatewayID string) error
	ReleaseByGateway(ctx context.Context, realmID uint32, gatewayID string) error
}
