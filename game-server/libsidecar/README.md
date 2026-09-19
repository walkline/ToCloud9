# libsidecar (Go) — DEPRECATED

> ⚠️ **This Go/cgo implementation is deprecated.** The maintained sidecar is
> [`game-server/libsidecar-cpp`](../libsidecar-cpp), which exposes the same C ABI
> (every export here has its homonym there, verified by
> `libsidecar-cpp/tests/c_api_compatibility_test.c`). The AzerothCore game-server
> Docker image and the `libsidecar-v*` release workflow already build only the
> C++ library; this directory is kept for reference and as a fallback
> (`make build-sidecar-go`) until it is removed.

## Migrating a deployment to libsidecar-cpp

The C++ library reads its configuration from **environment variables only**
(no YAML config file, `TC9_CONFIG_FILE` is not supported), and the variable
names differ:

| Go sidecar | libsidecar-cpp | default |
|---|---|---|
| `GRPC_PORT` | `TC9_GRPC_PORT` | `9501` |
| `HEALTH_CHECK_PORT` | `TC9_HEALTH_CHECK_PORT` | `8901` |
| `PREFERRED_HOSTNAME` | `TC9_PREFERRED_HOSTNAME` | *(auto)* |
| `SERVERS_REGISTRY_SERVICE_ADDRESS` | `TC9_SERVERS_REGISTRY_ADDRESS` | `localhost:8999` |
| `MATCHMAKING_SERVICE_ADDRESS` | `TC9_MATCHMAKING_ADDRESS` | `localhost:8994` |
| `GUID_PROVIDER_SERVICE_ADDRESS` | `TC9_GUID_PROVIDER_ADDRESS` | `localhost:8996` |
| `NATS_URL` | `TC9_NATS_URL` | `nats://localhost:4222` |
| `CHARACTER_GUIDS_BUFFER_SIZE` | `TC9_CHARACTER_GUIDS_BUFFER_SIZE` | `50` |
| `ITEM_GUIDS_BUFFER_SIZE` | `TC9_ITEM_GUIDS_BUFFER_SIZE` | `200` |
| `INSTANCE_GUIDS_BUFFER_SIZE` | `TC9_INSTANCE_GUIDS_BUFFER_SIZE` | `10` |
| — | `TC9_LOG_LEVEL` | `info` |
| — | `TC9_READ_THREADS` | `4` |
| — | `TC9_PARALLEL_READ_PROCESSING` | `0` |

The chart (`chart/templates/gameserver_ac.yaml`) and the Windows run guide
already use the `TC9_*` names.

## Building (fallback only)

```
make build-sidecar-go
```

The maintained path is `make build-sidecar`, which builds
`game-server/libsidecar-cpp` with CMake.
