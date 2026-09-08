#ifndef __LIBSIDECAR_H__
#define __LIBSIDECAR_H__

#include <stdint.h>
#include <stdbool.h>

/* Export/import decoration for Windows DLL */
#ifdef _WIN32
    #ifdef TC9_BUILDING_DLL
        #define TC9_API __declspec(dllexport)
    #else
        #define TC9_API __declspec(dllimport)
    #endif
#else
    #ifdef TC9_BUILDING_DLL
        #define TC9_API __attribute__((visibility("default")))
    #else
        #define TC9_API
    #endif
#endif

/* Compile-time version macros (generated from CMake project VERSION) */
#include "tc9_version.h"

/* Include all API headers */
#include "battleground-api.h"
#include "events-group.h"
#include "events-guild.h"
#include "events-servers-registry.h"
#include "monitoring.h"
#include "player-interactions-api.h"
#include "player-items-api.h"
#include "player-money-api.h"

#ifdef __cplusplus
extern "C" {
#endif

/* ABI / package version of the *linked* library (not the caller's headers).
 * Pass TC9_VERSION_MAJOR / TC9_VERSION_MINOR from the headers you compiled
 * against into TC9CheckAbiCompatible. Returns 0 if compatible. */
TC9_API void TC9GetVersion(int* major, int* minor, int* patch);
TC9_API const char* TC9GetVersionString(void);
TC9_API int TC9CheckAbiCompatible(int required_major, int required_minor);

/* Main library functions */
TC9_API void TC9InitLib(uint16_t port, uint32_t realmID, uint8_t isCrossRealm, char* availableMaps, uint32_t** assignedMaps, int* assignedMapsSize);
TC9_API void TC9GracefulShutdown();
TC9_API void TC9ProcessGRPCOrHTTPRequests();
TC9_API void TC9ProcessEventsHooks();

/* GUID generation */
TC9_API uint64_t TC9GetNextAvailableCharacterGuid(int realmID);
TC9_API uint64_t TC9GetNextAvailableItemGuid(int realmID);
TC9_API uint64_t TC9GetNextAvailableInstanceGuid(int realmID);

/* Map loading notification */
TC9_API void TC9ReadyToAcceptPlayersFromMaps(uint32_t* maps, int mapsLen);

/* Generic NATS pub/sub. Payloads are opaque bytes, subjects are arbitrary.
 * Subscription callbacks run on the thread draining TC9ProcessEventsHooks
 * (the world update thread), not on the NATS delivery thread. Both return
 * 0 on success, -1 on failure.
 *
 * Example — a mod broadcasting and consuming its own events:
 *
 *   // Publish (any thread):
 *   const char msg[] = "{\"zone\":1519,\"boss\":466}";
 *   TC9NatsPublish("mymod.boss.spawned", msg, sizeof(msg) - 1);
 *
 *   // Subscribe once at startup; the handler runs on the world update
 *   // thread, so it is safe to touch game state from it:
 *   void OnBossSpawned(const char* subject, const char* payload, int payloadLen)
 *   {
 *       std::string data(payload, payloadLen);  // payload is not NUL-terminated
 *       // ... react to the event ...
 *   }
 *   TC9NatsSubscribe("mymod.boss.spawned", &OnBossSpawned);
 */
typedef void (*TC9NatsMessageHandler)(const char* subject, const char* payload, int payloadLen);
TC9_API int TC9NatsPublish(const char* subject, const char* payload, int payloadLen);
TC9_API int TC9NatsSubscribe(const char* subject, TC9NatsMessageHandler handler);

/* Matchmaking notifications */
TC9_API void TC9PlayerLeftBattleground(uint64_t playerGUID, uint32_t realmID, uint32_t instanceID);
TC9_API void TC9BattlegroundStatusChanged(uint32_t instanceID, uint8_t status);

/* Enqueue a solo in-process player into a battleground queue — the same RPC
 * the gateway issues for real players (pvpTeamID: 1 alliance, 2 horde).
 * Blocking gRPC call, do not call from map update threads. 0 on success. */
TC9_API int TC9EnqueueLocalPlayerToBattleground(uint64_t playerGUID, uint32_t playerLvl,
    uint32_t bgTypeID, uint32_t pvpTeamID);

/* Group enqueue: leader plus party members in ONE queue entry, so a whole
 * group fill pops one full battleground instead of the matchmaking creating
 * the match mid-batch. memberGUIDs lists the OTHER party members only — the
 * leader travels as leaderGUID and must not be repeated, exactly like the
 * gateway's PartyMembers field. Blocking gRPC call. 0 on success. */
TC9_API int TC9EnqueueLocalGroupToBattleground(uint64_t leaderGUID, uint32_t leaderLvl,
    uint32_t bgTypeID, uint32_t pvpTeamID, const uint64_t* memberGUIDs, int memberCount);

/* Query the queue slot assigned to an invited player (matchmaking owns the BG
 * queues cluster-wide; in-process sessions have no gateway to accept invites).
 * outIsAssignedToThisServer is 1 when the assigned battleground runs on THIS
 * worldserver. Blocking gRPC call, do not call from map update threads.
 * 0 on success, -1 on error or when no battleground is assigned yet. */
TC9_API int TC9BattlegroundQueueDataForLocalPlayer(uint64_t playerGUID, uint32_t* outBgTypeID,
    uint32_t* outInstanceID, uint32_t* outMapID, int* outIsAssignedToThisServer);

/* Confirm to matchmaking that an in-process player entered the battleground
 * (the gateway does this for real players after AddPlayersToBattleground).
 * Blocking gRPC call, do not call from map update threads. 0 on success. */
TC9_API int TC9PlayerJoinedBattleground(uint64_t playerGUID, uint32_t instanceID);

/* Remove a player from a battleground queue (leftover in-process enqueues
 * that were never invited, or out-of-bracket after a level up). Blocking
 * gRPC call, do not call from map update threads. 0 on success. */
TC9_API int TC9RemovePlayerFromBattlegroundQueue(uint64_t playerGUID, uint32_t bgTypeID);

/* Event hooks registration */
TC9_API void TC9SetOnGroupCreatedHook(OnGroupCreatedHook h);
TC9_API void TC9SetOnGroupMemberAddedHook(OnGroupMemberAddedHook h);
TC9_API void TC9SetOnGroupMemberRemovedHook(OnGroupMemberRemovedHook h);
TC9_API void TC9SetOnGroupDisbandedHook(OnGroupDisbandedHook h);
TC9_API void TC9SetOnGroupLootTypeChangedHook(OnGroupLootTypeChangedHook h);
TC9_API void TC9SetOnGroupDungeonDifficultyChangedHook(OnGroupDungeonDifficultyChangedHook h);
TC9_API void TC9SetOnGroupRaidDifficultyChangedHook(OnGroupRaidDifficultyChangedHook h);
TC9_API void TC9SetOnGroupConvertedToRaidHook(OnGroupConvertedToRaidHook h);

TC9_API void TC9SetOnGuildMemberAddedHook(OnGuildMemberAddedHook h);
TC9_API void TC9SetOnGuildMemberRemovedHook(OnGuildMemberRemovedHook h);
TC9_API void TC9SetOnGuildMemberLeftHook(OnGuildMemberLeftHook h);

TC9_API void TC9SetOnMapsReassignedHook(OnMapsReassignedHook h);

/* Handler registration for gRPC requests */
TC9_API void TC9SetBattlegroundStartHandler(BattlegroundStartHandler h);
TC9_API void TC9SetBattlegroundAddPlayersHandler(BattlegroundAddPlayersHandler h);
TC9_API void TC9SetCanPlayerJoinBattlegroundQueueHandler(CanPlayerJoinBattlegroundQueueHandler h);
TC9_API void TC9SetCanPlayerTeleportToBattlegroundHandler(CanPlayerTeleportToBattlegroundHandler h);

TC9_API void TC9SetMonitoringDataCollectorHandler(MonitoringDataCollectorHandler h);

TC9_API void TC9SetCanPlayerInteractWithNPCAndFlagsHandler(CanPlayerInteractWithNPCAndFlagsHandler h);
TC9_API void TC9SetCanPlayerInteractWithGOAndTypeHandler(CanPlayerInteractWithGOAndTypeHandler h);

TC9_API void TC9SetGetPlayerItemsByGuidsHandler(GetPlayerItemsByGuidsHandler h);
TC9_API void TC9SetRemoveItemsWithGuidsFromPlayerHandler(RemoveItemsWithGuidsFromPlayerHandler h);
TC9_API void TC9SetAddExistingItemToPlayerHandler(AddExistingItemToPlayerHandler h);

TC9_API void TC9SetGetMoneyForPlayerHandler(GetMoneyForPlayerHandler h);
TC9_API void TC9SetModifyMoneyForPlayerHandler(ModifyMoneyForPlayerHandler h);

#ifdef __cplusplus
}
#endif

#endif /* __LIBSIDECAR_H__ */
