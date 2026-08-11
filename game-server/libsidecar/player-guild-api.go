package main

/*
#include "player-guild-api.h"
*/
import "C"

import "errors"

// TC9SetSetPlayerGuildFieldsHandler sets handler for refreshing a player's guild fields.
//
//export TC9SetSetPlayerGuildFieldsHandler
func TC9SetSetPlayerGuildFieldsHandler(h C.SetPlayerGuildFieldsHandler) {
	C.SetSetPlayerGuildFieldsHandler(h)
}

// SetPlayerGuildFieldsHandler calls the C(++) implementation and makes Go<->C conversions.
// Returns whether the player was online on this worldserver.
func SetPlayerGuildFieldsHandler(guid uint64, guildID, rank uint32) (bool, error) {
	res := C.CallSetPlayerGuildFieldsHandler(C.uint64_t(guid), C.uint32_t(guildID), C.uint32_t(rank))
	if res.errorCode != C.PlayerGuildErrorCodeNoError {
		return false, errors.New("no SetPlayerGuildFields handler registered")
	}

	return bool(res.applied), nil
}
