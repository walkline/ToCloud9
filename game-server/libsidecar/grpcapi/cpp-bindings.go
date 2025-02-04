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

type BattlegroundStartRequest struct {
	BattlegroundTypeID uint8
	ArenaType          uint8
	IsRated            bool
	MapID              uint32
	BracketLvl         uint8

	HordePlayerGUIDsToAdd    []uint64
	AlliancePlayerGUIDsToAdd []uint64
}

type BattlegroundStartResponse struct {
	InstanceID       uint64
	InstanceClientID uint64
}

type BattlegroundAddPlayersRequest struct {
	BattlegroundTypeID uint8
	InstanceID         uint64

	HordePlayerGUIDsToAdd    []uint64
	AlliancePlayerGUIDsToAdd []uint64
}

type GetPlayerItemsByGuidsHandler func(player uint64, items []uint64) ([]PlayerItem, error)

type RemoveItemsWithGuidsFromPlayerHandler func(player uint64, items []uint64, assignToPlayer uint64) ([]uint64, error)

type AddExistingItemToPlayerHandler func(player uint64, item *ItemToAdd) error

type GetMoneyForPlayerHandler func(player uint64) (uint32, error)

type ModifyMoneyForPlayerHandler func(player uint64, value int32) (uint32, error)

type CanPlayerInteractWithNPCWithFlagsHandler func(player, npc uint64, flag uint32) (bool, error)

type CanPlayerInteractWithGOWithTypeHandler func(player, goGUID uint64, goType uint8) (bool, error)

type StartBattlegroundHandler func(request BattlegroundStartRequest) (*BattlegroundStartResponse, error)

type AddPlayersToBattlegroundHandler func(request BattlegroundAddPlayersRequest) error

type CanPlayerJoinBattlegroundQueueHandler func(player uint64) error

type CanPlayerTeleportToBattlegroundHandler func(player uint64) error

type CppBindings struct {
	GetPlayerItemsByGuids           GetPlayerItemsByGuidsHandler
	RemoveItemsWithGuidsFromPlayer  RemoveItemsWithGuidsFromPlayerHandler
	AddExistingItemToPlayer         AddExistingItemToPlayerHandler
	GetMoneyForPlayer               GetMoneyForPlayerHandler
	ModifyMoneyForPlayer            ModifyMoneyForPlayerHandler
	CanPlayerInteractWithNPC        CanPlayerInteractWithNPCWithFlagsHandler
	CanPlayerInteractWithGO         CanPlayerInteractWithGOWithTypeHandler
	StartBattleground               StartBattlegroundHandler
	AddPlayersToBattleground        AddPlayersToBattlegroundHandler
	CanPlayerJoinBattlegroundQueue  CanPlayerJoinBattlegroundQueueHandler
	CanPlayerTeleportToBattleground CanPlayerTeleportToBattlegroundHandler
}
