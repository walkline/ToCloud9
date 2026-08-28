# Account session ownership

ToCloud9 permits one authenticated gateway connection per account across all
realms. The coordinator runs inside the existing servers-registry process and
uses its existing Redis and NATS connections; it is not a separate service.

## Login takeover

Each gateway process has a random eviction identity and each game connection
has a random fencing token. Before sending
`SMSG_AUTH_RESPONSE`, a gateway asks servers-registry to claim the account.
Servers-registry keeps the previous owner authoritative while it asks that
exact process, through NATS request/reply, to close the client and world
connection. Reused health-registry gateway IDs therefore cannot let another
process acknowledge an eviction it does not own.
Only after the previous session reports complete teardown does an atomic Redis
compare-and-set assign the new owner.

A failed or timed-out takeover leaves the previous owner unchanged. Releases
also compare the full realm, gateway and token value, so cleanup from an older
connection cannot delete a newer owner. Health-registry absence is deliberately
not treated as a fencing signal: an isolated gateway may still serve its client.

Character uniqueness follows from account uniqueness: a character belongs to
one account, the gateway verifies that relationship again at the final GUID
lookup before world login, and a session rejects a second player-login opcode
while already logged in.

## Failure behavior

- If Redis or servers-registry is unavailable, new authentication fails closed
  while existing players remain connected.
- A gateway that loses NATS disconnects its local sessions and terminates.
- If eviction delivery or acknowledgement is lost, the new login fails and can
  be retried. The previous owner remains recorded until teardown is confirmed.
- If a gateway crashes before releasing its token, the account remains safely
  locked until operators have stopped/fenced all gateways and removed the
  orphaned owner key.
- On `SIGTERM`/`SIGINT`, a gateway stops accepting clients, closes and joins all
  active game sessions, and releases their owner tokens before closing NATS.
  Forced termination (`SIGKILL`, node loss, or grace-period exhaustion) follows
  the crash recovery procedure above.
- Redis ownership data must be persistent. Any Redis restart, failover, restore,
  rollback, or other event that may lose an acknowledged write requires closing
  login traffic, stopping/fencing every gateway, and then restarting
  servers-registry and the gateways before reopening login. The running
  coordinator detects a changed Redis process identity and fails new claims
  closed, but a simultaneous control-plane restart cannot prove write
  continuity. This coordinated recovery runbook is the deliberate operational
  tradeoff that keeps ownership centralized and avoids per-gateway Redis leases,
  streams and epoch machinery.
