# Matchmaking Service

The matchmaking service owns the ephemeral cross-realm battleground and dungeon-finder queues. It creates matches and asks a worldserver to create the corresponding instance. The game gateway is its client and remains the source of truth for a connected player's queue intent.

## Availability model

This service is intentionally a single active in-memory matcher, matching the design agreed with the project maintainer. It is cluster-aware and restart-tolerant, but it is not an active/active horizontally sharded queue:

- Kubernetes (or another supervisor) restarts/reschedules the process.
- Every process start gets a new `instanceID`.
- Gateways retain each connected player's request ID, selections, and original queue timestamp.
- When the `instanceID` changes, gateways replay those requests with the original timestamp. Joins are idempotent by request ID.
- No queue database, distributed lease, or cross-node transaction is required.

Consequently a restart briefly pauses matching and disconnected players are not reconstructed. This is the deliberate POC trade-off; running multiple active replicas would require queue ownership/partitioning or durable coordination and is outside this first version.

## Dungeon finder POC

The initial policy is deliberately Blizzard-like:

- FIFO priority within a battlegroup, with a bounded candidate window.
- Parties remain atomic.
- A five-player group requires exactly one tank, one healer, and three damage roles. Multi-role players are assigned deterministically.
- Every entry must select the dungeon and every member must be eligible for it.

Dungeon eligibility is authoritative core data, not logic duplicated in the matcher. Before `JoinLFG`, the worldserver/core must evaluate the complete native rule set and send each member's `eligibleDungeonIDs`. This includes level and expansion restrictions, attunement/quest requirements, item requirements, lockouts, deserter/cooldowns, difficulty, and any future core rule. The matcher only intersects those sets and can never broaden them.

`MatchPolicy` is the narrow extension point for optional future PvE composition rules. The default remains `BlizzlikePolicy`; policy selection is intentionally not exposed as configuration in this POC.

The current proposal state proves queueing, eligibility, role composition, fairness, and restart replay. Proposal accept/decline, instance creation, teleport, and gateway packet wiring are the next vertical slice after maintainer review.

## Battleground policy

Battleground selection is FIFO by queue timestamp, bracket, and faction. `TeamAny` groups can fill either faction and parties are never split. There are no rating or composition rules in this first version. Selection lives behind the existing queue boundary so additional checks can be introduced without changing its callers.
