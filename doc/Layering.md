# Layering

ToCloud9 layers individual maps across worldservers. A worldserver can host at
most one copy of a given map, but it may host copies of several different maps.
For example, with three all-map worldservers, map 1 can have three layers while
map 571 has two; ordinary maps continue through the normal weighted distributor.

The servers registry owns the authoritative `(realm, map, layer) -> worldserver`
assignment. AzerothCore still receives a simple list of maps to load because a
single process never hosts two copies of the same map.

## Service ownership

Layering policy belongs exclusively to the servers registry. It owns map-layer
configuration, map redistribution, population accounting, switch limits,
least-loaded placement, group bindings, forced placement, and pending switch
state. The gateway never chooses a layer or compares layer populations: it
supplies the player's realm, map, group, and current worldserver to the registry
and follows the returned placement.

The gateway contains only client-session transport required for a seamless
cross-core handoff. It observes whether disconnecting the current world session
is safe, carries out the registry's decision, filters the loading-screen/login
visual packets, removes objects belonging to the old core, and forwards the
latest movement state to the new core. These mechanics do not assign layers and
can be replaced without changing registry placement policy.

Worldservers and the AzerothCore sidecar remain layer-unaware. They register
their map capabilities and load the maps assigned by the existing registry
workflow; they do not implement population balancing or group affinity.

## Configuration

Configure fixed layer counts at registry startup:

```yaml
servers-registry:
  layering:
    enabled: true
    maps:
      - mapID: 1
        layers: 3
      - mapID: 571
        layers: 2
    enableKubernetesAutoscaling: false
```

The environment equivalent is:

```text
LAYERING_ENABLED=true
LAYER_MAPS=1:3,571:2
LAYER_ENABLE_KUBERNETES_AUTOSCALING=false
```

Maps omitted from the configuration have one copy and use normal map
distribution. Updating the configuration through `UpdateMapLayerConfiguration`
immediately recomputes map assignments. Newly assigned maps remain unavailable
for placement until their worldserver confirms that loading completed.

The related registry RPCs are:

```text
GetMapLayerConfiguration
UpdateMapLayerConfiguration
BindGroupToGameServer
```

## Placement and group affinity

Every placement request carries the player's group ID. For ungrouped players,
the registry selects the compatible worldserver with the fewest active players.
For grouped players it atomically maintains:

```text
(realm ID, group ID, map ID) -> worldserver
```

When a group is created, the leader's gateway binds the leader's current map and
worldserver. Every member then requests placement with the new group ID and is
redirected only when necessary. Bindings for later maps are created lazily on
the least-loaded compatible worldserver. If a bound core crashes or loses its
map assignment, the stale binding is discarded and recreated.

The registry is intentionally deployed with one replica, so binding creation
and replacement are serialized by its service lock. A future multi-replica
registry must move bindings to a transactional shared store.

## Runtime redistribution

A configuration update preserves existing map assignments where possible,
adds or removes duplicate assignments, publishes the normal map-assignment
events, and waits for new maps to load. Gateway polling detects when a player's
current core no longer hosts the map and performs the same seamless redirect
used for group affinity.

The handoff saves and detaches the character from the old core, consumes client
loading opcodes, acknowledges both world ports internally, clears objects from
the previous map copy, and attaches to the destination core without a loading
screen. Movement remains unfrozen; the newest movement update is buffered during
the socket gap and delivered after destination world entry.

Layer switches wait until the player is alive, outside combat for the configured
`gateway.layering.postCombatDelaySeconds` (15 seconds by default), outside
instances and battlegrounds, and not falling, looting, trading, casting, or
releasing. Combat is read from AzerothCore's player combat flag, so healing that
places the character in combat is handled even when the character takes no
damage.

## Optional Kubernetes autoscaling

Kubernetes core provisioning is an optional extension and is disabled by
default. It is active only when both settings are present:

```yaml
enableKubernetesAutoscaling: true
provisioner:
  type: kubernetes
  namespace: tocloud9
  baseDeployments: [cloud9-tocloud9-gameserver-ac]
```

When enabled, the controller ensures enough worldserver pods for the highest
configured map layer count. Reducing the configuration drains excess cores and
deletes controller-owned deployments only after their players leave. The Helm
chart creates the controller ServiceAccount and RBAC rules only when this flag
and the Kubernetes provisioner are both enabled.

## GM commands

```text
.layer
.layer switch <number>
.layer switch <number> <playername>
```

`.layer` reports per-map configured counts, the current map/layer, legacy
capacity statistics, and whether Kubernetes autoscaling is enabled. Switching
is relative to the target player's current map. Network addresses remain
visible only to GMs; normal players receive friendly server/layer labels.
