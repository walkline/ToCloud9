# Layering

ToCloud9 layering creates and removes logical copies of the open world in
response to player population. The servers registry computes desired capacity
from configured map/zone scopes, provisions missing layers, passively drains
underused layers, and deletes their workloads after their last player naturally
leaves.

A logical layer consists of one or more worldserver processes with the same
`LAYER_ID`. A single process may host every map, or several map-specific cores
may collectively host the layer. Kubernetes can schedule those processes on the
same physical node or on different nodes. A single AzerothCore/TrinityCore
process cannot host multiple isolated open-world layers.

## Configuration

Enable the feature on both the servers registry and every gateway. A YAML
configuration can contain:

```yaml
servers-registry:
  layering:
    enabled: true
    maxPopulation: 1000
    targetPopulationPercent: 90
    overflowMarginPercent: 10
    minCapacityPercent: 10
    minCapacityDurationSeconds: 300
    switchCooldownSeconds: 60
    maxSwitchesPerHour: 6
    minLayers: 1
    maxLayers: 10
    reconcileIntervalSeconds: 5
    scopes:
      - name: human-starting-area
        zoneIDs: [12]
        maxPopulation: 200
    provisioner:
      type: kubernetes
      namespace: tocloud9
      namePrefix: tc9
      baseDeployments:
        - tocloud9-gameserver-ek
        - tocloud9-gameserver-kalimdor

gateway:
  layering:
    enabled: true
    switchQueueSize: 32
    queueProcessIntervalMs: 250
```

The equivalent environment variables are:

```text
LAYERING_ENABLED=true
LAYER_MAX_POPULATION=1000
LAYER_TARGET_POPULATION_PERCENT=90
LAYER_OVERFLOW_MARGIN_PERCENT=10
LAYER_MIN_CAPACITY_PERCENT=10
LAYER_MIN_CAPACITY_DURATION_SECONDS=300
LAYER_SWITCH_COOLDOWN_SECONDS=60
LAYER_MAX_SWITCHES_PER_HOUR=6
LAYER_MIN_LAYERS=1
LAYER_MAX_LAYERS=10
LAYER_RECONCILE_INTERVAL_SECONDS=5
LAYER_SCOPE_ZONE_IDS=12
LAYER_SCOPE_MAX_POPULATION=200
LAYER_PROVISIONER_TYPE=kubernetes
LAYER_KUBERNETES_NAMESPACE=tocloud9
LAYER_BASE_DEPLOYMENTS=tocloud9-gameserver-ek,tocloud9-gameserver-kalimdor
LAYER_SWITCH_QUEUE_SIZE=32
LAYER_QUEUE_PROCESS_INTERVAL_MS=250
```

Each core/sidecar needs a layer ID. Cores with the same ID collectively host one
copy of the world; different IDs host separate copies:

```text
# First core deployment
LAYER_ID=1

# Dynamically cloned deployments receive this automatically
LAYER_ID=2
```

When layering is enabled, the registry also supports older C++ sidecars that
omit the registration field: it assigns them to the least-represented
operator-owned layer from `1..minLayers`. Explicit non-zero IDs always take
precedence. This fallback is intended for static compatibility deployments;
dynamically provisioned layers must use a sidecar that sends `LAYER_ID`.

Map ownership remains configured with the core's existing
`Cluster.AvailableMaps` option. For example, within each layer:

```ini
# Eastern Kingdoms core
Cluster.AvailableMaps="0"

# Kalimdor core
Cluster.AvailableMaps="1"
```

An empty value means the core can host any map. Map balancing now runs
independently for every layer, so each logical layer receives a complete map
set. A layer is eligible for a player only when one of its registered cores has
finished loading the player's map.

## Player movement

The system never rebalances a character merely because a population poll finds
an overfull or draining layer. Login, open-world map changes, and completion of
a taxi/controlled spline are placement points. Before processing any queued
move, the gateway waits until the character is alive, out of combat, has taken
no damage for 30 seconds, is not falling, looting, trading, casting, or
releasing, and is on an open-world continent map. Dungeon, raid, battleground,
and arena maps are never layered. When a player accepts a party invitation, the
gateway queues an explicit request to join the inviter's layer and leaves it
queued until all of those conditions are safe. The registry then authorizes the
move against the cooldown and rolling hourly limit.

An authorized same-map move uses the existing `TC9CMsgPrepareForRedirect`
handshake: the old core saves and detaches the character, then the gateway logs
the character into the destination core. For a layer-only move the gateway
roots the character briefly, consumes the transfer/loading opcodes, and sends
both world-port acknowledgements internally. The client remains on the same map
and therefore does not open a loading screen. It receives friendly start and
movement-resumed messages; core addresses are included only for GM accounts.

Gateway polling only refreshes online accounting and retries an already queued
explicit switch; it cannot initiate population balancing. A layer marked as
draining receives no new placement. Its workload stays alive while existing
players continue normally, and is deleted only after they log out or leave at a
safe transition. Gateway sessions that vanish without logging out expire from
lifecycle accounting after 30 seconds.

## Lifecycle example

With a maximum of 200, a 90% target, a 10% overflow margin, a 10% minimum
capacity, and a five-minute minimum-capacity duration:

1. Layer 1 accepts 180 players and reaching that target requests layer 2. The
   provisioner clones every configured base deployment with `LAYER_ID=2`.
2. While layer 2 starts, layer 1 may temporarily accept up to 220 players. Once
   the new core is ready, new/safely transitioning players are placed there;
   the existing layer-1 players are not moved in the background.
3. Reaching 360 players requests layer 3 using the same rule.
4. If the highest layer falls to 20 players or fewer for five continuous
   minutes while the remaining layers provide the target capacity, it is marked
   draining and stops receiving placements.
5. Those players are not force-moved. After they log out, change maps, finish a
   flight path, or otherwise leave safely, the empty layer workload is deleted.

Provisioning is asynchronous because a worldserver must start and load its maps
before accepting sessions. Players arriving during that startup window may use
only the configured overflow margin. At the hard cap, placement waits for a new
layer rather than silently overloading an existing one.

## Kubernetes provisioner

The provisioner clones the pod specification of every `baseDeployment`, while
replacing selectors, managed labels, and `LAYER_ID`. This preserves the base
deployment's images, database secrets, PVC mounts, world configuration, and map
ownership. Configure one base deployment when a core hosts all maps, or several
when maps are split across cores. The Helm chart creates the required
ServiceAccount, Role, and RoleBinding when `provisioner.type` is `kubernetes`.

Layer population, drain state, and switch history currently live in the single
servers-registry process. Keep `servers_registry.replicaCount: 1`; restarting
that process reconstructs core availability from Redis but resets population
and lifecycle timers.

## GM commands

The gateway handles the following commands for accounts with a non-zero
`account_access.gmlevel` in the current realm (or realm `-1`):

```text
.layer
.layer switch <number>
.layer switch <number> <playername>
```

`.layer` shows the configured limits and, for each registered layer, its
current player count, ready core count, drain state, and the GM's current
layer. The switch variants force the GM or named online player to a layer.
Forced GM moves bypass population, cooldown, and hourly switch limits, but are
rejected if the layer does not exist or has no ready core for the player's map.
