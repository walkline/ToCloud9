package main

/*
#include "events-guild.h"
*/
import "C"
import (
	"github.com/rs/zerolog"

	"github.com/walkline/ToCloud9/game-server/libsidecar/consumer"
)

// TC9SetOnGuildMemberAddedHook sets hook for guild member added event.
//export TC9SetOnGuildMemberAddedHook
func TC9SetOnGuildMemberAddedHook(h C.OnGuildMemberAddedHook) {
	C.SetOnGuildMemberAddedHook(h)
}

// TC9SetOnGuildMemberRemovedHook sets hook for guild member removed (kicked) event.
//export TC9SetOnGuildMemberRemovedHook
func TC9SetOnGuildMemberRemovedHook(h C.OnGuildMemberRemovedHook) {
	C.SetOnGuildMemberRemovedHook(h)
}

// TC9SetOnGuildMemberLeftHook sets hook for guild member left event.
//export TC9SetOnGuildMemberLeftHook
func TC9SetOnGuildMemberLeftHook(h C.OnGuildMemberLeftHook) {
	C.SetOnGuildMemberLeftHook(h)
}

type eventsHandlerFunc func()

func (f eventsHandlerFunc) Handle() {
	f()
}

type guildHandlerFabric struct {
	logger zerolog.Logger
}

func NewGuildHandlerFabric(logger zerolog.Logger) consumer.GuildHandlersFabric {
	return &guildHandlerFabric{
		logger: logger,
	}
}

func (g guildHandlerFabric) GuildMemberAddedHandler(guildID, characterGUID uint64) consumer.Handler {
	return eventsHandlerFunc(func() {
		r := C.CallOnGuildMemberAddedHook(C.uint64_t(guildID), C.uint64_t(characterGUID))
		g.handleResponse(int(r), "GuildMemberAdded")
	})
}

func (g guildHandlerFabric) GuildMemberRemovedHandler(guildID, characterGUID uint64) consumer.Handler {
	return eventsHandlerFunc(func() {
		r := C.CallOnGuildMemberRemovedHook(C.uint64_t(guildID), C.uint64_t(characterGUID))
		g.handleResponse(int(r), "GuildMemberRemoved")
	})
}

func (g guildHandlerFabric) GuildMemberLeftHandler(guildID, characterGUID uint64) consumer.Handler {
	return eventsHandlerFunc(func() {
		r := C.CallOnGuildMemberLeftHook(C.uint64_t(guildID), C.uint64_t(characterGUID))
		g.handleResponse(int(r), "GuildMemberLeft")
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
