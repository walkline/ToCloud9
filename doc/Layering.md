# Layering

ToCloud9 layering creates logical copies of the open world from already-running
game cores. The servers registry activates layers in ascending `layerId` order:
new players stay on the first layer until `maxPopulation` is reached, then they
are assigned to the next registered layer. If every layer is full, the least
populated layer is used so logins are not rejected while an external autoscaler
starts more cores.

The registry does not create Kubernetes pods or processes. Configure an HPA,
operator, or another deployment controller to start the cores; once a core
registers with a new `layerId`, it becomes available to the placement logic.

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
LAYER_SWITCH_QUEUE_SIZE=32
LAYER_QUEUE_PROCESS_INTERVAL_MS=250
```

Each core/sidecar needs a layer ID. Cores with the same ID collectively host one
copy of the world; different IDs host separate copies:

```text
# First core deployment
LAYER_ID=1

# Standby/second-layer deployment
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

Layer population and switch history currently live in the single servers-registry
process. Keep `servers_registry.replicaCount: 1`; restarting that process resets
those counters. Registered core discovery itself remains stored in Redis.
