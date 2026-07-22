# Cluster-wide session ownership

The character service owns account admission. After validating the world-auth
proof, a gateway asks it to acquire `(realm ID, account ID)` before accepting
the session. A second login is rejected with the client's native
`AUTH_ALREADY_ONLINE` response; it never evicts the established session.

The gateway also passes the authenticated account ID when resolving the
selected character. The character service returns the character only when it
belongs to that account.

## Durable account ownership

The characters database stores one account lease per realm database. Its
primary key makes simultaneous claims from any number of gateway or character
service replicas mutually exclusive. Each account row contains the owning
gateway ID and a cryptographically random session token.

The database also stores one expiring liveness row per gateway. A gateway
refreshes that single row regardless of its player count. A graceful disconnect
deletes an account row only when its session token still matches. If a gateway
crashes, all account rows referencing it become reclaimable after its liveness
expires. Token-fenced release prevents cleanup from an old connection from
changing a newer owner's row.

The migration is:

```text
sql/characters/mysql/000005_create_account_session_locks.up.sql
sql/characters/mysql/000006_use_gateway_session_liveness.up.sql
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
- A gateway crash leaves only a bounded gateway-liveness interval before its
  account rows can be reclaimed.
- Database errors fail new authentication closed with `AUTH_UNAVAILABLE`.
- If a gateway cannot refresh liveness before it would expire, it terminates
  itself. This prevents its existing sessions from overlapping with accounts
  reclaimed by another gateway.
- Redis or NATS failure affects character fencing/recovery, not the account
  admission decision.

Steady-state database writes are one heartbeat and bounded stale-heartbeat
cleanup per gateway every one-third of `gatewayLivenessTTLSeconds`. Account
rows are written only at login, logout, and dead-gateway reclamation. Database
load therefore follows gateway count and session transitions, not connected
player count.
