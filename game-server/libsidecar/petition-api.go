package main

/*
#include "petition-api.h"
*/
import "C"

import (
	"unsafe"

	"github.com/walkline/ToCloud9/game-server/libsidecar/grpcapi"
)

// TC9SetCanTurnInGuildPetitionHandler sets handler that validates a guild petition
// turn-in on the worldserver side (charter item, ownership, signatures).
//
//export TC9SetCanTurnInGuildPetitionHandler
func TC9SetCanTurnInGuildPetitionHandler(h C.CanTurnInGuildPetitionHandler) {
	C.SetCanTurnInGuildPetitionHandler(h)
}

// CanTurnInGuildPetitionHandler calls C(++) CanTurnInGuildPetitionHandler implementation and makes Go<->C conversions of in/out params.
func CanTurnInGuildPetitionHandler(playerGUID, petitionItemGUID uint64) (*grpcapi.GuildPetitionCheckResult, error) {
	res := C.CallCanTurnInGuildPetitionHandler(C.uint64_t(playerGUID), C.uint64_t(petitionItemGUID))

	if res.status == C.GuildPetitionCheckStatusNoHandler {
		return nil, grpcapi.ErrNoPetitionHandler
	}

	result := &grpcapi.GuildPetitionCheckResult{
		Status: grpcapi.GuildPetitionCheckStatus(res.status),
	}

	if res.guildName != nil {
		result.GuildName = C.GoString(res.guildName)
		C.free(unsafe.Pointer(res.guildName))
	}

	if res.signatoryGUIDs != nil {
		size := int(res.signatoryGUIDsSize)
		result.SignatoryGUIDs = make([]uint64, size)
		for i := 0; i < size; i++ {
			cguid := (*C.uint64_t)(unsafe.Pointer(uintptr(unsafe.Pointer(res.signatoryGUIDs)) + uintptr(i)*unsafe.Sizeof(*res.signatoryGUIDs)))
			result.SignatoryGUIDs[i] = uint64(*cguid)
		}
		C.free(unsafe.Pointer(res.signatoryGUIDs))
	}

	return result, nil
}
