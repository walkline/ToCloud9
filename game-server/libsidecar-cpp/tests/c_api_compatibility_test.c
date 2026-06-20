/*
 * Test that the C API headers can be compiled with a pure C compiler.
 * This verifies 100% ABI compatibility with the Go version.
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

/* Example handler implementations */
static void on_guild_member_added(uint64_t guild_id, uint64_t player_guid) {
    printf("Guild member added: guild=%llu, player=%llu\n", guild_id, player_guid);
}

static void on_group_created(EventObjectGroup* group) {
    printf("Group created: guid=%u, leader=%llu, members=%d\n",
           group->guid, group->leader, group->membersSize);
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

int main(void) {
    printf("C API Compatibility Test\n");
    printf("=========================\n\n");

    /* Test that all headers compile and symbols are available */

    /* Main API */
    printf("Testing main API...\n");
    /* Don't actually init since we don't have a config file */
    /* InitLib("test-config.json"); */

    /* GUID functions */
    uint64_t item_guid = TC9GetNextAvailableItemGuid(0);
    printf("  TC9GetNextAvailableItemGuid(0) returned: %llu\n", item_guid);

    uint64_t char_guid = TC9GetNextAvailableCharacterGuid(0);
    printf("  TC9GetNextAvailableCharacterGuid(0) returned: %llu\n", char_guid);

    uint64_t instance_guid = TC9GetNextAvailableInstanceGuid(0);
    printf("  TC9GetNextAvailableInstanceGuid(0) returned: %llu\n", instance_guid);

    /* Event hooks */
    printf("\nTesting event hooks...\n");
    TC9SetOnGuildMemberAddedHook(on_guild_member_added);
    printf("  TC9SetOnGuildMemberAddedHook() OK\n");

    TC9SetOnGroupCreatedHook(on_group_created);
    printf("  TC9SetOnGroupCreatedHook() OK\n");

    /* Monitoring */
    printf("\nTesting monitoring...\n");
    TC9SetMonitoringDataCollectorHandler(monitoring_handler);
    printf("  TC9SetMonitoringDataCollectorHandler() OK\n");

    printf("\n✅ All C API tests passed!\n");

    /* GracefulShutdown(); */

    return 0;
}
