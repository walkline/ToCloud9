# WoWSimClient

A simulation client / bot for AzerothCore (WoW 3.3.5a) with embedded pathfinding, behavior trees, and support for grind, dungeon, and custom Lua behaviors.

**Note:** This code was generated and iteratively refined with assistance from AI.

## Building

```bash
cd apps/wowsimclient
go build -o wowsimclient ./cmd/wowsimclient
```

For a debug build with more symbols:
```bash
go build -gcflags="all=-N -l" -o wowsimclient-debug ./cmd/wowsimclient
```

## Running

### Single Bot (CLI mode)

Basic grind bot:

```bash
./wowsimclient --mode cli \
  --username admin --password admin \
  --char-name MyBot --race 10 --class 2 \
  --bot-mode grind \
  --data-dir /path/to/your/ac-data \
  --delete-existing-chars
```

- `--race 10` = Blood Elf, `--class 2` = Paladin (adjust as needed).
- `--data-dir` points to a directory containing `mmaps/`, `maps/`, `vmaps/` (for embedded pathfinding).
- Use `--log-decisions-to-chat=false` to reduce in-game spam.
- See `scripts/grind.lua` for an example of custom Lua-driven behavior.

Other modes:
- `--bot-mode hogger` (simple questing example)
- `--bot-mode dungeon --dungeon ragefire_chasm`
- `--bot-mode lua --lua-script scripts/dungeon.lua`

### Node / Server mode (HTTP API for remote control)

```bash
./wowsimclient --mode node --listen :8888
```

Bots can then be launched remotely via the orchestrator (see below).

### Orchestrator (multi-bot load / test controller)

```bash
./wowsimclient orchestrator \
  --num-bots 5 \
  --nodes "127.0.0.1:8888,127.0.0.1:8889" \
  --account-prefix loadbot \
  --duration 10m \
  --data-dir /path/to/ac-data
```

- Prepares accounts in the auth DB.
- Distributes bots across node servers.
- Collects results after the duration.

Flags include spawn rate limiting, DB DSN, etc. (see `--help`).

## Pathfinding Data

The client supports two modes:

1. Embedded (recommended for simplicity):
   - Provide `--data-dir` containing:
     - `mmaps/` (required for pathfinding)
     - `maps/`, `vmaps/` (optional but recommended)

2. External gRPC pathfinding service:
   - Use `--pathfinding-addr host:port`

## Configuration Notes

- The client expects a running AzerothCore auth + world server.
- Character creation / deletion is supported via `--delete-existing-chars`.
- For multi-level terrain (buildings, caves, "second floor" areas), the client prefers navmesh poly heights from the path generator and guards against large upward snaps to avoid teleporting between levels.
- Lua scripts can be used for fully custom AI (see `scripts/`).

## Development / Debugging (historical)

During development, extensive logging was used to diagnose height snapping and multi-level issues in the Blood Elf starting zone (map 530). For production builds, most debug output has been removed to keep the binary clean and the code readable.

## License

This project is part of the ToCloud9 / AzerothCore tooling ecosystem. Use responsibly on your own servers.

## Acknowledgments

Special thanks to the AzerothCore community.

This implementation was created and refined with the help of AI coding assistance.
