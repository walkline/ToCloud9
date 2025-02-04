package main

/*
#include "events-servers-registry.h"
*/
import "C"

import (
	"github.com/rs/zerolog"

	"github.com/walkline/ToCloud9/game-server/libsidecar/consumer"
	"github.com/walkline/ToCloud9/game-server/libsidecar/queue"
	"github.com/walkline/ToCloud9/shared/events"
)

// TC9SetOnMapsReassignedHook sets hook for maps reassigning by servers registry event.
//
//export TC9SetOnMapsReassignedHook
func TC9SetOnMapsReassignedHook(h C.OnMapsReassignedHook) {
	C.SetOnMapsReassignedHook(h)
}

type serversRegistryHandlerFabric struct {
	logger zerolog.Logger
}

func NewServerRegistryHandlerFabric(logger zerolog.Logger) consumer.ServerRegistryHandlerFabric {
	return &serversRegistryHandlerFabric{
		logger: logger,
	}
}

func (g serversRegistryHandlerFabric) GameServerMapsReassigned(payload *events.ServerRegistryEventGSMapsReassignedPayload) queue.Handler {
	for _, server := range payload.Servers {
		if server.ID == AssignedGameServerID && len(server.OldAssignedMapsToHandle) > 0 {
			newMaps := server.OnlyNewMaps()
			removedMaps := server.OnlyRemovedMaps()
			if len(removedMaps) > 0 || len(newMaps) > 0 {
				return eventsHandlerFunc(func() {
					if len(newMaps) > 0 && len(removedMaps) > 0 {
						r := C.CallOnMapsReassignedHook((*C.uint32_t)(&newMaps[0]), C.int(len(newMaps)), (*C.uint32_t)(&removedMaps[0]), C.int(len(removedMaps)))
						g.handleResponse(int(r), "GameServerMapsReassigned")
					} else if len(newMaps) > 0 {
						r := C.CallOnMapsReassignedHook((*C.uint32_t)(&newMaps[0]), C.int(len(newMaps)), (*C.uint32_t)(nil), C.int(len(removedMaps)))
						g.handleResponse(int(r), "GameServerMapsReassigned")
					} else {
						r := C.CallOnMapsReassignedHook((*C.uint32_t)(nil), C.int(len(newMaps)), (*C.uint32_t)(&removedMaps[0]), C.int(len(removedMaps)))
						g.handleResponse(int(r), "GameServerMapsReassigned")
					}
				})
			}
			return nil
		}
	}
	return nil
}

func (g serversRegistryHandlerFabric) handleResponse(resp int, hookName string) {
	const (
		CallStatusOk     = 0
		CallStatusNoHook = 1
	)
	switch resp {
	case CallStatusOk:
	case CallStatusNoHook:
		g.logger.Warn().Str("hook", hookName).Msg("no bound hook")
	default:
		g.logger.Error().Str("hook", hookName).Msg("unk status")
	}
}
