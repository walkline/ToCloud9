# Cluster-wide session ownership

The character service owns account admission. After validating the world-auth
proof, a gateway asks it to acquire `(realm ID, account ID)` before accepting
the session. A second login is rejected with the client's native
`AUTH_ALREADY_ONLINE` response; it never evicts the established session.

The gateway also passes the authenticated account ID when resolving the
selected character. The character service returns the character only when it
belongs to that account.

## Durable account lease

The characters database stores one account lease per realm database. Its
primary key makes simultaneous claims from any number of gateway or character
service replicas mutually exclusive. Each lease contains a cryptographically
random owner token and an expiry time.

Gateways renew their leases every 10 seconds. A graceful disconnect deletes a
lease only when its owner token still matches. If a gateway crashes, its lease
expires after 30 seconds and a replacement login can claim it. Token-fenced
renewal and release prevent cleanup from an old connection from changing a
newer's lease.

The migration is:

```text
sql/characters/mysql/000005_create_account_session_locks.up.sql
```

## Character ownership

Redis/NATS ownership remains limited to a selected character GUID. It protects
redirect and recovery flows from two gateways concurrently driving the same
character. Account admission does not use Redis and does not perform takeover.

## Failure behavior and scaling

- Gateway and character-service replicas keep no authoritative account state
  in RAM.
- Character-service failover is safe because account admission is serialized
  by the shared characters database.
- A gateway crash leaves only a bounded 30-second lease.
- Database errors fail new authentication closed with `AUTH_UNAVAILABLE`.
- A renewal error does not immediately disconnect an established player. If
  the lease is later found to belong to another token, that gateway closes its
  stale connection.
- Redis or NATS failure affects character fencing/recovery, not the account
  admission decision.
