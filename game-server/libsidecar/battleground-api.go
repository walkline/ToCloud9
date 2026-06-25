package main

/*
#include "battleground-api.h"
*/
import "C"

import (
	"unsafe"

	"github.com/walkline/ToCloud9/game-server/libsidecar/grpcapi"
)

func tc9BattlegroundGUIDArray(values []uint64) (*C.uint64_t, error) {
	if len(values) == 0 {
		return nil, nil
	}

	ptr := (*C.uint64_t)(C.malloc(C.size_t(len(values)) * C.size_t(unsafe.Sizeof(C.uint64_t(0)))))
	if ptr == nil {
		return nil, grpcapi.BattlegroundError(C.BattlegroundErrorCodeNoHandler)
	}

	slice := unsafe.Slice(ptr, len(values))
	for i, guid := range values {
		slice[i] = C.uint64_t(guid)
	}

	return ptr, nil
}

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
	crequest.allianceArenaTeamID = C.uint32_t(request.AllianceArenaTeamID)
	crequest.hordeArenaTeamID = C.uint32_t(request.HordeArenaTeamID)
	crequest.allianceArenaMatchmakerRating = C.uint32_t(request.AllianceArenaMatchmakerRating)
	crequest.hordeArenaMatchmakerRating = C.uint32_t(request.HordeArenaMatchmakerRating)
	crequest.alliancePlayersToAddSize = C.int(len(request.AlliancePlayerGUIDsToAdd))
	crequest.hordePlayersToAddSize = C.int(len(request.HordePlayerGUIDsToAdd))
	// TODO: add later
	crequest.randomBGPlayersSize = 0

	var err error
	crequest.alliancePlayersToAdd, err = tc9BattlegroundGUIDArray(request.AlliancePlayerGUIDsToAdd)
	if err != nil {
		return nil, err
	}
	crequest.hordePlayersToAdd, err = tc9BattlegroundGUIDArray(request.HordePlayerGUIDsToAdd)
	if err != nil {
		if crequest.alliancePlayersToAdd != nil {
			C.free(unsafe.Pointer(crequest.alliancePlayersToAdd))
		}
		return nil, err
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

	var err error
	crequest.alliancePlayersToAdd, err = tc9BattlegroundGUIDArray(request.AlliancePlayerGUIDsToAdd)
	if err != nil {
		return err
	}
	crequest.hordePlayersToAdd, err = tc9BattlegroundGUIDArray(request.HordePlayerGUIDsToAdd)
	if err != nil {
		if crequest.alliancePlayersToAdd != nil {
			C.free(unsafe.Pointer(crequest.alliancePlayersToAdd))
		}
		return err
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
