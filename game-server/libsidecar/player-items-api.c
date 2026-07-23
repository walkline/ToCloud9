#include "player-items-api.h"

static GetPlayerItemsByGuidsHandler getPlayerItemsByGuidsHandler;
void SetGetPlayerItemsByGuidsHandler(GetPlayerItemsByGuidsHandler h) {
    getPlayerItemsByGuidsHandler = h;
}

GetPlayerItemsByGuidsResponse CallGetPlayerItemsByGuidsHandler(uint64_t player_guid, uint64_t* items_guids, int items_guids_size) {
    if (getPlayerItemsByGuidsHandler == 0) {
        GetPlayerItemsByGuidsResponse resp;
        resp.errorCode = PlayerItemErrorCodeNoHandler;
        return resp;
    }

    return getPlayerItemsByGuidsHandler(player_guid, items_guids, items_guids_size);
}

static GetPlayerItemByPosHandler getPlayerItemByPosHandler;
void SetGetPlayerItemByPosHandler(GetPlayerItemByPosHandler h) {
    getPlayerItemByPosHandler = h;
}

GetPlayerItemByPosResponse CallGetPlayerItemByPosHandler(uint64_t player_guid, uint8_t bag, uint8_t slot) {
    if (getPlayerItemByPosHandler == 0) {
        GetPlayerItemByPosResponse resp = {0};
        resp.errorCode = PlayerItemErrorCodeNoHandler;
        return resp;
    }

    return getPlayerItemByPosHandler(player_guid, bag, slot);
}

static RemoveItemsWithGuidsFromPlayerHandler removeItemsWithGuidsFromPlayerHandler;
void SetRemoveItemsWithGuidsFromPlayerHandler(RemoveItemsWithGuidsFromPlayerHandler h) {
    removeItemsWithGuidsFromPlayerHandler = h;
}

RemoveItemsWithGuidsFromPlayerResponse CallRemoveItemsWithGuidsFromPlayerHandler(uint64_t player_guid, uint64_t* items_guids, int items_guids_size, uint64_t assign_player_guid) {
    if (getPlayerItemsByGuidsHandler == 0) {
        RemoveItemsWithGuidsFromPlayerResponse resp;
        resp.errorCode = PlayerItemErrorCodeNoHandler;
        return resp;
    }

    return removeItemsWithGuidsFromPlayerHandler(player_guid, items_guids, items_guids_size, assign_player_guid);
}

static AddExistingItemToPlayerHandler addExistingItemToPlayerHandler;
void SetAddExistingItemToPlayerHandler(AddExistingItemToPlayerHandler h) {
    addExistingItemToPlayerHandler = h;
}

PlayerItemErrorCode CallAddExistingItemToPlayerHandler(AddExistingItemToPlayerRequest *r) {
    if (addExistingItemToPlayerHandler == 0) {
        return PlayerItemErrorCodeNoHandler;
    }

    return addExistingItemToPlayerHandler(r);
}
