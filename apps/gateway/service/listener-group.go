package service

import (
	"github.com/nats-io/nats.go"

	eBroadcaster "github.com/walkline/ToCloud9/apps/gateway/events-broadcaster"
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
		events.WithGroupEventConsumerInviteDeclinedHandler(listener),
		events.WithGroupEventConsumerGroupCreatedHandler(listener),
		events.WithGroupEventConsumerGroupMemberOnlineStatusChangedHandler(listener),
		events.WithGroupEventConsumerGroupMemberLeftHandler(listener),
		events.WithGroupEventConsumerMemberAddedHandler(listener),
		events.WithGroupEventConsumerGroupDisbandHandler(listener),
		events.WithGroupEventConsumerConvertedToRaidHandler(listener),
		events.WithGroupEventConsumerLeaderChangedHandler(listener),
		events.WithGroupEventConsumerLootTypeChangedHandler(listener),
		events.WithGroupEventNewChatMessageHandler(listener),
		events.WithGroupEventNewTargetIconHandler(listener),
		events.WithGroupDifficultyChangedHandler(listener),
		events.WithGroupEventReadyCheckStartedHandler(listener),
		events.WithGroupEventReadyCheckMemberStateHandler(listener),
		events.WithGroupEventReadyCheckFinishedHandler(listener),
		events.WithGroupEventMemberSubGroupChangedHandler(listener),
		events.WithGroupEventMemberFlagsChangedHandler(listener),
		events.WithGroupEventMemberStateChangedHandler(listener),
		events.WithGroupEventMemberStatesChangedHandler(listener),
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

func (l *groupNatsListener) GroupInviteDeclinedEvent(payload *events.GroupEventInviteDeclinedPayload) error {
	l.broadcaster.NewGroupInviteDeclinedEvent(payload)
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

func (l *groupNatsListener) GroupLootTypeChangedEvent(payload *events.GroupEventGroupLootTypeChangedPayload) error {
	l.broadcaster.NewGroupLootTypeChangedEvent(payload)
	return nil
}

func (l *groupNatsListener) GroupChatMessageReceivedEvent(payload *events.GroupEventNewMessagePayload) error {
	l.broadcaster.NewGroupMessageEvent(payload)
	return nil
}

func (l *groupNatsListener) GroupTargetItemSetEvent(payload *events.GroupEventNewTargetIconPayload) error {
	l.broadcaster.NewGroupTargetIconEvent(payload)
	return nil
}

func (l *groupNatsListener) GroupDifficultyChangedEvent(payload *events.GroupEventGroupDifficultyChangedPayload) error {
	l.broadcaster.NewGroupDifficultyChangedEvent(payload)
	return nil
}

func (l *groupNatsListener) GroupReadyCheckStartedEvent(payload *events.GroupEventReadyCheckStartedPayload) error {
	l.broadcaster.NewGroupReadyCheckStartedEvent(payload)
	return nil
}

func (l *groupNatsListener) GroupReadyCheckMemberStateEvent(payload *events.GroupEventReadyCheckMemberStatePayload) error {
	l.broadcaster.NewGroupReadyCheckMemberStateEvent(payload)
	return nil
}

func (l *groupNatsListener) GroupReadyCheckFinishedEvent(payload *events.GroupEventReadyCheckFinishedPayload) error {
	l.broadcaster.NewGroupReadyCheckFinishedEvent(payload)
	return nil
}

func (l *groupNatsListener) GroupMemberSubGroupChangedEvent(payload *events.GroupEventMemberSubGroupChangedPayload) error {
	l.broadcaster.NewGroupMemberSubGroupChangedEvent(payload)
	return nil
}

func (l *groupNatsListener) GroupMemberFlagsChangedEvent(payload *events.GroupEventMemberFlagsChangedPayload) error {
	l.broadcaster.NewGroupMemberFlagsChangedEvent(payload)
	return nil
}

func (l *groupNatsListener) GroupMemberStateChangedEvent(payload *events.GroupEventMemberStateChangedPayload) error {
	l.broadcaster.NewGroupMemberStateChangedEvent(payload)
	return nil
}

func (l *groupNatsListener) GroupMemberStatesChangedEvent(payload *events.GroupEventMemberStatesChangedPayload) error {
	l.broadcaster.NewGroupMemberStatesChangedEvent(payload)
	return nil
}
