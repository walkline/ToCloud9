package events

import (
	"github.com/nats-io/nats.go"
	"github.com/rs/zerolog/log"
)

// GuildCacheInvalidateHandler receives peer cache invalidation notices.
// Implementations must ignore events published by the local ServiceID.
type GuildCacheInvalidateHandler interface {
	HandleGuildCacheInvalidate(payload GuildCacheInvalidatePayload) error
}

// GuildCacheConsumer listens for peer-cache coherence messages.
type GuildCacheConsumer interface {
	Listen() error
}

type guildCacheConsumer struct {
	nc      *nats.Conn
	handler GuildCacheInvalidateHandler
}

// NewGuildCacheConsumer builds a consumer for guild.cache.* peer events.
func NewGuildCacheConsumer(nc *nats.Conn, handler GuildCacheInvalidateHandler) GuildCacheConsumer {
	return &guildCacheConsumer{nc: nc, handler: handler}
}

func (c *guildCacheConsumer) Listen() error {
	if c.handler == nil {
		return nil
	}
	// Not a queue group: every replica must observe every invalidation.
	_, err := c.nc.Subscribe(GuildCacheEventInvalidateSubject, func(msg *nats.Msg) {
		var payload GuildCacheInvalidatePayload
		if _, err := Unmarshal(msg.Data, &payload); err != nil {
			log.Error().Err(err).Msg("guild cache invalidate: unmarshal failed")
			return
		}
		if err := c.handler.HandleGuildCacheInvalidate(payload); err != nil {
			log.Error().Err(err).
				Uint32("realmID", payload.RealmID).
				Uint64("guildID", payload.GuildID).
				Msg("guild cache invalidate: apply failed")
		}
	})
	return err
}
