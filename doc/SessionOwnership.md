# Cluster-wide session ownership

Gateways enforce one active session per realm/account and per
realm/character. The implementation is gateway-owned because only the gateway
has both the client socket and a cluster-wide view of authentication.

## State and takeover

Redis stores token-fenced owner records. An owner contains the gateway ID and a
cryptographically random session token. A normal logout deletes an owner only
when its token still matches, so cleanup from an older connection cannot delete
a newer connection's ownership.

A takeover atomically replaces the owner and appends an eviction event to the
previous gateway's Redis Stream. All keys involved in that transaction share a
realm-scoped Redis Cluster hash tag. The claimant verifies the owner again
before continuing, which prevents an intermediate claimant from succeeding
during concurrent logins.

NATS sends the same eviction as a low-latency fast path. Redis Streams are the
durable path: an eviction is still consumed if NATS delivery is interrupted.
Duplicate deliveries are deduplicated by eviction ID.

## Failure behaviour

- A gateway crash closes its client sockets. Stale owner records are harmless:
  the next claim replaces them using a new fencing token.
- Each gateway writes one expiring liveness heartbeat. A claimant waits for an
  eviction acknowledgement only while the previous gateway is considered live.
- A temporary Redis outage does not disconnect existing players because there
  are no per-session renewals. New claims fail closed until ownership can be
  established again.
- A temporary NATS outage falls back to the durable Redis eviction stream.

## Load model

Idle players generate no Redis traffic. Redis receives one heartbeat per
gateway every one-third of `gatewayLivenessTTLSeconds`, plus operations for
login, character selection, takeover and logout. Load therefore follows gateway
count and session transitions rather than the number of connected players.

Eviction work is bounded to 32 concurrent workers per gateway, and each Redis
Stream is approximately trimmed to 4096 events.

## Configuration

```yaml
gateway:
  redisUrl: redis://redis:6379/0
  gatewayLivenessTTLSeconds: 15
```

The liveness TTL must be at least 10 seconds. Redis and NATS high availability
are deployment concerns and can be provided independently of gateway scaling.
