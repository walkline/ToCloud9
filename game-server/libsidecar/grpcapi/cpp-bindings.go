package grpcapi

type PlayerItem struct {
	Guid             uint64
	Entry            uint32
	Owner            uint64
	BagSlot          uint8
	Slot             uint8
	IsTradable       bool
	Count            uint32
	Flags            uint16
	Durability       uint32
	RandomPropertyID uint32
	Text             string
}

type ItemToAdd struct {
	Guid             uint64
	Entry            uint32
	Count            uint32
	Flags            uint16
	Durability       uint32
	RandomPropertyID uint32
	Text             string
}

type GetPlayerItemsByGuidsHandler func(player uint64, items []uint64) ([]PlayerItem, error)
type RemoveItemsWithGuidsFromPlayerHandler func(player uint64, items []uint64, assignToPlayer uint64) ([]uint64, error)
type AddExistingItemToPlayerHandler func(player uint64, item *ItemToAdd) error
type GetMoneyForPlayerHandler func(player uint64) (uint32, error)
type ModifyMoneyForPlayerHandler func(player uint64, value int32) (uint32, error)
type CanPlayerInteractWithNPCWithFlagsHandler func(player, npc uint64, flag uint32) (bool, error)
type CanPlayerInteractWithGOWithTypeHandler func(player, goGUID uint64, goType uint8) (bool, error)

type CppBindings struct {
	GetPlayerItemsByGuids          GetPlayerItemsByGuidsHandler
	RemoveItemsWithGuidsFromPlayer RemoveItemsWithGuidsFromPlayerHandler
	AddExistingItemToPlayer        AddExistingItemToPlayerHandler
	GetMoneyForPlayer              GetMoneyForPlayerHandler
	ModifyMoneyForPlayer           ModifyMoneyForPlayerHandler
	CanPlayerInteractWithNPC       CanPlayerInteractWithNPCWithFlagsHandler
	CanPlayerInteractWithGO        CanPlayerInteractWithGOWithTypeHandler
}
