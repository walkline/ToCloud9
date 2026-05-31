#ifndef __EVENT_GROUP__
#define __EVENT_GROUP__

#include <stdint.h>
#include <stdlib.h>

enum GroupStatus {
    GroupHookStatusOK = 0,
    GroupHookStatusNoHook = 1
};

typedef struct {
    uint32_t guid;
    uint64_t leader;
    uint8_t lootMethod;
    uint64_t looterGuid;
    uint8_t lootThreshold;
    uint8_t groupType;
    uint8_t difficulty;
    uint8_t raidDifficulty;
    uint64_t masterLooterGuid;
    uint64_t *members;
    char **memberNames;
    uint8_t membersSize;
    uint32_t lfgDungeonEntry;
} EventObjectGroup;

typedef void (*OnGroupCreatedHook) (EventObjectGroup *group);
void SetOnGroupCreatedHook(OnGroupCreatedHook h);
int CallOnGroupCreatedHook(EventObjectGroup *group);

typedef void (*OnGroupMemberAddedHook) (uint32_t guid, uint64_t newMemberGuid);
void SetOnGroupMemberAddedHook(OnGroupMemberAddedHook h);
int CallOnGroupMemberAddedHook(uint32_t guid, uint64_t newMemberGuid);

typedef void (*OnGroupMemberRemovedHook) (uint32_t guid, uint64_t removedMemberGuid, uint64_t newLeaderGuid);
void SetOnGroupMemberRemovedHook(OnGroupMemberRemovedHook h);
int CallOnGroupMemberRemovedHook(uint32_t guid, uint64_t removedMemberGuid, uint64_t newLeaderGuid);

typedef void (*OnGroupLeaderChangedHook) (uint32_t guid, uint64_t previousLeaderGuid, uint64_t newLeaderGuid);
void SetOnGroupLeaderChangedHook(OnGroupLeaderChangedHook h);
int CallOnGroupLeaderChangedHook(uint32_t guid, uint64_t previousLeaderGuid, uint64_t newLeaderGuid);

typedef void (*OnGroupDisbandedHook) (uint32_t guid);
void SetOnGroupDisbandedHook(OnGroupDisbandedHook h);
int CallOnGroupDisbandedHook(uint32_t guid);

typedef void (*OnGroupLootTypeChangedHook) (uint32_t guid, uint8_t lootMethod, uint64_t looter, uint8_t lootThreshold);
void SetOnGroupLootTypeChangedHook(OnGroupLootTypeChangedHook h);
int CallOnGroupLootTypeChangedHook(uint32_t guid, uint8_t lootMethod, uint64_t looter, uint8_t lootThreshold);

typedef void (*OnGroupDungeonDifficultyChangedHook) (uint32_t guid, uint8_t difficulty);
void SetOnGroupDungeonDifficultyChangedHook(OnGroupDungeonDifficultyChangedHook h);
int CallOnGroupDungeonDifficultyChangedHook(uint32_t guid, uint8_t difficulty);

typedef void (*OnGroupRaidDifficultyChangedHook) (uint32_t guid, uint8_t difficulty);
void SetOnGroupRaidDifficultyChangedHook(OnGroupRaidDifficultyChangedHook h);
int CallOnGroupRaidDifficultyChangedHook(uint32_t guid, uint8_t difficulty);

typedef void (*OnGroupConvertedToRaidHook) (uint32_t guid);
void SetOnGroupConvertedToRaidHook(OnGroupConvertedToRaidHook h);
int CallOnGroupConvertedToRaidHook(uint32_t guid);


typedef struct {
    uint32_t groupGuid;
    uint64_t leaderGuid;
    uint32_t durationMs;
} GroupReadyCheckStarted;

typedef struct {
    uint32_t groupGuid;
    uint64_t memberGuid;
    uint8_t state; // 0 = waiting, 1 = ready, 2 = not ready
} GroupReadyCheckMemberState;

typedef struct {
    uint32_t groupGuid;
} GroupReadyCheckFinished;

