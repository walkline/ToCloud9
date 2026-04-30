# libsidecar-cpp

C++ implementation of libsidecar - a drop-in replacement for the Go version.

## Why

The Go version works but has CGO overhead and complex build requirements. This C++ version:
- Native C++ library, no CGO
- 100% API-compatible with Go version
- Same metric names, same behavior
- Zero code changes required in AzerothCore

## Build

```bash
mkdir build && cd build
cmake ..
cmake --build .
```

Output: `build/libsidecar.dylib` (or `.so` on Linux)

## Requirements

- CMake 3.15+
- C++17 compiler
- Dependencies fetched automatically: gRPC, Prometheus, NATS, spdlog

## Integration with AzerothCore

1. **Copy library and headers:**
```bash
cp build/libsidecar.dylib /path/to/azerothcore/deps/libsidecar/lib/
cp include/*.h /path/to/azerothcore/deps/libsidecar/include/
```

2. **Set environment variables:**
```conf
TC9_GUID_PROVIDER_ADDRESS=localhost:8996
TC9_SERVERS_REGISTRY_ADDRESS=localhost:8999
TC9_MATCHMAKING_ADDRESS=localhost:8994
TC9_NATS_URL=nats://localhost:4222
TC9_GRPC_PORT=57559
TC9_HEALTH_CHECK_PORT=57556
```

3. **Rebuild AzerothCore** - it will link against the C++ library instead of Go.

## API

All functions from Go version are implemented with TC9 prefix:

```c
void TC9InitLib(uint16_t port, uint32_t realmID, uint8_t isCrossRealm, 
                char* availableMaps, uint32_t** assignedMaps, int* assignedMapsSize);
void TC9ProcessGRPCOrHTTPRequests();  // Call on game loop
void TC9ProcessEventsHooks();          // Call on game loop
void TC9GracefulShutdown();

// GUID generation
uint64_t TC9GetNextAvailableCharacterGuid(int realmID);
uint64_t TC9GetNextAvailableItemGuid(int realmID);
uint64_t TC9GetNextAvailableInstanceGuid(int realmID);

// Handler registration - call during initialization
void TC9SetCanPlayerInteractWithGOAndTypeHandler(CanPlayerInteractWithGOAndTypeHandler h);
void TC9SetMonitoringDataCollectorHandler(MonitoringDataCollectorHandler h);
// ... and 9 more handler setters
```

## Monitoring

Metrics exposed at `http://localhost:57556/metrics` (Prometheus format):
- `active_connections`
- `delay_mean`, `delay_median`, `delay_95_percentile`, `delay_99_percentile`, `delay_max`

Health checks: `http://localhost:57556/healthcheck`
