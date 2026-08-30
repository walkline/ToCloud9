/*
 * Test that the C API headers can be compiled with a pure C compiler and
 * that the full exported surface (every TC9-prefixed symbol plus the
 * Go-era Set/Call helpers) links and behaves as documented when the library
 * is NOT initialized:
 *   - lifecycle calls are safe no-ops,
 *   - GUID getters return 0,
 *   - NATS publish/subscribe report failure,
 *   - every Call* helper reports NoHandler/NoHook while nothing is
 *     registered,
 *   - hook Call* helpers (guild/group/registry/monitoring) forward every
 *     argument once a hook is set; the gRPC-request families keep
 *     reporting NoHandler by design — their live path is the gRPC
 *     service — so only their no-handler contract is asserted.
 * TC9InitLib is intentionally never called: it needs live services.
 */

#include <libsidecar.h>
#include <events-guild.h>
#include <events-group.h>
#include <events-servers-registry.h>
#include <player-items-api.h>
#include <player-money-api.h>
#include <player-interactions-api.h>
#include <battleground-api.h>
#include <monitoring.h>

#include <stdio.h>
#include <string.h>

static int failures = 0;

#define CHECK(cond, msg)                       \
    do {                                       \
        if (!(cond)) {                         \
            printf("  FAIL: %s\n", msg);       \
            failures++;                        \
        }                                      \
    } while (0)

/* ------------------------------------------------------------------ */
/* Handlers capturing their arguments                                  */
/* ------------------------------------------------------------------ */

static uint64_t last_guild_id, last_guild_member;
static void on_guild_member_added(uint64_t guild_id, uint64_t player_guid) {
    last_guild_id = guild_id;
    last_guild_member = player_guid;
}
static void on_guild_member_left(uint64_t guild_id, uint64_t player_guid) {
    last_guild_id = guild_id;
    last_guild_member = player_guid;
}
static void on_guild_member_removed(uint64_t guild_id, uint64_t player_guid) {
    last_guild_id = guild_id;
    last_guild_member = player_guid;
}

static uint32_t last_group_guid;
static uint64_t last_member_guid, last_new_leader_guid, last_looter_guid;
static uint8_t last_u8_a, last_u8_b;
static uint8_t last_members_size;
static void on_group_created(EventObjectGroup* group) {
    last_group_guid = group->guid;
    last_member_guid = group->leader;
    last_members_size = group->membersSize;
}
static void on_group_member_added(uint32_t guid, uint64_t member) {
    last_group_guid = guid;
    last_member_guid = member;
}
static void on_group_member_removed(uint32_t guid, uint64_t member, uint64_t new_leader) {
    last_group_guid = guid;
    last_member_guid = member;
    last_new_leader_guid = new_leader;
}
static void on_group_disbanded(uint32_t guid) {
    last_group_guid = guid;
}
static void on_group_loot_changed(uint32_t guid, uint8_t loot_method, uint64_t looter, uint8_t threshold) {
    last_group_guid = guid;
    last_u8_a = loot_method;
    last_looter_guid = looter;
    last_u8_b = threshold;
}
static void on_group_dungeon_diff(uint32_t guid, uint8_t difficulty) {
    last_group_guid = guid;
    last_u8_a = difficulty;
}
static void on_group_raid_diff(uint32_t guid, uint8_t difficulty) {
    last_group_guid = guid;
    last_u8_b = difficulty;
}
static void on_group_converted(uint32_t guid) {
    last_group_guid = guid;
}

static int last_maps_added_size, last_maps_removed_size;
static void on_maps_reassigned(uint32_t* added, int added_size, uint32_t* removed, int removed_size) {
    (void)added;
    (void)removed;
    last_maps_added_size = added_size;
    last_maps_removed_size = removed_size;
}

static MonitoringDataCollectorResponse monitoring_handler(void) {
    MonitoringDataCollectorResponse resp;
    resp.errorCode = MonitoringErrorCodeNoError;
    resp.connectedPlayers = 42;
    resp.diffMean = 100;
    resp.diffMedian = 95;
    resp.diff95Percentile = 150;
    resp.diff99Percentile = 200;
    resp.diffMaxPercentile = 300;
    return resp;
}

