package consumer

import (
	"github.com/walkline/ToCloud9/game-server/libsidecar/queue"
	"github.com/walkline/ToCloud9/shared/events"
)

type GuildHandlersFabric interface {
	GuildMemberAddedHandler(guildID, characterGUID uint64) queue.Handler
	GuildMemberRemovedHandler(guildID, characterGUID uint64) queue.Handler
	GuildMemberLeftHandler(guildID, characterGUID uint64) queue.Handler
}

type GroupHandlersFabric interface {
	GroupCreated(payload *events.GroupEventGroupCreatedPayload) queue.Handler
	GroupMemberAdded(payload *events.GroupEventGroupMemberAddedPayload) queue.Handler
	GroupMemberRemoved(payload *events.GroupEventGroupMemberLeftPayload) queue.Handler
	GroupLeaderChanged(payload *events.GroupEventGroupLeaderChangedPayload) queue.Handler
	GroupDisbanded(payload *events.GroupEventGroupDisbandPayload) queue.Handler
	GroupLootTypeChanged(payload *events.GroupEventGroupLootTypeChangedPayload) queue.Handler
	GroupDifficultyChanged(payload *events.GroupEventGroupDifficultyChangedPayload) queue.Handler
	GroupConvertedToRaid(payload *events.GroupEventGroupConvertedToRaidPayload) queue.Handler
	GroupReadyCheckStarted(payload *events.GroupEventReadyCheckStartedPayload) queue.Handler
	GroupReadyCheckMemberState(payload *events.GroupEventReadyCheckMemberStatePayload) queue.Handler
	GroupReadyCheckFinished(payload *events.GroupEventReadyCheckFinishedPayload) queue.Handler
	GroupMemberSubGroupChanged(payload *events.GroupEventMemberSubGroupChangedPayload) queue.Handler
	GroupMemberFlagsChanged(payload *events.GroupEventMemberFlagsChangedPayload) queue.Handler
	GroupMemberStateChanged(payload *events.GroupEventMemberStateChangedPayload) queue.Handler
	GroupInstanceResetRequest(payload *events.GroupEventInstanceResetRequestPayload) queue.Handler
	GroupInstanceBindExtensionRequest(payload *events.GroupEventInstanceBindExtensionRequestPayload) queue.Handler
}

type ServerRegistryHandlerFabric interface {
	GameServerMapsReassigned(payload *events.ServerRegistryEventGSMapsReassignedPayload) queue.Handler
}
