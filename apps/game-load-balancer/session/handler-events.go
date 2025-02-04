package session

import (
	"context"

	eBroadcaster "github.com/walkline/ToCloud9/apps/game-load-balancer/events-broadcaster"
)

var EventsHandleMap = map[eBroadcaster.EventType]EventsHandlersQueue{
	eBroadcaster.EventTypeIncomingWhisper: NewEventHandler("IncomingWhisper", (*GameSession).HandleEventIncomingWhisperMessage),

	// Guild
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

	// Mail
	eBroadcaster.EventTypeIncomingMail: NewEventHandler("IncomingMail", (*GameSession).HandleEventIncomingMail),

	// Groups
	eBroadcaster.EventTypeGroupInviteCreated:             NewEventHandler("EventTypeGroupInviteCreated", (*GameSession).HandleEventGroupInviteCreated),
	eBroadcaster.EventTypeGroupCreated:                   NewEventHandler("EventTypeGroupCreated", (*GameSession).HandleEventGroupCreated),
	eBroadcaster.EventTypeGroupMemberOnlineStatusChanged: NewEventHandler("EventTypeGroupMemberOnlineStatusChanged", (*GameSession).HandleEventGroupMemberOnlineStatusChanged),
	eBroadcaster.EventTypeGroupMemberLeft:                NewEventHandler("EventTypeGroupMemberLeft", (*GameSession).HandleEventGroupMemberLeft),
	eBroadcaster.EventTypeGroupDisband:                   NewEventHandler("EventTypeGroupDisband", (*GameSession).HandleEventGroupDisband),
	eBroadcaster.EventTypeGroupMemberAdded:               NewEventHandler("EventTypeGroupMemberAdded", (*GameSession).HandleEventGroupMemberAdded),
	eBroadcaster.EventTypeGroupLeaderChanged:             NewEventHandler("EventTypeGroupLeaderChanged", (*GameSession).HandleEventGroupLeaderChanged),
	eBroadcaster.EventTypeGroupLootTypeChanged:           NewEventHandler("EventTypeGroupLootTypeChanged", (*GameSession).HandleEventGroupLootTypeChanged),
	eBroadcaster.EventTypeGroupConvertedToRaid:           NewEventHandler("EventTypeGroupConvertedToRaid", (*GameSession).HandleEventGroupConvertedToRaid),
	eBroadcaster.EventTypeGroupNewMessage:                NewEventHandler("EventTypeGroupNewMessage", (*GameSession).HandleEventGroupNewMessage),
	eBroadcaster.EventTypeGroupNewTargetIcon:             NewEventHandler("EventTypeGroupNewTargetIcon", (*GameSession).HandleEventGroupNewTargetIcon),
	eBroadcaster.EventTypeGroupDifficultyChanged:         NewEventHandler("EventTypeGroupDifficultyChanged", (*GameSession).HandleEventGroupDifficultyChanged),

	// Matchmaking
	eBroadcaster.EventTypeMMJoinedPVPQueue:           NewEventHandler("EventTypeMMJoinedPVPQueue", (*GameSession).HandleEventMMJoinedPVPQueue),
	eBroadcaster.EventTypeMMInvitedToBGOrArena:       NewEventHandler("EventTypeMMInvitedToBGOrArena", (*GameSession).HandleEventMMInvitedToBGOrArena),
	eBroadcaster.EventTypeMMInviteToBGOrArenaExpired: NewEventHandler("EventTypeMMInviteToBGOrArenaExpired", (*GameSession).HandleEventMMInviteToBGOrArenaExpired),
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
