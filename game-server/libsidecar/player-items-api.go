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

// TC9SetTakePlayerItemByPosHandler sets handler for removing one player item by bag/slot.
//
//export TC9SetTakePlayerItemByPosHandler
func TC9SetTakePlayerItemByPosHandler(h C.TakePlayerItemByPosHandler) {
	C.SetTakePlayerItemByPosHandler(h)
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
		itemsResult[i].Flags = uint32(returnedItems[i].flags)
		itemsResult[i].Durability = uint32(returnedItems[i].durability)
		itemsResult[i].RandomPropertyID = int32(returnedItems[i].randomPropertyID)
		itemsResult[i].Text = C.GoString(returnedItems[i].text)

		C.free((unsafe.Pointer)(returnedItems[i].text))
	}

	C.free((unsafe.Pointer)(res.items))

	return itemsResult, nil
}

// TakePlayerItemByPosHandler calls C++ TakePlayerItemByPosHandler implementation and makes Go<->C conversions of in/out params.
func TakePlayerItemByPosHandler(player uint64, bagSlot, slot uint8, count uint32, assignToPlayer uint64) (*grpcapi.TakePlayerItemByPosResponse, error) {
	res := C.CallTakePlayerItemByPosHandler(
		C.uint64_t(player),
		C.uint8_t(bagSlot),
		C.uint8_t(slot),
		C.uint32_t(count),
		C.uint64_t(assignToPlayer),
	)

	switch res.errorCode {
	case C.PlayerItemErrorCodeNoError:
		item := grpcapi.PlayerItem{
			Guid:             uint64(res.item.guid),
			Entry:            uint32(res.item.entry),
			Owner:            uint64(res.item.owner),
			BagSlot:          uint8(res.item.bagSlot),
			Slot:             uint8(res.item.slot),
			IsTradable:       bool(res.item.isTradable),
			Count:            uint32(res.item.count),
			Flags:            uint32(res.item.flags),
			Durability:       uint32(res.item.durability),
			RandomPropertyID: int32(res.item.randomPropertyID),
			Text:             C.GoString(res.item.text),
		}
		C.free((unsafe.Pointer)(res.item.text))

		return &grpcapi.TakePlayerItemByPosResponse{
			Status: grpcapi.PlayerItemTakeSuccess,
			Item:   item,
		}, nil
	case C.PlayerItemErrorCodePlayerNotFound:
		return &grpcapi.TakePlayerItemByPosResponse{Status: grpcapi.PlayerItemTakePlayerNotFound}, nil
	case C.PlayerItemErrorItemNotFound:
		return &grpcapi.TakePlayerItemByPosResponse{Status: grpcapi.PlayerItemTakeItemNotFound}, nil
	case C.PlayerItemErrorItemNotTradable:
		return &grpcapi.TakePlayerItemByPosResponse{Status: grpcapi.PlayerItemTakeItemNotTradable}, nil
	default:
		return &grpcapi.TakePlayerItemByPosResponse{Status: grpcapi.PlayerItemTakeFailed}, nil
	}
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
	request.itemFlags = C.uint32_t(item.Flags)
	request.itemDurability = C.uint8_t(item.Durability)
	request.itemRandomPropertyID = C.int32_t(item.RandomPropertyID)
	request.storeAtPos = C.bool(item.StoreAtPos)
	request.bagSlot = C.uint8_t(item.BagSlot)
	request.slot = C.uint8_t(item.Slot)

	res := C.CallAddExistingItemToPlayerHandler(&request)
	if res != C.PlayerItemErrorCodeNoError {
		return grpcapi.ItemError(res)
	}

	return nil
}
