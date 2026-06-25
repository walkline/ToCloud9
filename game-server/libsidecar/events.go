package main

import "C"

import (
	stdlog "log"
	"runtime/debug"

	nats "github.com/nats-io/nats.go"
	"github.com/rs/zerolog"

	"github.com/walkline/ToCloud9/game-server/libsidecar/consumer"
	"github.com/walkline/ToCloud9/game-server/libsidecar/queue"
)

var eventsHandlersQueue queue.HandlersQueue

// TC9ProcessEventsHooks calls all events hooks.
//
//export TC9ProcessEventsHooks
func TC9ProcessEventsHooks() {
	handler := eventsHandlersQueue.Pop()
	for handler != nil {
		handleEventHook(handler)
		handler = eventsHandlersQueue.Pop()
	}
}

func handleEventHook(handler queue.Handler) {
	defer func() {
		if r := recover(); r != nil {
			stdlog.Printf("TC9 event hook panic recovered: %v\n%s", r, debug.Stack())
		}
	}()

	handler.Handle()
}

func SetupEventsListener(nc *nats.Conn, realmID uint32, log zerolog.Logger) consumer.Consumer {
	eventsHandlersQueue = queue.NewHandlersFIFOQueue()
	natsConsumer := consumer.NewNatsEventsConsumer(
		nc,
		NewGuildHandlerFabric(log),
		NewGroupHandlerFabric(log),
		NewServerRegistryHandlerFabric(log),
		eventsHandlersQueue,
		realmID,
	)
	if err := natsConsumer.Start(); err != nil {
		log.Fatal().Err(err).Msg("can't start nats consumer")
	}

	return natsConsumer
}

type eventsHandlerFunc func()

func (f eventsHandlerFunc) Handle() {
	f()
}
