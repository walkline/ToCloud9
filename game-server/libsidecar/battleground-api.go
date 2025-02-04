package main

/*
#include "battleground-api.h"
*/
import "C"

import (
	"unsafe"

	"github.com/walkline/ToCloud9/game-server/libsidecar/grpcapi"
)

// TC9SetBattlegroundStartHandler sets handler for starting battleground.
//
//export TC9SetBattlegroundStartHandler
func TC9SetBattlegroundStartHandler(h C.BattlegroundStartHandler) {
	C.SetBattlegroundStartHandler(h)
}

// BattlegroundStartHandler calls C(++) BattlegroundStartHandler implementation and makes Go<->C conversions of in/out params.
func BattlegroundStartHandler(request grpcapi.BattlegroundStartRequest) (*grpcapi.BattlegroundStartResponse, error) {
	var crequest C.BattlegroundStartRequest
	crequest.battlegroundTypeID = C.uint8_t(request.BattlegroundTypeID)
	crequest.arenaType = C.uint32_t(request.ArenaType)
	crequest.isRated = C.bool(request.IsRated)
	crequest.mapID = C.uint32_t(request.MapID)
	crequest.bracketLvl = C.uint8_t(request.BracketLvl)
	crequest.alliancePlayersToAddSize = C.int(len(request.AlliancePlayerGUIDsToAdd))
	crequest.hordePlayersToAddSize = C.int(len(request.HordePlayerGUIDsToAdd))
	// TODO: add later
	crequest.randomBGPlayersSize = 0

	if len(request.AlliancePlayerGUIDsToAdd) > 0 {
		crequest.alliancePlayersToAdd = (*C.uint64_t)(C.malloc(C.size_t(len(request.AlliancePlayerGUIDsToAdd)) * C.size_t(unsafe.Sizeof(C.uint64_t(0)))))
		if crequest.alliancePlayersToAdd == nil {
			return nil, grpcapi.BattlegroundError(C.BattlegroundErrorCodeNoHandler) // or an appropriate error
		}

		for i, guid := range request.AlliancePlayerGUIDsToAdd {
			cguid := (*C.uint64_t)(unsafe.Pointer(uintptr(unsafe.Pointer(crequest.alliancePlayersToAdd)) + uintptr(i)*unsafe.Sizeof(*crequest.alliancePlayersToAdd)))
			*cguid = C.uint64_t(guid)
		}
	} else {
		crequest.alliancePlayersToAdd = nil
	}

	if len(request.HordePlayerGUIDsToAdd) > 0 {
		crequest.hordePlayersToAdd = (*C.uint64_t)(C.malloc(C.size_t(len(request.HordePlayerGUIDsToAdd)) * C.size_t(unsafe.Sizeof(C.uint64_t(0)))))
		if crequest.hordePlayersToAdd == nil {
			return nil, grpcapi.BattlegroundError(C.BattlegroundErrorCodeNoHandler) // or an appropriate error
		}

		for i, guid := range request.HordePlayerGUIDsToAdd {
			cguid := (*C.uint64_t)(unsafe.Pointer(uintptr(unsafe.Pointer(crequest.hordePlayersToAdd)) + uintptr(i)*unsafe.Sizeof(*crequest.alliancePlayersToAdd)))
			*cguid = C.uint64_t(guid)
		}
	} else {
		crequest.hordePlayersToAdd = nil
	}

	res := C.CallBattlegroundStartHandler((*C.BattlegroundStartRequest)(unsafe.Pointer(&crequest)))

	if crequest.alliancePlayersToAdd != nil {
		C.free(unsafe.Pointer(crequest.alliancePlayersToAdd))
	}

	if crequest.hordePlayersToAdd != nil {
		C.free(unsafe.Pointer(crequest.hordePlayersToAdd))
	}

	if res.errorCode != C.BattlegroundErrorCodeNoError {
		return nil, grpcapi.BattlegroundError(res.errorCode)
	}

	return &grpcapi.BattlegroundStartResponse{
		InstanceID:       uint64(res.instanceID),
		InstanceClientID: uint64(res.instanceClientID),
	}, nil
}

// TC9SetBattlegroundAddPlayersHandler sets handler for adding players to battleground.
//
//export TC9SetBattlegroundAddPlayersHandler
func TC9SetBattlegroundAddPlayersHandler(h C.BattlegroundAddPlayersHandler) {
	C.SetBattlegroundAddPlayersHandler(h)
}

