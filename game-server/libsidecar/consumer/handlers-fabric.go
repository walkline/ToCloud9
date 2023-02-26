package consumer

type Handler interface {
	Handle()
}

type GuildHandlersFabric interface {
	GuildMemberAddedHandler(guildID, characterGUID uint64) Handler
	GuildMemberRemovedHandler(guildID, characterGUID uint64) Handler
	GuildMemberLeftHandler(guildID, characterGUID uint64) Handler
}
