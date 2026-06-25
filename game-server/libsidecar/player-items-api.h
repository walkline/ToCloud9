#ifndef __PLAYER_ITEMS_API__
#define __PLAYER_ITEMS_API__

#include <stdint.h>
#include <stdbool.h>
#include <stdlib.h>

typedef enum PlayerItemErrorCode {
    PlayerItemErrorCodeNoError        = 0,
    PlayerItemErrorCodeNoHandler      = 1,
    PlayerItemErrorCodePlayerNotFound = 2,
    PlayerItemErrorNoInventorySpace   = 3,
    PlayerItemErrorUnknownTemplate    = 4,
    PlayerItemErrorFailedToCreateItem = 5,
    PlayerItemErrorItemNotFound       = 6,
    PlayerItemErrorItemNotTradable    = 7
} PlayerItemErrorCode;

// GetPlayerItemsByGuids request.
typedef struct {
    uint64_t guid;
    uint32_t entry;
    uint64_t owner;
    uint8_t bagSlot;
    uint8_t slot;
    bool isTradable;
    uint32_t count;
    uint32_t flags;
    uint8_t durability;
    int32_t randomPropertyID;
    const char* text;
} PlayerItem;

typedef struct {
    int errorCode;
    PlayerItem* items;
    int itemsSize;
} GetPlayerItemsByGuidsResponse;

typedef GetPlayerItemsByGuidsResponse (*GetPlayerItemsByGuidsHandler) (uint64_t /*player_guid*/, uint64_t* /*items_guids*/, int /*items_guids_size*/);
void SetGetPlayerItemsByGuidsHandler(GetPlayerItemsByGuidsHandler h);
GetPlayerItemsByGuidsResponse CallGetPlayerItemsByGuidsHandler(uint64_t player_guid, uint64_t* items_guids, int items_guids_size);

// TakePlayerItemByPos request.
typedef struct {
    int errorCode;
    PlayerItem item;
} TakePlayerItemByPosResponse;

typedef TakePlayerItemByPosResponse (*TakePlayerItemByPosHandler) (uint64_t /*player_guid*/, uint8_t /*bag_slot*/, uint8_t /*slot*/, uint32_t /*count*/, uint64_t /*assign_player_guid*/);
void SetTakePlayerItemByPosHandler(TakePlayerItemByPosHandler h);
TakePlayerItemByPosResponse CallTakePlayerItemByPosHandler(uint64_t player_guid, uint8_t bag_slot, uint8_t slot, uint32_t count, uint64_t assign_player_guid);

// RemoveItemsWithGuidsFromPlayer request.
typedef struct {
    int errorCode;
    uint64_t* updatedItems;
    int updatedItemsSize;
} RemoveItemsWithGuidsFromPlayerResponse;

typedef RemoveItemsWithGuidsFromPlayerResponse (*RemoveItemsWithGuidsFromPlayerHandler) (uint64_t /*player_guid*/, uint64_t* /*items_guids*/, int /*items_guids_size*/, uint64_t /*assign_player_guid*/);
void SetRemoveItemsWithGuidsFromPlayerHandler(RemoveItemsWithGuidsFromPlayerHandler h);
RemoveItemsWithGuidsFromPlayerResponse CallRemoveItemsWithGuidsFromPlayerHandler(uint64_t player_guid, uint64_t* items_guids, int items_guids_size, uint64_t assign_player_guid);

// AddExistingItemToPlayer request.
typedef struct {
    uint64_t playerGuid;
    uint64_t itemGuid;
    uint32_t itemEntry;
    uint32_t itemCount;
    uint32_t itemFlags;
    uint8_t itemDurability;
    int32_t itemRandomPropertyID;
    bool storeAtPos;
    uint8_t bagSlot;
    uint8_t slot;
} AddExistingItemToPlayerRequest;

typedef PlayerItemErrorCode (*AddExistingItemToPlayerHandler) (AddExistingItemToPlayerRequest*);
void SetAddExistingItemToPlayerHandler(AddExistingItemToPlayerHandler h);
PlayerItemErrorCode CallAddExistingItemToPlayerHandler(AddExistingItemToPlayerRequest*);

#endif
