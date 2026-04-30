/*
 * Compatibility API - Bridges Go-style C API to internal C++ implementation
 * This file provides ABI compatibility with the Go version of libsidecar
 */

#include "libsidecar.h"
#include "events-guild.h"
#include "events-group.h"
#include "events-servers-registry.h"
#include "player-items-api.h"
#include "player-money-api.h"
#include "player-interactions-api.h"
#include "battleground-api.h"
#include "monitoring.h"

// Include internal headers
#include "libsidecar/tc9_types.h"
#include "libsidecar/tc9_events.h"
#include "events/event_hooks.h"


#include <cstring>
#include <spdlog/spdlog.h>

// Global state for compatibility layer
namespace {
    // Store registered handlers
    struct CompatHandlers {
        // Guild hooks
        OnGuildMemberAddedHook guild_member_added = nullptr;
        OnGuildMemberLeftHook guild_member_left = nullptr;
        OnGuildMemberRemovedHook guild_member_removed = nullptr;

        // Group hooks
        OnGroupCreatedHook group_created = nullptr;
        OnGroupMemberAddedHook group_member_added = nullptr;
        OnGroupMemberRemovedHook group_member_removed = nullptr;
        OnGroupDisbandedHook group_disbanded = nullptr;
        OnGroupLootTypeChangedHook group_loot_changed = nullptr;
        OnGroupDungeonDifficultyChangedHook group_dungeon_diff_changed = nullptr;
        OnGroupRaidDifficultyChangedHook group_raid_diff_changed = nullptr;
        OnGroupConvertedToRaidHook group_converted_raid = nullptr;

        // Registry hooks
        OnMapsReassignedHook maps_reassigned = nullptr;

        // Player API handlers
        GetPlayerItemsByGuidsHandler get_player_items = nullptr;
        RemoveItemsWithGuidsFromPlayerHandler remove_items = nullptr;
        AddExistingItemToPlayerHandler add_item = nullptr;
        GetMoneyForPlayerHandler get_money = nullptr;
        ModifyMoneyForPlayerHandler modify_money = nullptr;
        CanPlayerInteractWithNPCAndFlagsHandler interact_npc = nullptr;
        CanPlayerInteractWithGOAndTypeHandler interact_go = nullptr;

        // Battleground handlers
        BattlegroundStartHandler bg_start = nullptr;
        BattlegroundAddPlayersHandler bg_add_players = nullptr;
        CanPlayerJoinBattlegroundQueueHandler can_join_bg_queue = nullptr;
        CanPlayerTeleportToBattlegroundHandler can_teleport_bg = nullptr;

        // Monitoring handler
        MonitoringDataCollectorHandler monitoring_collector = nullptr;
    };

    CompatHandlers g_compat_handlers;
}

