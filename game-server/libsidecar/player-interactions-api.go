package main

/*
#include "player-interactions-api.h"
*/
import "C"

import (
	"github.com/walkline/ToCloud9/game-server/libsidecar/grpcapi"
)

// TC9SetCanPlayerInteractWithNPCAndFlagsHandler sets handler for can player interact with NPC and with given NPC flags request.
//
//export TC9SetCanPlayerInteractWithNPCAndFlagsHandler
func TC9SetCanPlayerInteractWithNPCAndFlagsHandler(h C.CanPlayerInteractWithNPCAndFlagsHandler) {
	C.SetCanPlayerInteractWithNPCAndFlagsHandler(h)
}

// TC9SetCanPlayerInteractWithGOAndTypeHandler sets handler for can player interact with GameObject and with given object type request.
//
//export TC9SetCanPlayerInteractWithGOAndTypeHandler
func TC9SetCanPlayerInteractWithGOAndTypeHandler(h C.CanPlayerInteractWithGOAndTypeHandler) {
	C.SetCanPlayerInteractWithGOAndTypeHandler(h)
}

// CanPlayerInteractWithNPCAndFlagsHandler calls C(++) CanPlayerInteractWithNPCAndFlagsHandler implementation and makes Go<->C conversions of in/out params.
func CanPlayerInteractWithNPCAndFlagsHandler(playerGUID, npcGUID uint64, flags uint32) (bool, error) {
	res := C.CallCanPlayerInteractWithNPCAndFlagsHandler(C.uint64_t(playerGUID), C.uint64_t(npcGUID), C.uint32_t(flags))
	if res.errorCode != C.PlayerInteractionErrorCodeNoError {
		return false, grpcapi.InteractionsError(res.errorCode)
	}

	return bool(res.canInteract), nil
}

// CanPlayerInteractWithGOAndTypeHandler calls C(++) CanPlayerInteractWithGOAndTypeHandler implementation and makes Go<->C conversions of in/out params.
func CanPlayerInteractWithGOAndTypeHandler(playerGUID, goGUID uint64, goType uint8) (bool, error) {
	res := C.CallCanPlayerInteractWithGOAndTypeHandler(C.uint64_t(playerGUID), C.uint64_t(goGUID), C.uint8_t(goType))
	if res.errorCode != C.PlayerInteractionErrorCodeNoError {
		return false, grpcapi.InteractionsError(res.errorCode)
	}

	return bool(res.canInteract), nil
}
