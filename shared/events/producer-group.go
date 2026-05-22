package events

import (
	"encoding/json"

	"github.com/nats-io/nats.go"
)

// GroupServiceProducer defines the interface for producing group events.
//
//go:generate mockery --name=GroupServiceProducer
type GroupServiceProducer interface {
	// InviteCreated publishes an event for an invite being created.
	InviteCreated(payload *GroupEventInviteCreatedPayload) error

	// InviteDeclined publishes an event for an invite being declined.
	InviteDeclined(payload *GroupEventInviteDeclinedPayload) error

	// GroupCreated publishes an event for a group being created.
	GroupCreated(payload *GroupEventGroupCreatedPayload) error

	// GroupMemberOnlineStatusChanged publishes an event for a change in group member's online status.
	GroupMemberOnlineStatusChanged(payload *GroupEventGroupMemberOnlineStatusChangedPayload) error

	// GroupMemberLeft publishes an event for a group member leaving.
	GroupMemberLeft(payload *GroupEventGroupMemberLeftPayload) error

	// GroupDisband publishes an event for a group being disbanded.
	GroupDisband(payload *GroupEventGroupDisbandPayload) error

	// MemberAdded publishes an event for a member being added to the group.
	MemberAdded(payload *GroupEventGroupMemberAddedPayload) error

	// LeaderChanged publishes an event for a change in group leader.
	LeaderChanged(payload *GroupEventGroupLeaderChangedPayload) error

	// LootTypeChanged publishes an event for a change in loot type.
	LootTypeChanged(payload *GroupEventGroupLootTypeChangedPayload) error

	// ConvertedToRaid publishes an event for a group being converted to a raid.
	ConvertedToRaid(payload *GroupEventGroupConvertedToRaidPayload) error

	// TargetIconUpdated publishes an event for an update of target icon.
	TargetIconUpdated(payload *GroupEventNewTargetIconPayload) error

	// GroupDifficultyChanged publishes an event for an update of dungeon or raid difficulty
	GroupDifficultyChanged(payload *GroupEventGroupDifficultyChangedPayload) error

	// SendChatMessage publishes an event for a new chat message in a group or raid.
	SendChatMessage(payload *GroupEventNewMessagePayload) error

	GroupReadyCheckStarted(payload *GroupEventReadyCheckStartedPayload) error
	GroupReadyCheckMemberState(payload *GroupEventReadyCheckMemberStatePayload) error
	GroupReadyCheckFinished(payload *GroupEventReadyCheckFinishedPayload) error
	GroupMemberSubGroupChanged(payload *GroupEventMemberSubGroupChangedPayload) error
	GroupMemberFlagsChanged(payload *GroupEventMemberFlagsChangedPayload) error
	GroupMemberStateChanged(payload *GroupEventMemberStateChangedPayload) error
	GroupMemberStatesChanged(payload *GroupEventMemberStatesChangedPayload) error
	GroupInstanceResetRequest(payload *GroupEventInstanceResetRequestPayload) error
	GroupInstanceBindExtensionRequest(payload *GroupEventInstanceBindExtensionRequestPayload) error
}

// groupServiceProducerNatsJSON implements the GroupServiceProducer interface using NATS as the underlying message broker.
type groupServiceProducerNatsJSON struct {
	conn *nats.Conn
	ver  string
}

// NewGroupServiceProducerNatsJSON creates a new instance of groupServiceProducerNatsJSON.
func NewGroupServiceProducerNatsJSON(conn *nats.Conn, ver string) GroupServiceProducer {
	return &groupServiceProducerNatsJSON{
		conn: conn,
		ver:  ver,
	}
}

func (s *groupServiceProducerNatsJSON) InviteCreated(payload *GroupEventInviteCreatedPayload) error {
	return s.publish(GroupEventInviteCreated, payload)
}

func (s *groupServiceProducerNatsJSON) InviteDeclined(payload *GroupEventInviteDeclinedPayload) error {
	return s.publish(GroupEventInviteDeclined, payload)
}

func (s *groupServiceProducerNatsJSON) GroupCreated(payload *GroupEventGroupCreatedPayload) error {
	return s.publish(GroupEventGroupCreated, payload)
}

func (s *groupServiceProducerNatsJSON) GroupMemberOnlineStatusChanged(payload *GroupEventGroupMemberOnlineStatusChangedPayload) error {
	return s.publish(GroupEventGroupMemberOnlineStatusChanged, payload)
}

