# Layering

Layering splits a busy map across several worldservers so the realm can hold more
players without everyone sharing one overcrowded core.

To a player it still looks like the same zone (for example Kalimdor or Outland).
Under the hood you may be on a different copy of that map than someone else who
logged in around the same time.

## What players notice

Most of the time, nothing. You log in, travel, and play as usual.

You might notice that:

- A friend standing in the same place is not visible to you, or you cannot
  interact with them, until you are on the same layer.
- Joining a party or raid can move you to the layer your group already uses for
  that map, so you can see and play with them.
- Switching layers is a short world transition (similar to recovering after a
  worldserver restart), not a special UI or portal.

Battlegrounds and other instance content keep their normal behavior. Layering
is about open-world maps that you chose to run in multiple copies.

## Who ends up on which layer

**Solo players** go to the least loaded copy of the map they need.

**Grouped players** stay together on the same copy of each map. When the group
forms or someone joins, members are pulled onto that shared layer for the map
they are on. Login also respects the group, so party members do not log into
different copies of the same zone by accident.

If a layer’s worldserver goes away, the group is moved to another healthy copy
of that map when someone needs it again.

## Turning layers on

By default every map has a single layer. You only add more for maps that need
the capacity.

In the servers-registry config:

```yaml
servers-registry:
  layering:
    # map ID: how many copies
    maps:
      1: 2    # Kalimdor, two layers
      530: 2  # Outland, two layers
```

Or with the environment variable:

```text
LAYER_MAPS=1:2;530:2
```

You still need worldservers that advertise those maps (same as today with
`AC_CLUSTER_AVAILABLE_MAPS`). Each extra layer for a map needs another
worldserver that can host it. One worldserver never runs two layers of the same
map at once, but it can host different maps.

Friendly names like `red-onyxia-7k` are only for people and GM commands. They
do not change how the system stores or routes players.

## Checking and testing in game

With a character online:

- `.tc9 ws ls` lists the layer setup and each worldserver (address with alias).
- `.tc9 ws switch <alias-or-address>` moves your character to another layer of
  the current map (handy when testing group visibility or capacity).

## Adjusting layers at runtime (gRPC)

You can change how many layers a map has without restarting, through the
servers-registry service:

- **`GetMapLayerConfiguration`** — returns the current map → layer-count
  settings for a realm.
- **`UpdateMapLayerConfiguration`** — replaces those settings for a realm.
  After a successful update the registry reassigns maps across worldservers so
  the new counts take effect.

Request shape (proto field names):

```text
GetMapLayerConfiguration(api, realmID)
  → maps: [{ mapID, layerCount }, ...]

UpdateMapLayerConfiguration(api, realmID, maps: [{ mapID, layerCount }, ...])
```

Pass every map you want configured. Maps omitted from the update are no longer
treated as multi-layer (they fall back to a single copy). Layer counts must be
greater than zero. You still need enough worldservers advertising each map for
the new count to be satisfied.

To inspect load after a change, use **`GetLayerStats`** with a realm and map ID.
It returns how many layers are configured and, for each live copy, player count
and worldserver alias.

## What layering does not do

It is not a visibility filter, a second realm, or a new way to enter instances.
It only decides which worldserver runs each copy of a configured map and keeps
parties on the same copy when they share that map.