extern "C" {

// ============================================================================
// Main API (libsidecar.h)
// ============================================================================

// The old Go API has no TC9 prefix, so we provide non-prefixed wrappers
// These are for backwards compatibility with code that uses the old API

void InitLib(uint16_t port, uint32_t realmID, uint8_t isCrossRealm,
             char* availableMaps, uint32_t** assignedMaps, int* assignedMapsSize) {
    TC9InitLib(port, realmID, isCrossRealm, availableMaps, assignedMaps, assignedMapsSize);
}

void GracefulShutdown(void) {
    TC9GracefulShutdown();
}

void ProcessGRPCOrHTTPRequests(void) {
    TC9ProcessGRPCOrHTTPRequests();
}

void ProcessEventsHooks(void) {
    TC9ProcessEventsHooks();
}

uint64_t GetNextAvailableCharacterGuid(uint32_t realmID) {
    return TC9GetNextAvailableCharacterGuid(static_cast<int>(realmID));
}

uint64_t GetNextAvailableItemGuid(uint32_t realmID) {
    return TC9GetNextAvailableItemGuid(static_cast<int>(realmID));
}

uint64_t GetNextAvailableInstanceGuid(uint32_t realmID) {
    return TC9GetNextAvailableInstanceGuid(static_cast<int>(realmID));
}

void ReadyToAcceptPlayersFromMaps(uint32_t* maps, int mapsLen) {
    TC9ReadyToAcceptPlayersFromMaps(maps, mapsLen);
}

void PlayerLeftBattleground(uint64_t playerGUID, uint32_t realmID, uint32_t instanceID) {
    TC9PlayerLeftBattleground(playerGUID, realmID, instanceID);
}

void BattlegroundStatusChanged(uint32_t instanceID, uint8_t status) {
    TC9BattlegroundStatusChanged(instanceID, status);
}

// ============================================================================
// Guild Events (events-guild.h)
// ============================================================================

void SetOnGuildMemberAddedHook(OnGuildMemberAddedHook h) {
    g_compat_handlers.guild_member_added = h;
    TC9SetOnGuildMemberAddedHook(h);
}

int CallOnGuildMemberAddedHook(uint64_t guild_id, uint64_t player_guid) {
    if (g_compat_handlers.guild_member_added) {
        g_compat_handlers.guild_member_added(guild_id, player_guid);
        return GuildHookStatusOK;
    }
    return GuildHookStatusNoHook;
}

void SetOnGuildMemberLeftHook(OnGuildMemberLeftHook h) {
    g_compat_handlers.guild_member_left = h;
    TC9SetOnGuildMemberLeftHook(h);
}

int CallOnGuildMemberLeftHook(uint64_t guild_id, uint64_t player_guid) {
    if (g_compat_handlers.guild_member_left) {
        g_compat_handlers.guild_member_left(guild_id, player_guid);
        return GuildHookStatusOK;
    }
    return GuildHookStatusNoHook;
}

void SetOnGuildMemberRemovedHook(OnGuildMemberRemovedHook h) {
    g_compat_handlers.guild_member_removed = h;
    TC9SetOnGuildMemberRemovedHook(h);
}

int CallOnGuildMemberRemovedHook(uint64_t guild_id, uint64_t player_guid) {
    if (g_compat_handlers.guild_member_removed) {
        g_compat_handlers.guild_member_removed(guild_id, player_guid);
        return GuildHookStatusOK;
    }
    return GuildHookStatusNoHook;
}

// ============================================================================
// Group Events (events-group.h)
// ============================================================================

void SetOnGroupCreatedHook(OnGroupCreatedHook h) {
    g_compat_handlers.group_created = h;
    TC9SetOnGroupCreatedHook(h);
}

int CallOnGroupCreatedHook(EventObjectGroup* group) {
    if (g_compat_handlers.group_created) {
        g_compat_handlers.group_created(group);
        return GroupHookStatusOK;
    }
    return GroupHookStatusNoHook;
}

void SetOnGroupMemberAddedHook(OnGroupMemberAddedHook h) {
    g_compat_handlers.group_member_added = h;
    TC9SetOnGroupMemberAddedHook(h);
}

int CallOnGroupMemberAddedHook(uint32_t guid, uint64_t newMemberGuid) {
    if (g_compat_handlers.group_member_added) {
        g_compat_handlers.group_member_added(guid, newMemberGuid);
        return GroupHookStatusOK;
    }
    return GroupHookStatusNoHook;
}

void SetOnGroupMemberRemovedHook(OnGroupMemberRemovedHook h) {
    g_compat_handlers.group_member_removed = h;
    TC9SetOnGroupMemberRemovedHook(h);
}

int CallOnGroupMemberRemovedHook(uint32_t guid, uint64_t removedMemberGuid, uint64_t newLeaderGuid) {
    if (g_compat_handlers.group_member_removed) {
        g_compat_handlers.group_member_removed(guid, removedMemberGuid, newLeaderGuid);
        return GroupHookStatusOK;
    }
    return GroupHookStatusNoHook;
}

void SetOnGroupDisbandedHook(OnGroupDisbandedHook h) {
    g_compat_handlers.group_disbanded = h;
    TC9SetOnGroupDisbandedHook(h);
}

int CallOnGroupDisbandedHook(uint32_t guid) {
    if (g_compat_handlers.group_disbanded) {
        g_compat_handlers.group_disbanded(guid);
        return GroupHookStatusOK;
    }
    return GroupHookStatusNoHook;
}

void SetOnGroupLootTypeChangedHook(OnGroupLootTypeChangedHook h) {
    g_compat_handlers.group_loot_changed = h;
    TC9SetOnGroupLootTypeChangedHook(h);
}

int CallOnGroupLootTypeChangedHook(uint32_t guid, uint8_t lootMethod, uint64_t looter, uint8_t lootThreshold) {
    if (g_compat_handlers.group_loot_changed) {
        g_compat_handlers.group_loot_changed(guid, lootMethod, looter, lootThreshold);
        return GroupHookStatusOK;
    }
    return GroupHookStatusNoHook;
}

void SetOnGroupDungeonDifficultyChangedHook(OnGroupDungeonDifficultyChangedHook h) {
    g_compat_handlers.group_dungeon_diff_changed = h;
    TC9SetOnGroupDungeonDifficultyChangedHook(h);
}

int CallOnGroupDungeonDifficultyChangedHook(uint32_t guid, uint8_t difficulty) {
    if (g_compat_handlers.group_dungeon_diff_changed) {
        g_compat_handlers.group_dungeon_diff_changed(guid, difficulty);
        return GroupHookStatusOK;
    }
    return GroupHookStatusNoHook;
}

void SetOnGroupRaidDifficultyChangedHook(OnGroupRaidDifficultyChangedHook h) {
    g_compat_handlers.group_raid_diff_changed = h;
    TC9SetOnGroupRaidDifficultyChangedHook(h);
}

int CallOnGroupRaidDifficultyChangedHook(uint32_t guid, uint8_t difficulty) {
    if (g_compat_handlers.group_raid_diff_changed) {
        g_compat_handlers.group_raid_diff_changed(guid, difficulty);
        return GroupHookStatusOK;
    }
    return GroupHookStatusNoHook;
}

void SetOnGroupConvertedToRaidHook(OnGroupConvertedToRaidHook h) {
    g_compat_handlers.group_converted_raid = h;
    TC9SetOnGroupConvertedToRaidHook(h);
}

int CallOnGroupConvertedToRaidHook(uint32_t guid) {
    if (g_compat_handlers.group_converted_raid) {
        g_compat_handlers.group_converted_raid(guid);
        return GroupHookStatusOK;
    }
    return GroupHookStatusNoHook;
}

// ============================================================================
// Registry Events (events-servers-registry.h)
// ============================================================================

void SetOnMapsReassignedHook(OnMapsReassignedHook h) {
    g_compat_handlers.maps_reassigned = h;
    TC9SetOnMapsReassignedHook(h);
}

int CallOnMapsReassignedHook(uint32_t* maps_added, int maps_added_size, uint32_t* /*maps_removed*/, int /*maps_removed_size*/) {
    if (g_compat_handlers.maps_reassigned) {
        g_compat_handlers.maps_reassigned(maps_added, maps_added_size, nullptr, 0);
        return ServersRegistryHookStatusOK;
    }
    return ServersRegistryHookStatusNoHook;
}

// ============================================================================
// Monitoring (monitoring.h)
// ============================================================================

void SetMonitoringDataCollectorHandler(MonitoringDataCollectorHandler h) {
    g_compat_handlers.monitoring_collector = h;
    TC9SetMonitoringDataCollectorHandler(h);
}

MonitoringDataCollectorResponse CallMonitoringDataCollectorHandler() {
    if (g_compat_handlers.monitoring_collector) {
        return g_compat_handlers.monitoring_collector();
    }

    MonitoringDataCollectorResponse resp{};
    resp.errorCode = MonitoringErrorCodeNoHandler;
    return resp;
}

// ============================================================================
// Player Items API (player-items-api.h)
// ============================================================================

// Placeholder implementations - need full bridging logic
// These would require significant conversion between Go and C++ types
// For now, marking as TODO

void SetGetPlayerItemsByGuidsHandler(GetPlayerItemsByGuidsHandler h) {
    g_compat_handlers.get_player_items = h;
    // TODO: Bridge to TC9SetGetPlayerItemsByGuidsHandler
    spdlog::warn("SetGetPlayerItemsByGuidsHandler: Bridging not yet implemented");
}

GetPlayerItemsByGuidsResponse CallGetPlayerItemsByGuidsHandler(uint64_t /*player_guid*/, uint64_t* /*items_guids*/, int /*items_guids_size*/) {
    GetPlayerItemsByGuidsResponse resp{};
    resp.errorCode = PlayerItemErrorCodeNoHandler;
    return resp;
}

void SetRemoveItemsWithGuidsFromPlayerHandler(RemoveItemsWithGuidsFromPlayerHandler h) {
    g_compat_handlers.remove_items = h;
    spdlog::warn("SetRemoveItemsWithGuidsFromPlayerHandler: Bridging not yet implemented");
}

RemoveItemsWithGuidsFromPlayerResponse CallRemoveItemsWithGuidsFromPlayerHandler(uint64_t /*player_guid*/, uint64_t* /*items_guids*/, int /*items_guids_size*/, uint64_t /*assign_player_guid*/) {
    RemoveItemsWithGuidsFromPlayerResponse resp{};
    resp.errorCode = PlayerItemErrorCodeNoHandler;
    return resp;
}

void SetAddExistingItemToPlayerHandler(AddExistingItemToPlayerHandler h) {
    g_compat_handlers.add_item = h;
    spdlog::warn("SetAddExistingItemToPlayerHandler: Bridging not yet implemented");
}

PlayerItemErrorCode CallAddExistingItemToPlayerHandler(AddExistingItemToPlayerRequest* /*request*/) {
    return PlayerItemErrorCodeNoHandler;
}

// ============================================================================
// Player Money API (player-money-api.h)
// ============================================================================

void SetGetMoneyForPlayerHandler(GetMoneyForPlayerHandler h) {
    g_compat_handlers.get_money = h;
    spdlog::warn("SetGetMoneyForPlayerHandler: Bridging not yet implemented");
}

GetMoneyForPlayerResponse CallGetMoneyForPlayerHandler(uint64_t /*player_guid*/) {
    GetMoneyForPlayerResponse resp{};
    resp.errorCode = PlayerMoneyErrorCodeNoHandler;
    return resp;
}

void SetModifyMoneyForPlayerHandler(ModifyMoneyForPlayerHandler h) {
    g_compat_handlers.modify_money = h;
    spdlog::warn("SetModifyMoneyForPlayerHandler: Bridging not yet implemented");
}

ModifyMoneyForPlayerResponse CallModifyMoneyForPlayerHandler(uint64_t /*player_guid*/, int32_t /*amount*/) {
    ModifyMoneyForPlayerResponse resp{};
    resp.errorCode = PlayerMoneyErrorCodeNoHandler;
    return resp;
}

// ============================================================================
// Player Interactions API (player-interactions-api.h)
// ============================================================================

void SetCanPlayerInteractWithNPCAndFlagsHandler(CanPlayerInteractWithNPCAndFlagsHandler h) {
    g_compat_handlers.interact_npc = h;
    spdlog::warn("SetCanPlayerInteractWithNPCAndFlagsHandler: Bridging not yet implemented");
}

CanPlayerInteractWithNPCAndFlagsResponse CallCanPlayerInteractWithNPCAndFlagsHandler(uint64_t /*player_guid*/, uint64_t /*npc_guid*/, uint32_t /*npc_flags*/) {
    CanPlayerInteractWithNPCAndFlagsResponse resp{};
    resp.errorCode = PlayerInteractionErrorCodeNoHandler;
    resp.canInteract = false;
    return resp;
}

void SetCanPlayerInteractWithGOAndTypeHandler(CanPlayerInteractWithGOAndTypeHandler h) {
    g_compat_handlers.interact_go = h;
    spdlog::warn("SetCanPlayerInteractWithGOAndTypeHandler: Bridging not yet implemented");
}

CanPlayerInteractWithGOAndTypeResponse CallCanPlayerInteractWithGOAndTypeHandler(uint64_t /*player_guid*/, uint64_t /*go_guid*/, uint8_t /*go_type*/) {
    CanPlayerInteractWithGOAndTypeResponse resp{};
    resp.errorCode = PlayerInteractionErrorCodeNoHandler;
    resp.canInteract = false;
    return resp;
}

// ============================================================================
// Battleground API (battleground-api.h)
// ============================================================================

void SetBattlegroundStartHandler(BattlegroundStartHandler h) {
    g_compat_handlers.bg_start = h;
    spdlog::warn("SetBattlegroundStartHandler: Bridging not yet implemented");
}

BattlegroundStartResponse CallBattlegroundStartHandler(BattlegroundStartRequest* /*request*/) {
    BattlegroundStartResponse resp{};
    resp.errorCode = BattlegroundErrorCodeNoHandler;
    return resp;
}

void SetBattlegroundAddPlayersHandler(BattlegroundAddPlayersHandler h) {
    g_compat_handlers.bg_add_players = h;
    spdlog::warn("SetBattlegroundAddPlayersHandler: Bridging not yet implemented");
}

BattlegroundErrorCode CallBattlegroundAddPlayersHandler(BattlegroundAddPlayersRequest* /*request*/) {
    return BattlegroundErrorCodeNoHandler;
}

void SetCanPlayerJoinBattlegroundQueueHandler(CanPlayerJoinBattlegroundQueueHandler h) {
    g_compat_handlers.can_join_bg_queue = h;
    spdlog::warn("SetCanPlayerJoinBattlegroundQueueHandler: Bridging not yet implemented");
}

BattlegroundJoinCheckErrorCode CallCanPlayerJoinBattlegroundQueueHandler(uint64_t /*playerGuid*/) {
    return BattlegroundJoinCheckErrorCodeNoHook;
}

void SetCanPlayerTeleportToBattlegroundHandler(CanPlayerTeleportToBattlegroundHandler h) {
    g_compat_handlers.can_teleport_bg = h;
    spdlog::warn("SetCanPlayerTeleportToBattlegroundHandler: Bridging not yet implemented");
}

BattlegroundJoinCheckErrorCode CallCanPlayerTeleportToBattlegroundHandler(uint64_t /*playerGuid*/) {
    return BattlegroundJoinCheckErrorCodeNoHook;
}

} // extern "C"