typedef struct {
    uint32_t groupGuid;
    uint64_t memberGuid;
    uint8_t subGroup;
} GroupMemberSubGroupChanged;

typedef struct {
    uint32_t groupGuid;
    uint64_t memberGuid;
    uint8_t flags;
    uint8_t roles;
} GroupMemberFlagsChanged;

typedef struct {
    uint8_t slot;
    uint32_t spellId;
    uint8_t flags;
} GroupMemberAuraState;

typedef struct {
    uint32_t groupGuid;
    uint64_t memberGuid;
    uint8_t online;
    uint8_t level;
    uint8_t playerClass;
    uint32_t zoneId;
    uint32_t mapId;
    uint32_t health;
    uint32_t maxHealth;
    uint8_t powerType;
    uint32_t power;
    uint32_t maxPower;
    uint8_t aurasKnown;
    uint32_t auraCount;
    GroupMemberAuraState* auras;
} GroupMemberStateChanged;

typedef struct {
    uint32_t groupGuid;
    uint64_t playerGuid;
    uint32_t mapId;
    uint8_t difficulty;
} GroupInstanceResetRequest;

typedef struct {
    uint32_t groupGuid;
    uint64_t playerGuid;
    uint32_t mapId;
    uint8_t difficulty;
    uint8_t extended;
} GroupInstanceBindExtensionRequest;

typedef struct {
    uint64_t memberGuid;
    const char* memberName;
    uint8_t online;
    uint8_t flags;
    uint8_t roles;
    uint8_t subGroup;
} GroupMaterializedLfgMember;

typedef void (*OnGroupReadyCheckStartedHook)(GroupReadyCheckStarted* request);
void SetOnGroupReadyCheckStartedHook(OnGroupReadyCheckStartedHook h);
int CallOnGroupReadyCheckStartedHook(GroupReadyCheckStarted* request);

typedef void (*OnGroupReadyCheckMemberStateHook)(GroupReadyCheckMemberState* request);
void SetOnGroupReadyCheckMemberStateHook(OnGroupReadyCheckMemberStateHook h);
int CallOnGroupReadyCheckMemberStateHook(GroupReadyCheckMemberState* request);

typedef void (*OnGroupReadyCheckFinishedHook)(GroupReadyCheckFinished* request);
void SetOnGroupReadyCheckFinishedHook(OnGroupReadyCheckFinishedHook h);
int CallOnGroupReadyCheckFinishedHook(GroupReadyCheckFinished* request);

typedef void (*OnGroupMemberSubGroupChangedHook)(GroupMemberSubGroupChanged* request);
void SetOnGroupMemberSubGroupChangedHook(OnGroupMemberSubGroupChangedHook h);
int CallOnGroupMemberSubGroupChangedHook(GroupMemberSubGroupChanged* request);

typedef void (*OnGroupMemberFlagsChangedHook)(GroupMemberFlagsChanged* request);
void SetOnGroupMemberFlagsChangedHook(OnGroupMemberFlagsChangedHook h);
int CallOnGroupMemberFlagsChangedHook(GroupMemberFlagsChanged* request);

typedef void (*OnGroupMemberStateChangedHook)(GroupMemberStateChanged* request);
void SetOnGroupMemberStateChangedHook(OnGroupMemberStateChangedHook h);
int CallOnGroupMemberStateChangedHook(GroupMemberStateChanged* request);

typedef void (*OnGroupInstanceResetRequestHook)(GroupInstanceResetRequest* request);
void SetOnGroupInstanceResetRequestHook(OnGroupInstanceResetRequestHook h);
int CallOnGroupInstanceResetRequestHook(GroupInstanceResetRequest* request);

typedef void (*OnGroupInstanceBindExtensionRequestHook)(GroupInstanceBindExtensionRequest* request);
void SetOnGroupInstanceBindExtensionRequestHook(OnGroupInstanceBindExtensionRequestHook h);
int CallOnGroupInstanceBindExtensionRequestHook(GroupInstanceBindExtensionRequest* request);

#endif
