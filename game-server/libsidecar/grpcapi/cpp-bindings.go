package grpcapi

type PlayerItem struct {
	Guid             uint64
	Entry            uint32
	Owner            uint64
	BagSlot          uint8
	Slot             uint8
	IsTradable       bool
	Count            uint32
	Flags            uint32
	Durability       uint32
	RandomPropertyID int32
	Text             string
}

type TakePlayerItemByPosResponse struct {
	Status PlayerItemTakeStatus
	Item   PlayerItem
}

type PlayerItemTakeStatus uint8

const (
	PlayerItemTakeSuccess PlayerItemTakeStatus = iota
	PlayerItemTakePlayerNotFound
	PlayerItemTakeItemNotFound
	PlayerItemTakeItemNotTradable
	PlayerItemTakeFailed
)

type ItemToAdd struct {
	Guid             uint64
	Entry            uint32
	Count            uint32
	Flags            uint32
	Durability       uint32
	RandomPropertyID int32
	Text             string
	StoreAtPos       bool
	BagSlot          uint8
	Slot             uint8
}

type BattlegroundStartRequest struct {
	BattlegroundTypeID uint8
	ArenaType          uint8
	IsRated            bool
	MapID              uint32
	BracketLvl         uint8

	AllianceArenaTeamID           uint32
	HordeArenaTeamID              uint32
	AllianceArenaMatchmakerRating uint32
	HordeArenaMatchmakerRating    uint32

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

type LFGDungeonLock struct {
	DungeonEntry uint32
	LockStatus   uint32
}

type LFGPlayerLockInfoRequest struct {
	PlayerGUID     uint64
	DungeonEntries []uint32
}

type LFGPlayerLockInfoResponse struct {
	Locks               []LFGDungeonLock
	JoinResult          uint32
	ValidDungeonEntries []uint32
}

type LFGPlayerInfoRequest struct {
	PlayerGUID uint64
}

type LFGRewardItem struct {
	ItemID    uint32
	DisplayID uint32
	Count     uint32
}

type LFGRandomDungeonInfo struct {
	DungeonEntry   uint32
	Done           bool
	RewardMoney    uint32
	RewardXP       uint32
	RewardUnknown1 uint32
	RewardUnknown2 uint32
	RewardItems    []LFGRewardItem
}

type LFGPlayerInfoResponse struct {
	RandomDungeons []LFGRandomDungeonInfo
	Locks          []LFGDungeonLock
}

type LFGDungeonInfoRequest struct {
	DungeonEntry uint32
}

type LFGDungeonInfoResponse struct {
	DungeonEntry uint32
	DungeonID    uint32
	MapID        uint32
	TypeID       uint32
	Difficulty   uint32
}

type LFGTeleportPlayerRequest struct {
	PlayerGUID   uint64
	Out          bool
	DungeonEntry uint32
}

type LFGSetBootVoteRequest struct {
	PlayerGUID uint64
	Agree      bool
}

type LFGMaterializeProposalMember struct {
	PlayerGUID      uint64
	SelectedRoles   uint8
	AssignedRole    uint8
	QueueLeaderGUID uint64
}

type LFGMaterializeProposalRequest struct {
	RealmID      uint32
	ProposalID   uint32
	DungeonEntry uint32
	LeaderGUID   uint64
	Members      []LFGMaterializeProposalMember
}

type GuildCreateResponse struct {
	ErrorCode uint32
	GuildID   uint64
}

type CreateGuildHandler func(leaderGuid uint64, guildName string) (*GuildCreateResponse, error)

type GetPlayerItemsByGuidsHandler func(player uint64, items []uint64) ([]PlayerItem, error)

type TakePlayerItemByPosHandler func(player uint64, bagSlot, slot uint8, count uint32, assignToPlayer uint64) (*TakePlayerItemByPosResponse, error)

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

type GetLFGPlayerLockInfoHandler func(request LFGPlayerLockInfoRequest) (*LFGPlayerLockInfoResponse, error)

type GetLFGPlayerInfoHandler func(request LFGPlayerInfoRequest) (*LFGPlayerInfoResponse, error)

type GetLFGDungeonInfoHandler func(request LFGDungeonInfoRequest) (*LFGDungeonInfoResponse, error)

type TeleportLFGPlayerHandler func(request LFGTeleportPlayerRequest) error

type SetLFGBootVoteHandler func(request LFGSetBootVoteRequest) error

type MaterializeLFGProposalHandler func(request LFGMaterializeProposalRequest) error

type CppBindings struct {
	GetPlayerItemsByGuids           GetPlayerItemsByGuidsHandler
	TakePlayerItemByPos             TakePlayerItemByPosHandler
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
	GetLFGPlayerLockInfo            GetLFGPlayerLockInfoHandler
	GetLFGPlayerInfo                GetLFGPlayerInfoHandler
	GetLFGDungeonInfo               GetLFGDungeonInfoHandler
	TeleportLFGPlayer               TeleportLFGPlayerHandler
	SetLFGBootVote                  SetLFGBootVoteHandler
	MaterializeLFGProposal          MaterializeLFGProposalHandler
	CreateGuild                     CreateGuildHandler
}
