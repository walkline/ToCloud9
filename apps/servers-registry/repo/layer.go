package repo

import "context"

// LayerStore contains the small amount of shared state needed for layering.
// Implementations must make BindGroup atomic across registry replicas.
type LayerStore interface {
	Configuration(context.Context, uint32) (map[uint32]uint32, error)
	SetConfiguration(context.Context, uint32, map[uint32]uint32) error
	GroupBinding(context.Context, uint32, uint32, uint32) (string, error)
	BindGroup(context.Context, uint32, uint32, uint32, string) (string, error)
	SetGroupBinding(context.Context, uint32, uint32, uint32, string) error
	ReplaceGroupBinding(context.Context, uint32, uint32, uint32, string, string) (string, error)
	LockRealm(context.Context, uint32) (func(), error)
}