func (s *groupServiceProducerNatsJSON) GroupMemberLeft(payload *GroupEventGroupMemberLeftPayload) error {
	return s.publish(GroupEventGroupMemberLeft, payload)
}

func (s *groupServiceProducerNatsJSON) GroupDisband(payload *GroupEventGroupDisbandPayload) error {
	return s.publish(GroupEventGroupDisband, payload)
}

func (s *groupServiceProducerNatsJSON) MemberAdded(payload *GroupEventGroupMemberAddedPayload) error {
	return s.publish(GroupEventGroupMemberAdded, payload)
}

func (s *groupServiceProducerNatsJSON) LeaderChanged(payload *GroupEventGroupLeaderChangedPayload) error {
	return s.publish(GroupEventGroupLeaderChanged, payload)
}

func (s *groupServiceProducerNatsJSON) LootTypeChanged(payload *GroupEventGroupLootTypeChangedPayload) error {
	return s.publish(GroupEventGroupLootTypeChanged, payload)
}

func (s *groupServiceProducerNatsJSON) ConvertedToRaid(payload *GroupEventGroupConvertedToRaidPayload) error {
	return s.publish(GroupEventGroupConvertedToRaid, payload)
}

func (s *groupServiceProducerNatsJSON) SendChatMessage(payload *GroupEventNewMessagePayload) error {
	return s.publish(GroupEventNewChatMessage, payload)
}

func (s *groupServiceProducerNatsJSON) TargetIconUpdated(payload *GroupEventNewTargetIconPayload) error {
	return s.publish(GroupEventNewTargetIcon, payload)
}

func (s *groupServiceProducerNatsJSON) GroupDifficultyChanged(payload *GroupEventGroupDifficultyChangedPayload) error {
	return s.publish(GroupEventGroupDifficultyChanged, payload)
}

func (s *groupServiceProducerNatsJSON) publish(e GroupServiceEvent, payload interface{}) error {
	msg := EventToSendGenericPayload{
		Version:   s.ver,
		EventType: int(e),
		Payload:   payload,
	}

	d, err := json.Marshal(&msg)
	if err != nil {
		return err
	}

	return s.conn.Publish(e.SubjectName(), d)
}

func (s *groupServiceProducerNatsJSON) GroupReadyCheckStarted(payload *GroupEventReadyCheckStartedPayload) error {
	return s.publish(GroupEventGroupReadyCheckStarted, payload)
}

func (s *groupServiceProducerNatsJSON) GroupReadyCheckMemberState(payload *GroupEventReadyCheckMemberStatePayload) error {
	return s.publish(GroupEventGroupReadyCheckMemberState, payload)
}

func (s *groupServiceProducerNatsJSON) GroupReadyCheckFinished(payload *GroupEventReadyCheckFinishedPayload) error {
	return s.publish(GroupEventGroupReadyCheckFinished, payload)
}

func (s *groupServiceProducerNatsJSON) GroupMemberSubGroupChanged(payload *GroupEventMemberSubGroupChangedPayload) error {
	return s.publish(GroupEventGroupMemberSubGroupChanged, payload)
}

func (s *groupServiceProducerNatsJSON) GroupMemberFlagsChanged(payload *GroupEventMemberFlagsChangedPayload) error {
	return s.publish(GroupEventGroupMemberFlagsChanged, payload)
}

func (s *groupServiceProducerNatsJSON) GroupMemberStateChanged(payload *GroupEventMemberStateChangedPayload) error {
	return s.publish(GroupEventGroupMemberStateChanged, payload)
}

func (s *groupServiceProducerNatsJSON) GroupMemberStatesChanged(payload *GroupEventMemberStatesChangedPayload) error {
	return s.publish(GroupEventGroupMemberStatesChanged, payload)
}

func (s *groupServiceProducerNatsJSON) GroupInstanceResetRequest(payload *GroupEventInstanceResetRequestPayload) error {
	return s.publish(GroupEventGroupInstanceResetRequest, payload)
}

func (s *groupServiceProducerNatsJSON) GroupInstanceBindExtensionRequest(payload *GroupEventInstanceBindExtensionRequestPayload) error {
	return s.publish(GroupEventGroupInstanceBindExtensionRequest, payload)
}
