package main

/*
#include "lfg-api.h"
*/
import "C"

import (
	"unsafe"

	"github.com/walkline/ToCloud9/game-server/libsidecar/grpcapi"
)

// TC9SetLfgPlayerLockInfoHandler sets handler for AzerothCore LFG lock info.
//
//export TC9SetLfgPlayerLockInfoHandler
func TC9SetLfgPlayerLockInfoHandler(h C.LfgPlayerLockInfoHandler) {
	C.SetLfgPlayerLockInfoHandler(h)
}

func LfgPlayerLockInfoHandler(request grpcapi.LFGPlayerLockInfoRequest) (*grpcapi.LFGPlayerLockInfoResponse, error) {
	var crequest C.LfgPlayerLockInfoRequest
	crequest.playerGuid = C.uint64_t(request.PlayerGUID)
	crequest.dungeonEntriesSize = C.int(len(request.DungeonEntries))

	if len(request.DungeonEntries) > 0 {
		crequest.dungeonEntries = (*C.uint32_t)(C.malloc(C.size_t(len(request.DungeonEntries)) * C.size_t(unsafe.Sizeof(C.uint32_t(0)))))
		if crequest.dungeonEntries == nil {
			return nil, grpcapi.LFGPlayerLockInfoError(grpcapi.LFGPlayerLockInfoErrorCodeInternalError)
		}
		defer C.free(unsafe.Pointer(crequest.dungeonEntries))

		for i, entry := range request.DungeonEntries {
			centry := (*C.uint32_t)(unsafe.Pointer(uintptr(unsafe.Pointer(crequest.dungeonEntries)) + uintptr(i)*unsafe.Sizeof(*crequest.dungeonEntries)))
			*centry = C.uint32_t(entry)
		}
	}

	res := C.CallLfgPlayerLockInfoHandler((*C.LfgPlayerLockInfoRequest)(unsafe.Pointer(&crequest)))
	if res.errorCode != C.LfgPlayerLockInfoErrorCodeSuccess {
		return nil, grpcapi.LFGPlayerLockInfoError(res.errorCode)
	}
	if res.locks != nil {
		defer C.free(unsafe.Pointer(res.locks))
	}
	if res.validDungeonEntries != nil {
		defer C.free(unsafe.Pointer(res.validDungeonEntries))
	}

	locks := make([]grpcapi.LFGDungeonLock, 0, int(res.locksSize))
	for i := 0; i < int(res.locksSize); i++ {
		clock := (*C.LfgDungeonLock)(unsafe.Pointer(uintptr(unsafe.Pointer(res.locks)) + uintptr(i)*unsafe.Sizeof(*res.locks)))
		locks = append(locks, grpcapi.LFGDungeonLock{
			DungeonEntry: uint32(clock.dungeonEntry),
			LockStatus:   uint32(clock.lockStatus),
		})
	}

	validDungeonEntries := make([]uint32, 0, int(res.validDungeonEntriesSize))
	for i := 0; i < int(res.validDungeonEntriesSize); i++ {
		entry := (*C.uint32_t)(unsafe.Pointer(uintptr(unsafe.Pointer(res.validDungeonEntries)) + uintptr(i)*unsafe.Sizeof(*res.validDungeonEntries)))
		validDungeonEntries = append(validDungeonEntries, uint32(*entry))
	}

	return &grpcapi.LFGPlayerLockInfoResponse{
		Locks:               locks,
		JoinResult:          uint32(res.joinResult),
		ValidDungeonEntries: validDungeonEntries,
	}, nil
}

// TC9SetLfgPlayerInfoHandler sets handler for AzerothCore LFG player catalog and lock info.
//
//export TC9SetLfgPlayerInfoHandler
func TC9SetLfgPlayerInfoHandler(h C.LfgPlayerInfoHandler) {
	C.SetLfgPlayerInfoHandler(h)
}

