package main

/*
#include "player-money-api.h"
*/
import "C"

import "github.com/walkline/ToCloud9/game-server/libsidecar/grpcapi"

// TC9SetGetMoneyForPlayerHandler sets handler for getting money for player request.
//
//export TC9SetGetMoneyForPlayerHandler
func TC9SetGetMoneyForPlayerHandler(h C.GetMoneyForPlayerHandler) {
	C.SetGetMoneyForPlayerHandler(h)
}

// TC9SetModifyMoneyForPlayerHandler sets handler for modify money for given player request.
//
//export TC9SetModifyMoneyForPlayerHandler
func TC9SetModifyMoneyForPlayerHandler(h C.ModifyMoneyForPlayerHandler) {
	C.SetModifyMoneyForPlayerHandler(h)
}

// GetMoneyForPlayerHandler calls C(++) GetMoneyForPlayerHandler implementation and makes Go<->C conversions of in/out params.
func GetMoneyForPlayerHandler(guid uint64) (uint32, error) {
	res := C.CallGetMoneyForPlayerHandler(C.uint64_t(guid))
	if res.errorCode != C.PlayerMoneyErrorCodeNoError {
		return 0, grpcapi.MoneyError(res.errorCode)
	}

	return uint32(res.money), nil
}

// ModifyMoneyForPlayerHandler calls C(++) ModifyMoneyForPlayerHandler implementation and makes Go<->C conversions of in/out params.
func ModifyMoneyForPlayerHandler(guid uint64, value int32) (uint32, error) {
	res := C.CallModifyMoneyForPlayerHandler(C.uint64_t(guid), C.int32_t(value))
	if res.errorCode != C.PlayerMoneyErrorCodeNoError {
		return 0, grpcapi.MoneyError(res.errorCode)
	}

	return uint32(res.newMoneyValue), nil
}
