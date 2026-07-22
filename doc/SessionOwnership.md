# Character login locks

The character service owns cluster-wide login admission. Authentication and
character enumeration remain unchanged. When the client sends
`CMSG_PLAYER_LOGIN`, the gateway calls `AcquireCharacterLoginLock` with the
authenticated account ID, selected character GUID, realm ID and registry
gateway ID.

The character service first verifies that the character belongs to the
authenticated account. It then inserts a lock into the auth database. The
primary key on `(realm_id, account_id)` permits one active character per account
and a unique key on `(realm_id, character_guid)` prevents duplicate character
sessions. Concurrent requests are serialized by MySQL's unique constraints.

The schema is created by:

```text
sql/auth/mysql/000002_create_character_login_locks.up.sql
```

## Releasing locks

- Normal logout and client disconnect already publish
  `GWEventCharacterLoggedOut`, including account, character, realm and gateway.
  Character service processing deletes the matching login lock.
- If world connection fails after acquisition, the gateway publishes the same
  logout event to release the provisional lock.
- When the servers registry removes an unhealthy gateway, the character
  service deletes every login lock associated with that gateway before
  processing its existing online-character cleanup.

The gateway holds no login ownership state in Redis or RAM. Character-service
replicas share the auth database, so any replica can acquire or release a lock.
The gateway-offline event is handled by the existing character-service NATS
queue group and the resulting database cleanup is idempotent.

## Event sanitation

The current project uses non-durable NATS events for gateway and character
lifecycle updates. If every character-service replica is unavailable when a
gateway-offline event is emitted, any affected lifecycle state can become
stale. The project treats reconciliation/sanitation of missed lifecycle events
as a generic concern rather than part of character login locking.
