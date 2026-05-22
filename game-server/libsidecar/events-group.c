#include "events-group.h"

// OnGroupCreatedHook
OnGroupCreatedHook groupCreatedHook;
void SetOnGroupCreatedHook(OnGroupCreatedHook h) {
    groupCreatedHook = h;
}

int CallOnGroupCreatedHook(EventObjectGroup *group) {
    if (groupCreatedHook == 0) {
        return GroupHookStatusNoHook;
    }
    groupCreatedHook(group);
    return GroupHookStatusOK;
}

// GroupMemberAdded
static OnGroupMemberAddedHook groupMemberAddedHook;
void SetOnGroupMemberAddedHook(OnGroupMemberAddedHook h) {
    groupMemberAddedHook = h;
}

int CallOnGroupMemberAddedHook(uint32_t guid, uint64_t newMemberGuid) {
    if (groupMemberAddedHook == 0) {
        return GroupHookStatusNoHook;
    }
    groupMemberAddedHook(guid, newMemberGuid);
    return GroupHookStatusOK;
}

// GroupMemberRemoved
static OnGroupMemberRemovedHook groupMemberRemovedHook;
void SetOnGroupMemberRemovedHook(OnGroupMemberRemovedHook h) {
    groupMemberRemovedHook = h;
}

int CallOnGroupMemberRemovedHook(uint32_t guid, uint64_t removedMemberGuid, uint64_t newLeaderGuid) {
    if (groupMemberRemovedHook == 0) {
        return GroupHookStatusNoHook;
    }
    groupMemberRemovedHook(guid, removedMemberGuid, newLeaderGuid);
    return GroupHookStatusOK;
}

static OnGroupLeaderChangedHook groupLeaderChangedHook;
void SetOnGroupLeaderChangedHook(OnGroupLeaderChangedHook h) {
    groupLeaderChangedHook = h;
}

int CallOnGroupLeaderChangedHook(uint32_t guid, uint64_t previousLeaderGuid, uint64_t newLeaderGuid) {
    if (groupLeaderChangedHook == 0) {
        return GroupHookStatusNoHook;
    }
    groupLeaderChangedHook(guid, previousLeaderGuid, newLeaderGuid);
    return GroupHookStatusOK;
}

typedef void (*OnGroupDisbandedHook) (uint32_t guid);
void SetOnGroupDisbandedHook(OnGroupDisbandedHook h);
int CallOnGroupDisbandedHook(uint32_t guid);

static OnGroupDisbandedHook groupDisbandedHook;
void SetOnGroupDisbandedHook(OnGroupDisbandedHook h) {
    groupDisbandedHook = h;
}

int CallOnGroupDisbandedHook(uint32_t guid) {
    if (groupDisbandedHook == 0) {
        return GroupHookStatusNoHook;
    }
    groupDisbandedHook(guid);
    return GroupHookStatusOK;
}

static OnGroupLootTypeChangedHook groupLootTypeChanged;
void SetOnGroupLootTypeChangedHook(OnGroupLootTypeChangedHook h) {
    groupLootTypeChanged = h;
}

int CallOnGroupLootTypeChangedHook(uint32_t guid, uint8_t lootMethod, uint64_t looter, uint8_t lootThreshold) {
    if (groupLootTypeChanged == 0) {
        return GroupHookStatusNoHook;
    }
    groupLootTypeChanged(guid, lootMethod, looter, lootThreshold);
    return GroupHookStatusOK;
}

static OnGroupDungeonDifficultyChangedHook groupDungeonDifficultyChanged;
void SetOnGroupDungeonDifficultyChangedHook(OnGroupDungeonDifficultyChangedHook h) {
    groupDungeonDifficultyChanged = h;
}

int CallOnGroupDungeonDifficultyChangedHook(uint32_t guid, uint8_t difficulty) {
    if (groupDungeonDifficultyChanged == 0) {
        return GroupHookStatusNoHook;
    }
    groupDungeonDifficultyChanged(guid, difficulty);
    return GroupHookStatusOK;
}

static OnGroupRaidDifficultyChangedHook groupRaidDifficultyChanged;
void SetOnGroupRaidDifficultyChangedHook(OnGroupRaidDifficultyChangedHook h) {
    groupRaidDifficultyChanged = h;
}

int CallOnGroupRaidDifficultyChangedHook(uint32_t guid, uint8_t difficulty) {
    if (groupRaidDifficultyChanged == 0) {
        return GroupHookStatusNoHook;
    }
    groupRaidDifficultyChanged(guid, difficulty);
    return GroupHookStatusOK;
}

typedef void (*OnGroupConvertedToRaidHook) (uint32_t guid);
void SetOnGroupConvertedToRaidHook(OnGroupConvertedToRaidHook h);
int CallOnGroupConvertedToRaidHook(uint32_t guid);

