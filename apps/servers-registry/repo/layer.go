package repo

import "context"

// LayerStore contains the small amount of shared state needed for layering.
// Implementations must make BindGroup atomic across registry replicas.
type LayerStore interface {
	Configuration(ctx context.Context, realmID uint32) (map[uint32]uint32, error)
	SetConfiguration(ctx context.Context, realmID uint32, layersByMapID map[uint32]uint32) error
	GroupBinding(ctx context.Context, realmID, groupID, mapID uint32) (gameServerID string, err error)
	BindGroup(ctx context.Context, realmID, groupID, mapID uint32, gameServerID string) (boundGameServerID string, err error)
	SetGroupBinding(ctx context.Context, realmID, groupID, mapID uint32, gameServerID string) error
	ReplaceGroupBinding(ctx context.Context, realmID, groupID, mapID uint32, previousGameServerID, replacementGameServerID string) (boundGameServerID string, err error)
	LockRealm(ctx context.Context, realmID uint32) (unlock func(), err error)
}
