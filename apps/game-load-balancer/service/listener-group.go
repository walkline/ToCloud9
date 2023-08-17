package service

import (
	"github.com/nats-io/nats.go"

	eBroadcaster "github.com/walkline/ToCloud9/apps/game-load-balancer/events-broadcaster"
	"github.com/walkline/ToCloud9/shared/events"
)

type groupNatsListener struct {
	consumer    events.GroupEventsConsumer
	broadcaster eBroadcaster.Broadcaster
}

func NewGroupNatsListener(nc *nats.Conn, broadcaster eBroadcaster.Broadcaster) Listener {
	listener := &groupNatsListener{
		broadcaster: broadcaster,
	}
	listener.consumer = events.NewGroupEventsConsumer(
		nc,
		events.WithGroupEventConsumerInviteCreatedHandler(listener),
		events.WithGroupEventConsumerGroupCreatedHandler(listener),
		events.WithGroupEventConsumerGroupMemberOnlineStatusChangedHandler(listener),
		events.WithGroupEventConsumerGroupMemberLeftHandler(listener),
		events.WithGroupEventConsumerMemberAddedHandler(listener),
		events.WithGroupEventConsumerGroupDisbandHandler(listener),
		events.WithGroupEventConsumerConvertedToRaidHandler(listener),
		events.WithGroupEventConsumerLeaderChangedHandler(listener),
	)

	return listener
}

func (l *groupNatsListener) Listen() error {
	return l.consumer.Listen()
}

func (l *groupNatsListener) Stop() error {
	return l.consumer.Stop()
}

func (l *groupNatsListener) GroupInviteCreatedEvent(payload *events.GroupEventInviteCreatedPayload) error {
	l.broadcaster.NewGroupInviteCreatedEvent(payload)
	return nil
}

func (l *groupNatsListener) GroupCreatedEvent(payload *events.GroupEventGroupCreatedPayload) error {
	l.broadcaster.NewGroupCreatedEvent(payload)
	return nil
}

func (l *groupNatsListener) GroupMemberOnlineStatusChangedEvent(payload *events.GroupEventGroupMemberOnlineStatusChangedPayload) error {
	l.broadcaster.NewGroupMemberOnlineStatusChangedEvent(payload)
	return nil
}

func (l *groupNatsListener) GroupMemberLeftEvent(payload *events.GroupEventGroupMemberLeftPayload) error {
	l.broadcaster.NewGroupMemberLeftEvent(payload)
	return nil
}

func (l *groupNatsListener) GroupMemberAddedEvent(payload *events.GroupEventGroupMemberAddedPayload) error {
	l.broadcaster.NewGroupMemberAddedEvent(payload)
	return nil
}

func (l *groupNatsListener) GroupDisbandEvent(payload *events.GroupEventGroupDisbandPayload) error {
	l.broadcaster.NewGroupDisbandEvent(payload)
	return nil
}

func (l *groupNatsListener) GroupConvertedToRaidEvent(payload *events.GroupEventGroupConvertedToRaidPayload) error {
	l.broadcaster.NewGroupConvertedToRaidEvent(payload)
	return nil
}

func (l *groupNatsListener) GroupLeaderChangedEvent(payload *events.GroupEventGroupLeaderChangedPayload) error {
	l.broadcaster.NewGroupLeaderChangedEvent(payload)
	return nil
}
