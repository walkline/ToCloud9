package session

import (
	"context"

	eBroadcaster "github.com/walkline/ToCloud9/apps/game-load-balancer/events-broadcaster"
)

var EventsHandleMap = map[eBroadcaster.EventType]EventsHandlersQueue{
	eBroadcaster.EventTypeIncomingWhisper:     NewEventHandler("IncomingWhisper", (*GameSession).HandleEventIncomingWhisperMessage),
	eBroadcaster.EventTypeGuildInviteCreated:  NewEventHandler("GuildInviteCreated", (*GameSession).HandleEventGuildInviteCreated),
	eBroadcaster.EventTypeGuildMemberPromoted: NewEventHandler("GuildMemberPromoted", (*GameSession).HandleEventGuildMemberPromoted),
	eBroadcaster.EventTypeGuildMemberDemoted:  NewEventHandler("GuildMemberDemoted", (*GameSession).HandleEventGuildMemberDemoted),
	eBroadcaster.EventTypeGuildMOTDUpdated:    NewEventHandler("GuildMOTDUpdated", (*GameSession).HandleEventGuildMOTDUpdated),
	eBroadcaster.EventTypeGuildMemberAdded:    NewEventHandler("GuildMemberAdded", (*GameSession).HandleEventGuildMemberAdded),
	eBroadcaster.EventTypeGuildMemberLeft:     NewEventHandler("GuildMemberLeft", (*GameSession).HandleEventGuildMemberLeft),
	eBroadcaster.EventTypeGuildMemberKicked:   NewEventHandler("GuildMemberKicked", (*GameSession).HandleEventGuildMemberKicked),
	eBroadcaster.EventTypeGuildRankCreated:    NewEventHandler("GuildRankCreated", (*GameSession).HandleEventGuildRankCreated),
	eBroadcaster.EventTypeGuildRankUpdated:    NewEventHandler("GuildRankUpdated", (*GameSession).HandleEventGuildRankUpdated),
	eBroadcaster.EventTypeGuildRankDeleted:    NewEventHandler("GuildRankDeleted", (*GameSession).HandleEventGuildRankDeleted),
	eBroadcaster.EventTypeGuildNewMessage:     NewEventHandler("GuildNewMessage", (*GameSession).HandleEventGuildNewMessage),
	eBroadcaster.EventTypeIncomingMail:        NewEventHandler("IncomingMail", (*GameSession).HandleEventIncomingMail),
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
