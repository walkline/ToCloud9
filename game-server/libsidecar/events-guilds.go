package main

/*
#include <stdlib.h>
#include "events-guild.h"
*/
import "C"

import (
	"unsafe"

	"github.com/rs/zerolog"

	"github.com/walkline/ToCloud9/game-server/libsidecar/consumer"
	"github.com/walkline/ToCloud9/game-server/libsidecar/queue"
	"github.com/walkline/ToCloud9/shared/events"
)

// TC9SetOnGuildMemberAddedHook sets hook for guild member added event.
//
//export TC9SetOnGuildMemberAddedHook
func TC9SetOnGuildMemberAddedHook(h C.OnGuildMemberAddedHook) {
	C.SetOnGuildMemberAddedHook(h)
}

// TC9SetOnGuildMemberRemovedHook sets hook for guild member removed (kicked) event.
//
//export TC9SetOnGuildMemberRemovedHook
func TC9SetOnGuildMemberRemovedHook(h C.OnGuildMemberRemovedHook) {
	C.SetOnGuildMemberRemovedHook(h)
}

// TC9SetOnGuildMemberLeftHook sets hook for guild member left event.
//
//export TC9SetOnGuildMemberLeftHook
func TC9SetOnGuildMemberLeftHook(h C.OnGuildMemberLeftHook) {
	C.SetOnGuildMemberLeftHook(h)
}

// TC9SetOnGuildCreatedHook sets hook for guild created event.
//
//export TC9SetOnGuildCreatedHook
func TC9SetOnGuildCreatedHook(h C.OnGuildCreatedHook) {
	C.SetOnGuildCreatedHook(h)
}

type guildHandlerFabric struct {
	logger zerolog.Logger
}

func NewGuildHandlerFabric(logger zerolog.Logger) consumer.GuildHandlersFabric {
	return &guildHandlerFabric{
		logger: logger,
	}
}

func (g guildHandlerFabric) GuildMemberAddedHandler(guildID, characterGUID uint64) queue.Handler {
	return eventsHandlerFunc(func() {
		r := C.CallOnGuildMemberAddedHook(C.uint64_t(guildID), C.uint64_t(characterGUID))
		g.handleResponse(int(r), "GuildMemberAdded")
	})
}

func (g guildHandlerFabric) GuildMemberRemovedHandler(guildID, characterGUID uint64) queue.Handler {
	return eventsHandlerFunc(func() {
		r := C.CallOnGuildMemberRemovedHook(C.uint64_t(guildID), C.uint64_t(characterGUID))
		g.handleResponse(int(r), "GuildMemberRemoved")
	})
}

func (g guildHandlerFabric) GuildMemberLeftHandler(guildID, characterGUID uint64) queue.Handler {
	return eventsHandlerFunc(func() {
		r := C.CallOnGuildMemberLeftHook(C.uint64_t(guildID), C.uint64_t(characterGUID))
		g.handleResponse(int(r), "GuildMemberLeft")
	})
}

func (g guildHandlerFabric) GuildCreatedHandler(payload *events.GuildEventGuildCreatedPayload) queue.Handler {
	return eventsHandlerFunc(func() {
		cname := C.CString(payload.GuildName)
		defer C.free(unsafe.Pointer(cname))

		var cmembers *C.uint64_t
		if len(payload.MemberGUIDs) > 0 {
			cmembers = (*C.uint64_t)(C.malloc(C.size_t(len(payload.MemberGUIDs)) * C.size_t(unsafe.Sizeof(C.uint64_t(0)))))
			defer C.free(unsafe.Pointer(cmembers))
			for i, guid := range payload.MemberGUIDs {
				cguid := (*C.uint64_t)(unsafe.Pointer(uintptr(unsafe.Pointer(cmembers)) + uintptr(i)*unsafe.Sizeof(*cmembers)))
				*cguid = C.uint64_t(guid)
			}
		}

		r := C.CallOnGuildCreatedHook(
			C.uint64_t(payload.GuildID), cname, C.uint64_t(payload.LeaderGUID),
			cmembers, C.int(len(payload.MemberGUIDs)),
		)
		g.handleResponse(int(r), "GuildCreated")
	})
}

func (g guildHandlerFabric) handleResponse(resp int, hookName string) {
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
