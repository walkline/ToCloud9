# Layering

Layering is a map-placement policy owned by the servers registry. It does not
introduce portal handling, area-trigger routing, an instance coordinator, or a
new player-state service.

The gateway supplies the player's realm, destination map and group ID through
the normal map-selection request. Existing `InterceptNewWorld` and
`InterceptMoveWorldPortAck` processing performs any required worldserver
redirect.

## Configuration and map assignment

Configure the number of copies required for individual maps at registry
startup:

```yaml
servers_registry:
  layering:
    maps:
      - mapID: 1
        layers: 2
      - mapID: 531
        layers: 2
```

The equivalent environment variable is `LAYER_MAPS=1:2,531:2`. Startup values
are stored in Redis. `UpdateMapLayerConfiguration` replaces the configuration
at runtime and triggers the existing map-redistribution workflow.

For a map configured with N copies, the registry assigns that map to N distinct
compatible gameservers. Compatibility still comes from
`AC_CLUSTER_AVAILABLE_MAPS`. A gameserver can host copies of several different
maps, but can host at most one copy of any particular map.

Gameservers do not register a global layer ID. Their normal gameserver ID is
the routing identity. The registry derives a deterministic, display-only alias
such as `thrall-onyxia-a1b2c3d4` from the server address. Aliases never replace
the gameserver ID in Redis bindings.

## Player and group placement

An ungrouped player is sent to the available gameserver for the requested map
with the fewest active connections. A grouped player uses this Redis binding:

```text
(realm ID, group ID, map ID) -> gameserver ID
```

The binding is created atomically, shared by every registry replica and
refreshed while active. If the owning gameserver is no longer available for the
map, an atomic compare-and-set moves the binding to the least-loaded eligible
server. When a group is created, the leader's gateway binds the new group to
the leader's current map and gameserver before applying group affinity.

Population is deliberately approximate and uses existing gameserver connection
metrics. The registry does not maintain an in-memory player directory.

## Instance binding ownership

Instances use the same existing TC9 map-transition flow; layering adds no
instance-specific RPC or Redis state. Dedicated instance cores are configured
by advertising the desired instance map IDs through
`AC_CLUSTER_AVAILABLE_MAPS`.

The AzerothCore image applies one narrowly scoped patch in
`game-server/azerothcore/patches`: `PlayerBindToInstance` persists a character
instance binding only when cluster mode is disabled or the current worldserver
owns the instance map. The in-memory bind is still created so the owning core's
normal gameplay logic is unchanged. The pinned TC9 AzerothCore fork already
applies the same ownership rule to instance saves and instance-script data.

This prevents a source worldserver from writing a binding while TC9 redirects
the player to the worldserver that actually owns the destination map. Native
AzerothCore remains responsible for instance IDs, saves, resets and lockouts.

## Test commands

- `.layer` shows the current map's configured copy count, available gameserver
  aliases and approximate populations.
- `.layer switch <gameserver-alias>` forces the current character to another
  gameserver assigned to the current map for testing.

The test command uses the ordinary redirect path. There is no visibility cache,
transition state machine, movement preservation, special recovery flow, portal
catalog or reset handoff.