func LfgPlayerInfoHandler(request grpcapi.LFGPlayerInfoRequest) (*grpcapi.LFGPlayerInfoResponse, error) {
	var crequest C.LfgPlayerInfoRequest
	crequest.playerGuid = C.uint64_t(request.PlayerGUID)

	res := C.CallLfgPlayerInfoHandler((*C.LfgPlayerInfoRequest)(unsafe.Pointer(&crequest)))
	if res.errorCode != C.LfgPlayerInfoErrorCodeSuccess {
		return nil, grpcapi.LFGPlayerInfoError(res.errorCode)
	}
	if res.randomDungeons != nil {
		defer C.free(unsafe.Pointer(res.randomDungeons))
	}
	if res.locks != nil {
		defer C.free(unsafe.Pointer(res.locks))
	}

	randomDungeons := make([]grpcapi.LFGRandomDungeonInfo, 0, int(res.randomDungeonsSize))
	for i := 0; i < int(res.randomDungeonsSize); i++ {
		cdungeon := (*C.LfgRandomDungeonInfo)(unsafe.Pointer(uintptr(unsafe.Pointer(res.randomDungeons)) + uintptr(i)*unsafe.Sizeof(*res.randomDungeons)))
		if cdungeon.rewardItems != nil {
			defer C.free(unsafe.Pointer(cdungeon.rewardItems))
		}

		rewardItems := make([]grpcapi.LFGRewardItem, 0, int(cdungeon.rewardItemsSize))
		for j := 0; j < int(cdungeon.rewardItemsSize); j++ {
			citem := (*C.LfgRewardItem)(unsafe.Pointer(uintptr(unsafe.Pointer(cdungeon.rewardItems)) + uintptr(j)*unsafe.Sizeof(*cdungeon.rewardItems)))
			rewardItems = append(rewardItems, grpcapi.LFGRewardItem{
				ItemID:    uint32(citem.itemId),
				DisplayID: uint32(citem.displayId),
				Count:     uint32(citem.count),
			})
		}

		randomDungeons = append(randomDungeons, grpcapi.LFGRandomDungeonInfo{
			DungeonEntry:   uint32(cdungeon.dungeonEntry),
			Done:           cdungeon.done != 0,
			RewardMoney:    uint32(cdungeon.rewardMoney),
			RewardXP:       uint32(cdungeon.rewardXP),
			RewardUnknown1: uint32(cdungeon.rewardUnknown1),
			RewardUnknown2: uint32(cdungeon.rewardUnknown2),
			RewardItems:    rewardItems,
		})
	}

	locks := make([]grpcapi.LFGDungeonLock, 0, int(res.locksSize))
	for i := 0; i < int(res.locksSize); i++ {
		clock := (*C.LfgDungeonLock)(unsafe.Pointer(uintptr(unsafe.Pointer(res.locks)) + uintptr(i)*unsafe.Sizeof(*res.locks)))
		locks = append(locks, grpcapi.LFGDungeonLock{
			DungeonEntry: uint32(clock.dungeonEntry),
			LockStatus:   uint32(clock.lockStatus),
		})
	}

	return &grpcapi.LFGPlayerInfoResponse{
		RandomDungeons: randomDungeons,
		Locks:          locks,
	}, nil
}

// TC9SetLfgDungeonInfoHandler sets handler for AzerothCore LFG dungeon metadata.
//
//export TC9SetLfgDungeonInfoHandler
func TC9SetLfgDungeonInfoHandler(h C.LfgDungeonInfoHandler) {
	C.SetLfgDungeonInfoHandler(h)
}

func LfgDungeonInfoHandler(request grpcapi.LFGDungeonInfoRequest) (*grpcapi.LFGDungeonInfoResponse, error) {
	var crequest C.LfgDungeonInfoRequest
	crequest.dungeonEntry = C.uint32_t(request.DungeonEntry)

	res := C.CallLfgDungeonInfoHandler((*C.LfgDungeonInfoRequest)(unsafe.Pointer(&crequest)))
	if res.errorCode != C.LfgDungeonInfoErrorCodeSuccess {
		return nil, grpcapi.LFGDungeonInfoError(res.errorCode)
	}

	return &grpcapi.LFGDungeonInfoResponse{
		DungeonEntry: uint32(res.dungeonEntry),
		DungeonID:    uint32(res.dungeonId),
		MapID:        uint32(res.mapId),
		TypeID:       uint32(res.typeId),
		Difficulty:   uint32(res.difficulty),
	}, nil
}

