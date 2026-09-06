package events

import (
	"encoding/json"

	"github.com/nats-io/nats.go"
)

// GuildCacheProducer publishes peer-cache coherence messages (not client UI events).
//
//go:generate mockery --name=GuildCacheProducer
type GuildCacheProducer interface {
	// InvalidateGuild notifies peer guildserver replicas that a guild's
	// in-memory roster may be stale after a successful mutation.
	InvalidateGuild(payload *GuildCacheInvalidatePayload) error
}

type guildCacheProducerNatsJSON struct {
	conn *nats.Conn
	ver  string
}

// NewGuildCacheProducerNatsJSON creates a NATS JSON producer for guild cache coherence.
func NewGuildCacheProducerNatsJSON(conn *nats.Conn, ver string) GuildCacheProducer {
	return &guildCacheProducerNatsJSON{conn: conn, ver: ver}
}

func (p *guildCacheProducerNatsJSON) InvalidateGuild(payload *GuildCacheInvalidatePayload) error {
	if payload == nil {
		return nil
	}
	msg := EventToSendGenericPayload{
		Version:   p.ver,
		EventType: 1, // single-type bus for now; reserved for future ops (snapshot/delta)
		Payload:   payload,
	}
	d, err := json.Marshal(&msg)
	if err != nil {
		return err
	}
	return p.conn.Publish(GuildCacheEventInvalidateSubject, d)
}

// NopGuildCacheProducer discards all cache coherence publishes (tests / single-replica).
type NopGuildCacheProducer struct{}

func (NopGuildCacheProducer) InvalidateGuild(*GuildCacheInvalidatePayload) error { return nil }