static OnGroupConvertedToRaidHook groupConvertedToRaid;
void SetOnGroupConvertedToRaidHook(OnGroupConvertedToRaidHook h) {
    groupConvertedToRaid = h;
}

int CallOnGroupConvertedToRaidHook(uint32_t guid) {
    if (groupConvertedToRaid == 0) {
        return GroupHookStatusNoHook;
    }
    groupConvertedToRaid(guid);
    return GroupHookStatusOK;
}


static OnGroupReadyCheckStartedHook groupReadyCheckStartedHook;
void SetOnGroupReadyCheckStartedHook(OnGroupReadyCheckStartedHook h) {
    groupReadyCheckStartedHook = h;
}

int CallOnGroupReadyCheckStartedHook(GroupReadyCheckStarted* request) {
    if (groupReadyCheckStartedHook == 0)
        return GroupHookStatusNoHook;

    groupReadyCheckStartedHook(request);
    return GroupHookStatusOK;
}

static OnGroupReadyCheckMemberStateHook groupReadyCheckMemberStateHook;
void SetOnGroupReadyCheckMemberStateHook(OnGroupReadyCheckMemberStateHook h) {
    groupReadyCheckMemberStateHook = h;
}

int CallOnGroupReadyCheckMemberStateHook(GroupReadyCheckMemberState* request) {
    if (groupReadyCheckMemberStateHook == 0)
        return GroupHookStatusNoHook;

    groupReadyCheckMemberStateHook(request);
    return GroupHookStatusOK;
}

static OnGroupReadyCheckFinishedHook groupReadyCheckFinishedHook;
void SetOnGroupReadyCheckFinishedHook(OnGroupReadyCheckFinishedHook h) {
    groupReadyCheckFinishedHook = h;
}

int CallOnGroupReadyCheckFinishedHook(GroupReadyCheckFinished* request) {
    if (groupReadyCheckFinishedHook == 0)
        return GroupHookStatusNoHook;

    groupReadyCheckFinishedHook(request);
    return GroupHookStatusOK;
}

static OnGroupMemberSubGroupChangedHook groupMemberSubGroupChangedHook;
void SetOnGroupMemberSubGroupChangedHook(OnGroupMemberSubGroupChangedHook h) {
    groupMemberSubGroupChangedHook = h;
}

int CallOnGroupMemberSubGroupChangedHook(GroupMemberSubGroupChanged* request) {
    if (groupMemberSubGroupChangedHook == 0)
        return GroupHookStatusNoHook;

    groupMemberSubGroupChangedHook(request);
    return GroupHookStatusOK;
}

static OnGroupMemberFlagsChangedHook groupMemberFlagsChangedHook;
void SetOnGroupMemberFlagsChangedHook(OnGroupMemberFlagsChangedHook h) {
    groupMemberFlagsChangedHook = h;
}

int CallOnGroupMemberFlagsChangedHook(GroupMemberFlagsChanged* request) {
    if (groupMemberFlagsChangedHook == 0)
        return GroupHookStatusNoHook;

    groupMemberFlagsChangedHook(request);
    return GroupHookStatusOK;
}

static OnGroupMemberStateChangedHook groupMemberStateChangedHook;
void SetOnGroupMemberStateChangedHook(OnGroupMemberStateChangedHook h) {
    groupMemberStateChangedHook = h;
}

int CallOnGroupMemberStateChangedHook(GroupMemberStateChanged* request) {
    if (groupMemberStateChangedHook == 0)
        return GroupHookStatusNoHook;

    groupMemberStateChangedHook(request);
    return GroupHookStatusOK;
}

static OnGroupInstanceResetRequestHook groupInstanceResetRequestHook;
void SetOnGroupInstanceResetRequestHook(OnGroupInstanceResetRequestHook h) {
    groupInstanceResetRequestHook = h;
}

int CallOnGroupInstanceResetRequestHook(GroupInstanceResetRequest* request) {
    if (groupInstanceResetRequestHook == 0)
        return GroupHookStatusNoHook;

    groupInstanceResetRequestHook(request);
    return GroupHookStatusOK;
}

static OnGroupInstanceBindExtensionRequestHook groupInstanceBindExtensionRequestHook;
void SetOnGroupInstanceBindExtensionRequestHook(OnGroupInstanceBindExtensionRequestHook h) {
    groupInstanceBindExtensionRequestHook = h;
}

int CallOnGroupInstanceBindExtensionRequestHook(GroupInstanceBindExtensionRequest* request) {
    if (groupInstanceBindExtensionRequestHook == 0)
        return GroupHookStatusNoHook;

    groupInstanceBindExtensionRequestHook(request);
    return GroupHookStatusOK;
}