// TC9SetLfgTeleportPlayerHandler sets handler for AzerothCore LFG teleport in/out.
//
//export TC9SetLfgTeleportPlayerHandler
func TC9SetLfgTeleportPlayerHandler(h C.LfgTeleportPlayerHandler) {
	C.SetLfgTeleportPlayerHandler(h)
}

func LfgTeleportPlayerHandler(request grpcapi.LFGTeleportPlayerRequest) error {
	var crequest C.LfgTeleportPlayerRequest
	crequest.playerGuid = C.uint64_t(request.PlayerGUID)
	crequest.out = C.uint8_t(boolToUint8(request.Out))
	crequest.dungeonEntry = C.uint32_t(request.DungeonEntry)

	res := C.CallLfgTeleportPlayerHandler((*C.LfgTeleportPlayerRequest)(unsafe.Pointer(&crequest)))
	if res != C.LfgTeleportPlayerErrorCodeSuccess {
		return grpcapi.LFGTeleportPlayerError(int(res))
	}
	return nil
}

// TC9SetLfgBootVoteHandler sets handler for AzerothCore LFG boot vote replies.
//
//export TC9SetLfgBootVoteHandler
func TC9SetLfgBootVoteHandler(h C.LfgBootVoteHandler) {
	C.SetLfgBootVoteHandler(h)
}

func LfgBootVoteHandler(request grpcapi.LFGSetBootVoteRequest) error {
	var crequest C.LfgBootVoteRequest
	crequest.playerGuid = C.uint64_t(request.PlayerGUID)
	crequest.agree = C.uint8_t(boolToUint8(request.Agree))

	res := C.CallLfgBootVoteHandler((*C.LfgBootVoteRequest)(unsafe.Pointer(&crequest)))
	if res != C.LfgBootVoteErrorCodeSuccess {
		return grpcapi.LFGSetBootVoteError(int(res))
	}
	return nil
}

// TC9SetLfgMaterializeProposalHandler sets handler for AzerothCore LFG proposal materialization.
//
//export TC9SetLfgMaterializeProposalHandler
func TC9SetLfgMaterializeProposalHandler(h C.LfgMaterializeProposalHandler) {
	C.SetLfgMaterializeProposalHandler(h)
}

func LfgMaterializeProposalHandler(request grpcapi.LFGMaterializeProposalRequest) error {
	var crequest C.LfgMaterializeProposalRequest
	crequest.realmId = C.uint32_t(request.RealmID)
	crequest.proposalId = C.uint32_t(request.ProposalID)
	crequest.dungeonEntry = C.uint32_t(request.DungeonEntry)
	crequest.leaderGuid = C.uint64_t(request.LeaderGUID)
	crequest.membersSize = C.int(len(request.Members))

	if len(request.Members) > 0 {
		crequest.members = (*C.LfgMaterializeProposalMember)(C.malloc(C.size_t(len(request.Members)) * C.size_t(unsafe.Sizeof(C.LfgMaterializeProposalMember{}))))
		if crequest.members == nil {
			return grpcapi.LFGMaterializeProposalError(grpcapi.LFGMaterializeProposalErrorCodeInternalError)
		}
		defer C.free(unsafe.Pointer(crequest.members))

		for i, member := range request.Members {
			cmember := (*C.LfgMaterializeProposalMember)(unsafe.Pointer(uintptr(unsafe.Pointer(crequest.members)) + uintptr(i)*unsafe.Sizeof(*crequest.members)))
			cmember.playerGuid = C.uint64_t(member.PlayerGUID)
			cmember.selectedRoles = C.uint8_t(member.SelectedRoles)
			cmember.assignedRole = C.uint8_t(member.AssignedRole)
			cmember.queueLeaderGuid = C.uint64_t(member.QueueLeaderGUID)
		}
	}

	res := C.CallLfgMaterializeProposalHandler((*C.LfgMaterializeProposalRequest)(unsafe.Pointer(&crequest)))
	if res != C.LfgMaterializeProposalErrorCodeSuccess {
		return grpcapi.LFGMaterializeProposalError(int(res))
	}
	return nil
}
