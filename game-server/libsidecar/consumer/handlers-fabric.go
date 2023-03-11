package consumer

import "github.com/walkline/ToCloud9/game-server/libsidecar/queue"

type GuildHandlersFabric interface {
	GuildMemberAddedHandler(guildID, characterGUID uint64) queue.Handler
	GuildMemberRemovedHandler(guildID, characterGUID uint64) queue.Handler
	GuildMemberLeftHandler(guildID, characterGUID uint64) queue.Handler
}