/* gRPC-request handlers: registration only is exercised here (the live
 * invocation path goes through the gRPC service, which needs InitLib). */
static GetPlayerItemsByGuidsResponse get_items_handler(uint64_t p, uint64_t* g, int n) {
    GetPlayerItemsByGuidsResponse resp;
    (void)p; (void)g; (void)n;
    memset(&resp, 0, sizeof(resp));
    return resp;
}
static RemoveItemsWithGuidsFromPlayerResponse remove_items_handler(uint64_t p, uint64_t* g, int n, uint64_t a) {
    RemoveItemsWithGuidsFromPlayerResponse resp;
    (void)p; (void)g; (void)n; (void)a;
    memset(&resp, 0, sizeof(resp));
    return resp;
}
static PlayerItemErrorCode add_item_handler(AddExistingItemToPlayerRequest* req) {
    (void)req;
    return PlayerItemErrorCodeNoError;
}
static GetMoneyForPlayerResponse get_money_handler(uint64_t p) {
    GetMoneyForPlayerResponse resp;
    (void)p;
    memset(&resp, 0, sizeof(resp));
    return resp;
}
static ModifyMoneyForPlayerResponse modify_money_handler(uint64_t p, int32_t v) {
    ModifyMoneyForPlayerResponse resp;
    (void)p; (void)v;
    memset(&resp, 0, sizeof(resp));
    return resp;
}
static CanPlayerInteractWithNPCAndFlagsResponse interact_npc_handler(uint64_t p, uint64_t n, uint32_t f) {
    CanPlayerInteractWithNPCAndFlagsResponse resp;
    (void)p; (void)n; (void)f;
    memset(&resp, 0, sizeof(resp));
    return resp;
}
static CanPlayerInteractWithGOAndTypeResponse interact_go_handler(uint64_t p, uint64_t g, uint8_t t) {
    CanPlayerInteractWithGOAndTypeResponse resp;
    (void)p; (void)g; (void)t;
    memset(&resp, 0, sizeof(resp));
    return resp;
}
static BattlegroundStartResponse bg_start_handler(BattlegroundStartRequest* req) {
    BattlegroundStartResponse resp;
    (void)req;
    memset(&resp, 0, sizeof(resp));
    return resp;
}
static BattlegroundErrorCode bg_add_players_handler(BattlegroundAddPlayersRequest* req) {
    (void)req;
    return BattlegroundErrorCodeNoError;
}
static BattlegroundJoinCheckErrorCode bg_can_join_handler(uint64_t p) {
    (void)p;
    return BattlegroundJoinCheckErrorCodeOK;
}
static BattlegroundJoinCheckErrorCode bg_can_teleport_handler(uint64_t p) {
    (void)p;
    return BattlegroundJoinCheckErrorCodeOK;
}

static void nats_message_handler(const char* subject, const char* payload, int payload_len) {
    (void)subject; (void)payload; (void)payload_len;
}

