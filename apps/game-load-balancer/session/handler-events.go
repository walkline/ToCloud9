package session

import (
	"context"

	eBroadcaster "github.com/walkline/ToCloud9/apps/game-load-balancer/events-broadcaster"
)

var EventsHandleMap = map[eBroadcaster.EventType]EventsHandlersQueue{
	eBroadcaster.EventTypeIncomingWhisper: NewEventHandler("IncomingWhisper", (*GameSession).HandleEventIncomingWhisperMessage),
}

type EventHandler func(*GameSession, context.Context, *eBroadcaster.Event) error

func NewEventHandler(name string, handlers ...EventHandler) EventsHandlersQueue {
	return EventsHandlersQueue{
		name:  name,
		queue: handlers,
	}
}

type EventsHandlersQueue struct {
	name  string
	queue []EventHandler
}

func (q *EventsHandlersQueue) Handle(ctx context.Context, session *GameSession, e *eBroadcaster.Event) error {
	var err error
	for i := range q.queue {
		err = q.queue[i](session, ctx, e)
		if err != nil {
			return err
		}
	}
	return nil
}
