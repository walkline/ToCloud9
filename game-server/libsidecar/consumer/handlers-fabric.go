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
	GroupDisbanded(payload *events.GroupEventGroupDisbandPayload) queue.Handler
	GroupLootTypeChanged(payload *events.GroupEventGroupLootTypeChangedPayload) queue.Handler
	GroupDifficultyChanged(payload *events.GroupEventGroupDifficultyChangedPayload) queue.Handler
	GroupConvertedToRaid(payload *events.GroupEventGroupConvertedToRaidPayload) queue.Handler
}