int main(void) {
    int major = -1, minor = -1, patch = -1;
    const char* version_str = NULL;

    printf("C API Compatibility Test\n");
    printf("=========================\n\n");

    /* Version / ABI APIs */
    printf("Testing version API...\n");
    TC9GetVersion(&major, &minor, &patch);
    version_str = TC9GetVersionString();
    printf("  TC9GetVersion -> %d.%d.%d\n", major, minor, patch);
    printf("  TC9GetVersionString -> %s\n", version_str ? version_str : "(null)");
    printf("  headers TC9_VERSION_* -> %d.%d.%d (%s)\n",
           TC9_VERSION_MAJOR, TC9_VERSION_MINOR, TC9_VERSION_PATCH,
           TC9_VERSION_STRING);

    CHECK(major == TC9_VERSION_MAJOR && minor == TC9_VERSION_MINOR && patch == TC9_VERSION_PATCH,
          "runtime version does not match header macros");
    CHECK(version_str && strcmp(version_str, TC9_VERSION_STRING) == 0,
          "TC9GetVersionString mismatch");
    CHECK(TC9CheckAbiCompatible(TC9_VERSION_MAJOR, TC9_VERSION_MINOR) == 0,
          "TC9CheckAbiCompatible should accept matching headers");
    CHECK(TC9CheckAbiCompatible(TC9_VERSION_MAJOR + 1, 0) != 0,
          "TC9CheckAbiCompatible should reject different major");
    CHECK(TC9CheckAbiCompatible(TC9_VERSION_MAJOR, TC9_VERSION_MINOR + 1) != 0,
          "TC9CheckAbiCompatible should reject higher required minor");

    /* TC9InitLib presence (never called: it needs live services) */
    {
        void (*init_ptr)(uint16_t, uint32_t, uint8_t, char*, uint32_t**, int*) = TC9InitLib;
        CHECK(init_ptr != NULL, "TC9InitLib symbol missing");
    }

    /* Lifecycle calls must be safe no-ops before initialization */
    printf("\nTesting uninitialized lifecycle no-ops...\n");
    TC9ProcessEventsHooks();
    TC9ProcessGRPCOrHTTPRequests();
    TC9ReadyToAcceptPlayersFromMaps(NULL, 0);
    TC9PlayerLeftBattleground(1, 1, 1);
    TC9BattlegroundStatusChanged(1, 0);
    TC9GracefulShutdown();
    printf("  lifecycle no-ops OK\n");

    /* NATS API must report failure before initialization */
    printf("\nTesting uninitialized NATS API...\n");
    CHECK(TC9NatsPublish("test.subject", "x", 1) == -1,
          "TC9NatsPublish should fail before init");
    CHECK(TC9NatsPublish(NULL, NULL, 0) == -1,
          "TC9NatsPublish should reject a null subject");
    CHECK(TC9NatsSubscribe("test.subject", NULL) == -1,
          "TC9NatsSubscribe should reject a null handler");
    CHECK(TC9NatsSubscribe(NULL, nats_message_handler) == -1,
          "TC9NatsSubscribe should reject a null subject");
    CHECK(TC9NatsSubscribe("test.subject", nats_message_handler) == -1,
          "TC9NatsSubscribe should fail before init");

    /* GUID getters must return 0 before initialization */
    printf("\nTesting uninitialized GUID API...\n");
    CHECK(TC9GetNextAvailableCharacterGuid(0) == 0,
          "TC9GetNextAvailableCharacterGuid should return 0 before init");
    CHECK(TC9GetNextAvailableItemGuid(0) == 0,
          "TC9GetNextAvailableItemGuid should return 0 before init");
    CHECK(TC9GetNextAvailableInstanceGuid(0) == 0,
          "TC9GetNextAvailableInstanceGuid should return 0 before init");

    /* Call* helpers report NoHook/NoHandler while nothing is registered */
    printf("\nTesting Call* helpers without handlers...\n");
    CHECK(CallOnGuildMemberAddedHook(1, 2) == GuildHookStatusNoHook,
          "CallOnGuildMemberAddedHook should report NoHook");
    CHECK(CallOnGuildMemberLeftHook(1, 2) == GuildHookStatusNoHook,
          "CallOnGuildMemberLeftHook should report NoHook");
    CHECK(CallOnGuildMemberRemovedHook(1, 2) == GuildHookStatusNoHook,
          "CallOnGuildMemberRemovedHook should report NoHook");
    CHECK(CallOnGroupMemberAddedHook(1, 2) == GroupHookStatusNoHook,
          "CallOnGroupMemberAddedHook should report NoHook");
    CHECK(CallOnGroupMemberRemovedHook(1, 2, 3) == GroupHookStatusNoHook,
          "CallOnGroupMemberRemovedHook should report NoHook");
    CHECK(CallOnGroupDisbandedHook(1) == GroupHookStatusNoHook,
          "CallOnGroupDisbandedHook should report NoHook");
    CHECK(CallOnGroupLootTypeChangedHook(1, 0, 2, 0) == GroupHookStatusNoHook,
          "CallOnGroupLootTypeChangedHook should report NoHook");
    CHECK(CallOnGroupDungeonDifficultyChangedHook(1, 0) == GroupHookStatusNoHook,
          "CallOnGroupDungeonDifficultyChangedHook should report NoHook");
    CHECK(CallOnGroupRaidDifficultyChangedHook(1, 0) == GroupHookStatusNoHook,
          "CallOnGroupRaidDifficultyChangedHook should report NoHook");
    CHECK(CallOnGroupConvertedToRaidHook(1) == GroupHookStatusNoHook,
          "CallOnGroupConvertedToRaidHook should report NoHook");
    CHECK(CallOnMapsReassignedHook(NULL, 0, NULL, 0) == ServersRegistryHookStatusNoHook,
          "CallOnMapsReassignedHook should report NoHook");
    CHECK(CallMonitoringDataCollectorHandler().errorCode == MonitoringErrorCodeNoHandler,
          "CallMonitoringDataCollectorHandler should report NoHandler");
    CHECK(CallGetPlayerItemsByGuidsHandler(1, NULL, 0).errorCode == PlayerItemErrorCodeNoHandler,
          "CallGetPlayerItemsByGuidsHandler should report NoHandler");
    CHECK(CallRemoveItemsWithGuidsFromPlayerHandler(1, NULL, 0, 0).errorCode == PlayerItemErrorCodeNoHandler,
          "CallRemoveItemsWithGuidsFromPlayerHandler should report NoHandler");
    CHECK(CallGetMoneyForPlayerHandler(1).errorCode == PlayerMoneyErrorCodeNoHandler,
          "CallGetMoneyForPlayerHandler should report NoHandler");
    CHECK(CallCanPlayerInteractWithNPCAndFlagsHandler(1, 2, 0).errorCode == PlayerInteractionErrorCodeNoHandler,
          "CallCanPlayerInteractWithNPCAndFlagsHandler should report NoHandler");
    CHECK(CallCanPlayerInteractWithGOAndTypeHandler(1, 2, 0).errorCode == PlayerInteractionErrorCodeNoHandler,
          "CallCanPlayerInteractWithGOAndTypeHandler should report NoHandler");
    {
        BattlegroundStartRequest req;
        BattlegroundAddPlayersRequest add_req;
        AddExistingItemToPlayerRequest item_req;
        memset(&req, 0, sizeof(req));
        memset(&add_req, 0, sizeof(add_req));
        memset(&item_req, 0, sizeof(item_req));
        CHECK(CallBattlegroundStartHandler(&req).errorCode == BattlegroundErrorCodeNoHandler,
              "CallBattlegroundStartHandler should report NoHandler");
        CHECK(CallBattlegroundAddPlayersHandler(&add_req) == BattlegroundErrorCodeNoHandler,
              "CallBattlegroundAddPlayersHandler should report NoHandler");
        CHECK(CallAddExistingItemToPlayerHandler(&item_req) == PlayerItemErrorCodeNoHandler,
              "CallAddExistingItemToPlayerHandler should report NoHandler");
        CHECK(CallCanPlayerJoinBattlegroundQueueHandler(1) == BattlegroundJoinCheckErrorCodeNoHook,
              "CallCanPlayerJoinBattlegroundQueueHandler should report NoHook");
        CHECK(CallCanPlayerTeleportToBattlegroundHandler(1) == BattlegroundJoinCheckErrorCodeNoHook,
              "CallCanPlayerTeleportToBattlegroundHandler should report NoHook");
        CHECK(CallModifyMoneyForPlayerHandler(1, 1).errorCode == PlayerMoneyErrorCodeNoHandler,
              "CallModifyMoneyForPlayerHandler should report NoHandler");
    }

    /* Register the full TC9 surface (matches what the worldserver does) */
    printf("\nRegistering the full TC9Set* surface...\n");
    TC9SetOnGroupCreatedHook(on_group_created);
    TC9SetOnGroupMemberAddedHook(on_group_member_added);
    TC9SetOnGroupMemberRemovedHook(on_group_member_removed);
    TC9SetOnGroupDisbandedHook(on_group_disbanded);
    TC9SetOnGroupLootTypeChangedHook(on_group_loot_changed);
    TC9SetOnGroupDungeonDifficultyChangedHook(on_group_dungeon_diff);
    TC9SetOnGroupRaidDifficultyChangedHook(on_group_raid_diff);
    TC9SetOnGroupConvertedToRaidHook(on_group_converted);
    TC9SetOnGuildMemberAddedHook(on_guild_member_added);
    TC9SetOnGuildMemberLeftHook(on_guild_member_left);
    TC9SetOnGuildMemberRemovedHook(on_guild_member_removed);
    TC9SetOnMapsReassignedHook(on_maps_reassigned);
    TC9SetMonitoringDataCollectorHandler(monitoring_handler);
    TC9SetGetPlayerItemsByGuidsHandler(get_items_handler);
    TC9SetRemoveItemsWithGuidsFromPlayerHandler(remove_items_handler);
    TC9SetAddExistingItemToPlayerHandler(add_item_handler);
    TC9SetGetMoneyForPlayerHandler(get_money_handler);
    TC9SetModifyMoneyForPlayerHandler(modify_money_handler);
    TC9SetCanPlayerInteractWithNPCAndFlagsHandler(interact_npc_handler);
    TC9SetCanPlayerInteractWithGOAndTypeHandler(interact_go_handler);
    TC9SetBattlegroundStartHandler(bg_start_handler);
    TC9SetBattlegroundAddPlayersHandler(bg_add_players_handler);
    TC9SetCanPlayerJoinBattlegroundQueueHandler(bg_can_join_handler);
    TC9SetCanPlayerTeleportToBattlegroundHandler(bg_can_teleport_handler);
    printf("  all TC9Set* calls OK\n");

    /* Go-era Set* of the gRPC-request families: the live path goes through
     * the gRPC service and the stored bindings, so these shims only store
     * the pointer today — referencing them still pins the exported ABI
     * surface (a removed or renamed export fails this build). */
    printf("\nReferencing Go-era Set* helpers of the gRPC-request families...\n");
    SetGetPlayerItemsByGuidsHandler(get_items_handler);
    SetRemoveItemsWithGuidsFromPlayerHandler(remove_items_handler);
    SetAddExistingItemToPlayerHandler(add_item_handler);
    SetGetMoneyForPlayerHandler(get_money_handler);
    SetModifyMoneyForPlayerHandler(modify_money_handler);
    SetCanPlayerInteractWithNPCAndFlagsHandler(interact_npc_handler);
    SetCanPlayerInteractWithGOAndTypeHandler(interact_go_handler);
    SetBattlegroundStartHandler(bg_start_handler);
    SetBattlegroundAddPlayersHandler(bg_add_players_handler);
    SetCanPlayerJoinBattlegroundQueueHandler(bg_can_join_handler);
    SetCanPlayerTeleportToBattlegroundHandler(bg_can_teleport_handler);
    printf("  all Go-era Set* calls OK\n");

    /* Hook round-trips through the Go-era Set and Call helpers */
    printf("\nTesting hook round-trips (Set* then Call*)...\n");
    SetOnGuildMemberAddedHook(on_guild_member_added);
    CHECK(CallOnGuildMemberAddedHook(11, 22) == GuildHookStatusOK,
          "CallOnGuildMemberAddedHook should report OK after Set");
    CHECK(last_guild_id == 11 && last_guild_member == 22,
          "guild member added arguments not forwarded");

    SetOnGuildMemberLeftHook(on_guild_member_left);
    CHECK(CallOnGuildMemberLeftHook(33, 44) == GuildHookStatusOK,
          "CallOnGuildMemberLeftHook should report OK after Set");
    CHECK(last_guild_id == 33 && last_guild_member == 44,
          "guild member left arguments not forwarded");

    SetOnGuildMemberRemovedHook(on_guild_member_removed);
    CHECK(CallOnGuildMemberRemovedHook(55, 66) == GuildHookStatusOK,
          "CallOnGuildMemberRemovedHook should report OK after Set");
    CHECK(last_guild_id == 55 && last_guild_member == 66,
          "guild member removed arguments not forwarded");

    {
        uint64_t members[2] = {7, 8};
        EventObjectGroup group;
        memset(&group, 0, sizeof(group));
        group.guid = 77;
        group.leader = 7;
        group.members = members;
        group.membersSize = 2;
        SetOnGroupCreatedHook(on_group_created);
        CHECK(CallOnGroupCreatedHook(&group) == GroupHookStatusOK,
              "CallOnGroupCreatedHook should report OK after Set");
        CHECK(last_group_guid == 77 && last_member_guid == 7 && last_members_size == 2,
              "group created arguments not forwarded");
    }

    SetOnGroupMemberAddedHook(on_group_member_added);
    CHECK(CallOnGroupMemberAddedHook(88, 99) == GroupHookStatusOK,
          "CallOnGroupMemberAddedHook should report OK after Set");
    CHECK(last_group_guid == 88 && last_member_guid == 99,
          "group member added arguments not forwarded");

    /* NewLeaderID must reach the hook as the third argument */
    SetOnGroupMemberRemovedHook(on_group_member_removed);
    CHECK(CallOnGroupMemberRemovedHook(7, 8, 9) == GroupHookStatusOK,
          "CallOnGroupMemberRemovedHook should report OK after Set");
    CHECK(last_group_guid == 7 && last_member_guid == 8 && last_new_leader_guid == 9,
          "group member removed arguments (incl. newLeaderGuid) not forwarded");

    SetOnGroupDisbandedHook(on_group_disbanded);
    CHECK(CallOnGroupDisbandedHook(111) == GroupHookStatusOK,
          "CallOnGroupDisbandedHook should report OK after Set");
    CHECK(last_group_guid == 111, "group disbanded argument not forwarded");

    SetOnGroupLootTypeChangedHook(on_group_loot_changed);
    CHECK(CallOnGroupLootTypeChangedHook(5, 2, 123, 3) == GroupHookStatusOK,
          "CallOnGroupLootTypeChangedHook should report OK after Set");
    CHECK(last_group_guid == 5 && last_u8_a == 2 && last_looter_guid == 123 && last_u8_b == 3,
          "group loot changed arguments not forwarded");

    SetOnGroupDungeonDifficultyChangedHook(on_group_dungeon_diff);
    CHECK(CallOnGroupDungeonDifficultyChangedHook(6, 1) == GroupHookStatusOK,
          "CallOnGroupDungeonDifficultyChangedHook should report OK after Set");
    CHECK(last_group_guid == 6 && last_u8_a == 1,
          "dungeon difficulty arguments not forwarded");

    SetOnGroupRaidDifficultyChangedHook(on_group_raid_diff);
    CHECK(CallOnGroupRaidDifficultyChangedHook(9, 2) == GroupHookStatusOK,
          "CallOnGroupRaidDifficultyChangedHook should report OK after Set");
    CHECK(last_group_guid == 9 && last_u8_b == 2,
          "raid difficulty arguments not forwarded");

    SetOnGroupConvertedToRaidHook(on_group_converted);
    CHECK(CallOnGroupConvertedToRaidHook(13) == GroupHookStatusOK,
          "CallOnGroupConvertedToRaidHook should report OK after Set");
    CHECK(last_group_guid == 13, "group converted argument not forwarded");

    {
        uint32_t added[1] = {530};
        uint32_t removed[2] = {0, 1};
        SetOnMapsReassignedHook(on_maps_reassigned);
        CHECK(CallOnMapsReassignedHook(added, 1, removed, 2) == ServersRegistryHookStatusOK,
              "CallOnMapsReassignedHook should report OK after Set");
        CHECK(last_maps_added_size == 1 && last_maps_removed_size == 2,
              "maps reassigned arguments not forwarded");
    }

    SetMonitoringDataCollectorHandler(monitoring_handler);
    {
        MonitoringDataCollectorResponse resp = CallMonitoringDataCollectorHandler();
        CHECK(resp.errorCode == MonitoringErrorCodeNoError && resp.connectedPlayers == 42,
              "monitoring collector response not forwarded");
    }

    if (failures != 0) {
        printf("\n[FAILED] C API tests failed (%d)\n", failures);
        return 1;
    }

    printf("\n[OK] All C API tests passed!\n");
    return 0;
}
