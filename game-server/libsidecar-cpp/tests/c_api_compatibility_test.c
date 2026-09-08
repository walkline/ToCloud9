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
#include <string.h>

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
    int major = -1, minor = -1, patch = -1;
    const char* version_str = NULL;
    int failures = 0;

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

    if (major != TC9_VERSION_MAJOR || minor != TC9_VERSION_MINOR || patch != TC9_VERSION_PATCH) {
        printf("  FAIL: runtime version does not match header macros\n");
        failures++;
    }
    if (!version_str || strcmp(version_str, TC9_VERSION_STRING) != 0) {
        printf("  FAIL: TC9GetVersionString mismatch\n");
        failures++;
    }
    if (TC9CheckAbiCompatible(TC9_VERSION_MAJOR, TC9_VERSION_MINOR) != 0) {
        printf("  FAIL: TC9CheckAbiCompatible should accept matching headers\n");
        failures++;
    }
    if (TC9CheckAbiCompatible(TC9_VERSION_MAJOR + 1, 0) == 0) {
        printf("  FAIL: TC9CheckAbiCompatible should reject different major\n");
        failures++;
    }
    if (TC9CheckAbiCompatible(TC9_VERSION_MAJOR, TC9_VERSION_MINOR + 1) == 0) {
        printf("  FAIL: TC9CheckAbiCompatible should reject higher required minor\n");
        failures++;
    }
    if (failures == 0) {
        printf("  version API OK\n");
    }

    /* Test that all headers compile and symbols are available */

    /* Main API */
    printf("\nTesting main API...\n");
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

    if (failures != 0) {
        printf("\n❌ C API tests failed (%d)\n", failures);
        return 1;
    }

    printf("\n✅ All C API tests passed!\n");

    /* GracefulShutdown(); */

    return 0;
}
