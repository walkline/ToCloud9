package main

/*
#include "guild-api.h"
*/
import "C"

import (
	"unsafe"

	"github.com/walkline/ToCloud9/game-server/libsidecar/grpcapi"
)

// TC9SetGuildCreateHandler sets handler for creating guilds.
//
//export TC9SetGuildCreateHandler
func TC9SetGuildCreateHandler(h C.GuildCreateHandler) {
	C.SetGuildCreateHandler(h)
}

func CreateGuildHandler(leaderGuid uint64, guildName string) (*grpcapi.GuildCreateResponse, error) {
	cName := C.CString(guildName)
	defer C.free(unsafe.Pointer(cName))

	req := C.GuildCreateRequest{
		leaderGuid: C.uint64_t(leaderGuid),
		guildName:  cName,
	}

	res := C.CallGuildCreateHandler(&req)

	return &grpcapi.GuildCreateResponse{
		ErrorCode: uint32(res.errorCode),
		GuildID:   uint64(res.guildId),
	}, nil
}