// BattlegroundAddPlayersHandler calls C(++) BattlegroundAddPlayersHandler implementation and makes Go<->C conversions of in/out params.
func BattlegroundAddPlayersHandler(request grpcapi.BattlegroundAddPlayersRequest) error {
	var crequest C.BattlegroundAddPlayersRequest
	crequest.battlegroundTypeID = C.uint8_t(request.BattlegroundTypeID)
	crequest.instanceID = C.uint64_t(request.InstanceID)
	crequest.alliancePlayersToAddSize = C.int(len(request.AlliancePlayerGUIDsToAdd))
	crequest.hordePlayersToAddSize = C.int(len(request.HordePlayerGUIDsToAdd))
	// TODO: add later
	crequest.randomBGPlayersSize = 0

	if len(request.AlliancePlayerGUIDsToAdd) > 0 {
		crequest.alliancePlayersToAdd = (*C.uint64_t)(C.malloc(C.size_t(len(request.AlliancePlayerGUIDsToAdd)) * C.size_t(unsafe.Sizeof(C.uint64_t(0)))))
		if crequest.alliancePlayersToAdd == nil {
			return grpcapi.BattlegroundError(C.BattlegroundErrorCodeNoHandler) // or an appropriate error
		}

		for i, guid := range request.AlliancePlayerGUIDsToAdd {
			cguid := (*C.uint64_t)(unsafe.Pointer(uintptr(unsafe.Pointer(crequest.alliancePlayersToAdd)) + uintptr(i)*unsafe.Sizeof(*crequest.alliancePlayersToAdd)))
			*cguid = C.uint64_t(guid)
		}
	} else {
		crequest.alliancePlayersToAdd = nil
	}

	if len(request.HordePlayerGUIDsToAdd) > 0 {
		crequest.hordePlayersToAdd = (*C.uint64_t)(C.malloc(C.size_t(len(request.HordePlayerGUIDsToAdd)) * C.size_t(unsafe.Sizeof(C.uint64_t(0)))))
		if crequest.hordePlayersToAdd == nil {
			return grpcapi.BattlegroundError(C.BattlegroundErrorCodeNoHandler) // or an appropriate error
		}

		for i, guid := range request.HordePlayerGUIDsToAdd {
			cguid := (*C.uint64_t)(unsafe.Pointer(uintptr(unsafe.Pointer(crequest.hordePlayersToAdd)) + uintptr(i)*unsafe.Sizeof(*crequest.alliancePlayersToAdd)))
			*cguid = C.uint64_t(guid)
		}
	} else {
		crequest.hordePlayersToAdd = nil
	}

	res := C.CallBattlegroundAddPlayersHandler((*C.BattlegroundAddPlayersRequest)(unsafe.Pointer(&crequest)))

	if crequest.alliancePlayersToAdd != nil {
		C.free(unsafe.Pointer(crequest.alliancePlayersToAdd))
	}

	if crequest.hordePlayersToAdd != nil {
		C.free(unsafe.Pointer(crequest.hordePlayersToAdd))
	}

	if res != C.BattlegroundErrorCodeNoError {
		return grpcapi.BattlegroundError(res)
	}

	return nil
}

// TC9SetCanPlayerJoinBattlegroundQueueHandler sets handler for checking if player can join to battleground queue.
//
//export TC9SetCanPlayerJoinBattlegroundQueueHandler
func TC9SetCanPlayerJoinBattlegroundQueueHandler(h C.CanPlayerJoinBattlegroundQueueHandler) {
	C.SetCanPlayerJoinBattlegroundQueueHandler(h)
}

// CanPlayerJoinBattlegroundQueueHandler calls C(++) CanPlayerJoinBattlegroundQueueHandler implementation and makes Go<->C conversions of in/out params.
func CanPlayerJoinBattlegroundQueueHandler(player uint64) error {
	res := C.CallCanPlayerJoinBattlegroundQueueHandler(C.uint64_t(player))
	if res != C.BattlegroundErrorCodeNoError {
		return grpcapi.BattlegroundJoinCheckError(res)
	}
	return nil
}

// TC9SetCanPlayerTeleportToBattlegroundHandler sets handler for checking if player can teleport to battleground.
//
//export TC9SetCanPlayerTeleportToBattlegroundHandler
func TC9SetCanPlayerTeleportToBattlegroundHandler(h C.CanPlayerTeleportToBattlegroundHandler) {
	C.SetCanPlayerTeleportToBattlegroundHandler(h)
}

// CanPlayerTeleportToBattlegroundHandler calls C(++) CanPlayerTeleportToBattlegroundHandler implementation and makes Go<->C conversions of in/out params.
func CanPlayerTeleportToBattlegroundHandler(player uint64) error {
	res := C.CallCanPlayerTeleportToBattlegroundHandler(C.uint64_t(player))
	if res != C.BattlegroundErrorCodeNoError {
		return grpcapi.BattlegroundJoinCheckError(res)
	}
	return nil
}
