package events

// Guild cache peer-coherence bus.
//
// Decoupled from client UI events (guild.member.*, guild.motd.*, …): those
// remain gateway-facing. These subjects exist only so multi-replica guildserver
// processes can drop stale in-memory roster caches after another replica
// commits a mutation to MySQL.
//
// Protocol (Sprint A — mark-stale invalidate):
//   1. Mutator commits MySQL + updates local cache.
//   2. Mutator publishes GuildCacheInvalidate (with ServiceID).
//   3. Peers skip self (ServiceID), then mark that guild dirty (no MySQL load).
//   4. Next roster read on a dirty guild reloads once from MySQL (singleflight).
// Bank money/items stay DB-centric. Bank authz uses a point query, not full reload.

const (
	// GuildCacheEventInvalidateSubject is the NATS subject for peer invalidation.
	GuildCacheEventInvalidateSubject = "guild.cache.invalidate"
)

// GuildCacheInvalidatePayload asks peer guildserver replicas to discard and
// re-hydrate a single guild's roster cache from MySQL.
type GuildCacheInvalidatePayload struct {
	// ServiceID of the publishing guildserver instance (skip self-echo).
	ServiceID string `json:"serviceID"`
	// RealmID of the characters DB realm.
	RealmID uint32 `json:"realmID"`
	// GuildID that was mutated.
	GuildID uint64 `json:"guildID"`
}
