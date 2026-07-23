#ifndef TC9_EVENT_HOOKS_H
#define TC9_EVENT_HOOKS_H

#include "libsidecar/tc9_events.h"
#include <mutex>

namespace tc9 {

/**
 * EventHooks - Singleton registry for NATS event callback handlers
 *
 * This class stores and dispatches callbacks for various game events
 * received via NATS messaging (groups, guilds, server registry).
 *
 * Thread Safety:
 * - Registration methods are NOT thread-safe (should only be called during init)
 * - Dispatch methods ARE thread-safe (can be called from event processing thread)
 */
class EventHooks {
public:
    static EventHooks& Instance();

    // Delete copy/move constructors
    EventHooks(const EventHooks&) = delete;
    EventHooks& operator=(const EventHooks&) = delete;
    EventHooks(EventHooks&&) = delete;
    EventHooks& operator=(EventHooks&&) = delete;

    // Group event registration
    void RegisterGroupCreated(TC9OnGroupCreatedHook hook);
    void RegisterGroupMemberAdded(TC9OnGroupMemberAddedHook hook);
    void RegisterGroupMemberRemoved(TC9OnGroupMemberRemovedHook hook);
    void RegisterGroupDisbanded(TC9OnGroupDisbandedHook hook);
    void RegisterGroupLootTypeChanged(TC9OnGroupLootTypeChangedHook hook);
    void RegisterGroupDungeonDifficultyChanged(TC9OnGroupDungeonDifficultyChangedHook hook);
    void RegisterGroupRaidDifficultyChanged(TC9OnGroupRaidDifficultyChangedHook hook);
    void RegisterGroupConvertedToRaid(TC9OnGroupConvertedToRaidHook hook);

    // Guild event registration
    void RegisterGuildMemberAdded(TC9OnGuildMemberAddedHook hook);
    void RegisterGuildMemberLeft(TC9OnGuildMemberLeftHook hook);
    void RegisterGuildMemberRemoved(TC9OnGuildMemberRemovedHook hook);
    void RegisterGuildCreated(TC9OnGuildCreatedHook hook);

    // Registry event registration
    void RegisterMapsReassigned(TC9OnMapsReassignedHook hook);

    // Group event dispatching
    void DispatchGroupCreated(const TC9EventGroupCreated& event);
    void DispatchGroupMemberAdded(const TC9EventGroupMemberAdded& event);
    void DispatchGroupMemberRemoved(const TC9EventGroupMemberRemoved& event);
    void DispatchGroupDisbanded(const TC9EventGroupDisbanded& event);
    void DispatchGroupLootTypeChanged(const TC9EventGroupLootTypeChanged& event);
    void DispatchGroupDungeonDifficultyChanged(const TC9EventGroupDungeonDifficultyChanged& event);
    void DispatchGroupRaidDifficultyChanged(const TC9EventGroupRaidDifficultyChanged& event);
    void DispatchGroupConvertedToRaid(const TC9EventGroupConvertedToRaid& event);

    // Guild event dispatching
    void DispatchGuildMemberAdded(const TC9EventGuildMemberAdded& event);
    void DispatchGuildMemberLeft(const TC9EventGuildMemberLeft& event);
    void DispatchGuildMemberRemoved(const TC9EventGuildMemberRemoved& event);
    void DispatchGuildCreated(const TC9EventGuildCreated& event);

    // Registry event dispatching
    void DispatchMapsReassigned(const TC9EventMapsReassigned& event);

private:
    EventHooks() = default;
    ~EventHooks() = default;

    std::mutex mutex_;

    // Group event hooks
    TC9OnGroupCreatedHook on_group_created_ = nullptr;
    TC9OnGroupMemberAddedHook on_group_member_added_ = nullptr;
    TC9OnGroupMemberRemovedHook on_group_member_removed_ = nullptr;
    TC9OnGroupDisbandedHook on_group_disbanded_ = nullptr;
    TC9OnGroupLootTypeChangedHook on_group_loot_type_changed_ = nullptr;
    TC9OnGroupDungeonDifficultyChangedHook on_group_dungeon_difficulty_changed_ = nullptr;
    TC9OnGroupRaidDifficultyChangedHook on_group_raid_difficulty_changed_ = nullptr;
    TC9OnGroupConvertedToRaidHook on_group_converted_to_raid_ = nullptr;

    // Guild event hooks
    TC9OnGuildMemberAddedHook on_guild_member_added_ = nullptr;
    TC9OnGuildMemberLeftHook on_guild_member_left_ = nullptr;
    TC9OnGuildMemberRemovedHook on_guild_member_removed_ = nullptr;
    TC9OnGuildCreatedHook on_guild_created_ = nullptr;

    // Registry event hooks
    TC9OnMapsReassignedHook on_maps_reassigned_ = nullptr;
};

}  // namespace tc9

#endif  // TC9_EVENT_HOOKS_H
