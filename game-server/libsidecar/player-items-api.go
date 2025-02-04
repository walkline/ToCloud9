package main

/*
#include "player-items-api.h"
*/
import "C"

import (
	"unsafe"

	"github.com/walkline/ToCloud9/game-server/libsidecar/grpcapi"
)

// TC9SetGetPlayerItemsByGuidsHandler sets handler for getting players item by guids request.
//
//export TC9SetGetPlayerItemsByGuidsHandler
func TC9SetGetPlayerItemsByGuidsHandler(h C.GetPlayerItemsByGuidsHandler) {
	C.SetGetPlayerItemsByGuidsHandler(h)
}

// TC9SetRemoveItemsWithGuidsFromPlayerHandler sets handler for removing items by guids from player request.
//
//export TC9SetRemoveItemsWithGuidsFromPlayerHandler
func TC9SetRemoveItemsWithGuidsFromPlayerHandler(h C.RemoveItemsWithGuidsFromPlayerHandler) {
	C.SetRemoveItemsWithGuidsFromPlayerHandler(h)
}

// TC9SetAddExistingItemToPlayerHandler sets handler for adding item to player request.
//
//export TC9SetAddExistingItemToPlayerHandler
func TC9SetAddExistingItemToPlayerHandler(h C.AddExistingItemToPlayerHandler) {
	C.SetAddExistingItemToPlayerHandler(h)
}

// GetPlayerItemsByGuidHandler calls C++ GetPlayerItemsByGuidHandler implementation and makes Go<->C conversions of in/out params.
func GetPlayerItemsByGuidHandler(player uint64, items []uint64) ([]grpcapi.PlayerItem, error) {
	res := C.CallGetPlayerItemsByGuidsHandler(C.uint64_t(player), (*C.uint64_t)(&items[0]), C.int(len(items)))
	if res.errorCode != C.PlayerItemErrorCodeNoError {
		return nil, grpcapi.ItemError(res.errorCode)
	}

	itemsResult := make([]grpcapi.PlayerItem, int(res.itemsSize))
	returnedItems := unsafe.Slice(res.items, int(res.itemsSize))
	for i := range returnedItems {
		itemsResult[i].Guid = uint64(returnedItems[i].guid)
		itemsResult[i].Entry = uint32(returnedItems[i].entry)
		itemsResult[i].Owner = uint64(returnedItems[i].owner)
		itemsResult[i].BagSlot = uint8(returnedItems[i].bagSlot)
		itemsResult[i].Slot = uint8(returnedItems[i].slot)
		itemsResult[i].IsTradable = bool(returnedItems[i].isTradable)
		itemsResult[i].Count = uint32(returnedItems[i].count)
		itemsResult[i].Flags = uint16(returnedItems[i].flags)
		itemsResult[i].Durability = uint32(returnedItems[i].durability)
		itemsResult[i].RandomPropertyID = uint32(returnedItems[i].randomPropertyID)

		C.free((unsafe.Pointer)(returnedItems[i].text))
	}

	C.free((unsafe.Pointer)(res.items))

	return itemsResult, nil
}

// RemoveItemsWithGuidsFromPlayerHandler calls C++ RemoveItemsWithGuidsFromPlayerHandler implementation and makes Go<->C conversions of in/out params.
func RemoveItemsWithGuidsFromPlayerHandler(player uint64, items []uint64, assignToPlayer uint64) ([]uint64, error) {
	res := C.CallRemoveItemsWithGuidsFromPlayerHandler(
		C.uint64_t(player),
		(*C.uint64_t)(&items[0]),
		C.int(len(items)),
		C.uint64_t(assignToPlayer),
	)
	if res.errorCode != C.PlayerItemErrorCodeNoError {
		return nil, grpcapi.ItemError(res.errorCode)
	}

	itemsResult := make([]uint64, int(res.updatedItemsSize))
	returnedItems := unsafe.Slice(res.updatedItems, int(res.updatedItemsSize))
	for i := range returnedItems {
		itemsResult[i] = uint64(returnedItems[i])
	}

	C.free((unsafe.Pointer)(res.updatedItems))

	return itemsResult, nil
}

// AddExistingItemToPlayerHandler calls C++ AddExistingItemToPlayerHandler implementation and makes Go<->C conversions of in/out params.
func AddExistingItemToPlayerHandler(player uint64, item *grpcapi.ItemToAdd) error {
	var request C.AddExistingItemToPlayerRequest
	request.playerGuid = C.uint64_t(player)
	request.itemGuid = C.uint64_t(item.Guid)
	request.itemEntry = C.uint32_t(item.Entry)
	request.itemCount = C.uint32_t(item.Count)
	request.itemFlags = C.uint16_t(item.Flags)
	request.itemDurability = C.uint8_t(item.Durability)
	request.itemRandomPropertyID = C.int8_t(item.RandomPropertyID)

	res := C.CallAddExistingItemToPlayerHandler(&request)
	if res != C.PlayerItemErrorCodeNoError {
		return grpcapi.ItemError(res)
	}

	return nil
}
