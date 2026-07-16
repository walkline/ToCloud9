# Layering

ToCloud9 layering creates and removes logical copies of the open world in
response to player population. The servers registry computes desired capacity
from configured map/zone scopes, provisions missing layers, drains excess
layers, and deletes their workloads after every player has acknowledged a safe
redirect.

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
    switchCooldownSeconds: 60
    maxSwitchesPerHour: 6
    minLayers: 1
    maxLayers: 10
    reconcileIntervalSeconds: 5
    scaleDownDelaySeconds: 300
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
LAYER_SWITCH_COOLDOWN_SECONDS=60
LAYER_MAX_SWITCHES_PER_HOUR=6
LAYER_MIN_LAYERS=1
LAYER_MAX_LAYERS=10
LAYER_RECONCILE_INTERVAL_SECONDS=5
LAYER_SCALE_DOWN_DELAY_SECONDS=300
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

Initial login and map changes retain the player's assigned layer whenever that
layer can host the destination map. When a player accepts a party invitation,
the gateway queues a request to join the inviter's layer. The queue is processed
at `queueProcessIntervalMs`; the registry centrally authorizes the move against
the cooldown and rolling hourly limit.

An authorized same-map move uses the existing `TC9CMsgPrepareForRedirect`
handshake: the old core saves and detaches the character, then the gateway logs
the character into the destination core and completes the world-port
acknowledgement. No client patch or additional core redirect API is required.

Gateways poll lifecycle actions while the player is online. A layer marked as
draining no longer receives logins. Its players are queued onto lower layers,
and the registry keeps the old deployment alive until each redirect reports
success. Failed redirects are cleared and retried. Gateway sessions that vanish
without logging out expire from lifecycle accounting after 30 seconds.

## Lifecycle example

With the human starting area configured for 200 players per layer:

1. Layer 1 accepts the first 200 players.
2. The arrival of player 201 raises desired capacity to two layers. The
   provisioner clones every configured base deployment with `LAYER_ID=2`.
3. The arrival of player 401 raises desired capacity to three layers and creates
   the `LAYER_ID=3` deployment set.
4. When population falls to 300, desired capacity becomes two. After
   `scaleDownDelaySeconds`, layer 3 stops receiving players and enters draining.
5. Its remaining players are redirected to layers 1 and 2. Only after all moves
   complete does the provisioner delete every layer-3 deployment.

Provisioning is asynchronous because a worldserver must start and load its maps
before accepting sessions. Players arriving during that startup window remain
on an available layer; subsequent placement uses the new layer as soon as all
required map cores register.

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
