#include "event_hooks.h"
#include <spdlog/spdlog.h>

namespace tc9 {

EventHooks& EventHooks::Instance() {
    static EventHooks instance;
    return instance;
}

// Group event registration

void EventHooks::RegisterGroupCreated(TC9OnGroupCreatedHook hook) {
    on_group_created_ = hook;
    spdlog::debug("Registered OnGroupCreated hook");
}

void EventHooks::RegisterGroupMemberAdded(TC9OnGroupMemberAddedHook hook) {
    on_group_member_added_ = hook;
    spdlog::debug("Registered OnGroupMemberAdded hook");
}

void EventHooks::RegisterGroupMemberRemoved(TC9OnGroupMemberRemovedHook hook) {
    on_group_member_removed_ = hook;
    spdlog::debug("Registered OnGroupMemberRemoved hook");
}

void EventHooks::RegisterGroupDisbanded(TC9OnGroupDisbandedHook hook) {
    on_group_disbanded_ = hook;
    spdlog::debug("Registered OnGroupDisbanded hook");
}

void EventHooks::RegisterGroupLootTypeChanged(TC9OnGroupLootTypeChangedHook hook) {
    on_group_loot_type_changed_ = hook;
    spdlog::debug("Registered OnGroupLootTypeChanged hook");
}

void EventHooks::RegisterGroupDungeonDifficultyChanged(TC9OnGroupDungeonDifficultyChangedHook hook) {
    on_group_dungeon_difficulty_changed_ = hook;
    spdlog::debug("Registered OnGroupDungeonDifficultyChanged hook");
}

void EventHooks::RegisterGroupRaidDifficultyChanged(TC9OnGroupRaidDifficultyChangedHook hook) {
    on_group_raid_difficulty_changed_ = hook;
    spdlog::debug("Registered OnGroupRaidDifficultyChanged hook");
}

void EventHooks::RegisterGroupConvertedToRaid(TC9OnGroupConvertedToRaidHook hook) {
    on_group_converted_to_raid_ = hook;
    spdlog::debug("Registered OnGroupConvertedToRaid hook");
}

// Guild event registration

void EventHooks::RegisterGuildMemberAdded(TC9OnGuildMemberAddedHook hook) {
    on_guild_member_added_ = hook;
    spdlog::debug("Registered OnGuildMemberAdded hook");
}

void EventHooks::RegisterGuildMemberLeft(TC9OnGuildMemberLeftHook hook) {
    on_guild_member_left_ = hook;
    spdlog::debug("Registered OnGuildMemberLeft hook");
}

void EventHooks::RegisterGuildMemberRemoved(TC9OnGuildMemberRemovedHook hook) {
    on_guild_member_removed_ = hook;
    spdlog::debug("Registered OnGuildMemberRemoved hook");
}

void EventHooks::RegisterGuildCreated(TC9OnGuildCreatedHook hook) {
    on_guild_created_ = hook;
    spdlog::debug("Registered OnGuildCreated hook");
}

// Registry event registration

void EventHooks::RegisterMapsReassigned(TC9OnMapsReassignedHook hook) {
    on_maps_reassigned_ = hook;
    spdlog::debug("Registered OnMapsReassigned hook");
}

// Group event dispatching

void EventHooks::DispatchGroupCreated(const TC9EventGroupCreated& event) {
    std::lock_guard<std::mutex> lock(mutex_);
    if (on_group_created_) {
        try {
            on_group_created_(event);
        } catch (const std::exception& e) {
            spdlog::error("Error in OnGroupCreated hook: {}", e.what());
        } catch (...) {
            spdlog::error("Unknown error in OnGroupCreated hook");
        }
    }
}

void EventHooks::DispatchGroupMemberAdded(const TC9EventGroupMemberAdded& event) {
    std::lock_guard<std::mutex> lock(mutex_);
    if (on_group_member_added_) {
        try {
            on_group_member_added_(event);
        } catch (const std::exception& e) {
            spdlog::error("Error in OnGroupMemberAdded hook: {}", e.what());
        } catch (...) {
            spdlog::error("Unknown error in OnGroupMemberAdded hook");
        }
    }
}

void EventHooks::DispatchGroupMemberRemoved(const TC9EventGroupMemberRemoved& event) {
    std::lock_guard<std::mutex> lock(mutex_);
    if (on_group_member_removed_) {
        try {
            on_group_member_removed_(event);
        } catch (const std::exception& e) {
            spdlog::error("Error in OnGroupMemberRemoved hook: {}", e.what());
        } catch (...) {
            spdlog::error("Unknown error in OnGroupMemberRemoved hook");
        }
    }
}

void EventHooks::DispatchGroupDisbanded(const TC9EventGroupDisbanded& event) {
    std::lock_guard<std::mutex> lock(mutex_);
    if (on_group_disbanded_) {
        try {
            on_group_disbanded_(event);
        } catch (const std::exception& e) {
            spdlog::error("Error in OnGroupDisbanded hook: {}", e.what());
        } catch (...) {
            spdlog::error("Unknown error in OnGroupDisbanded hook");
        }
    }
}

void EventHooks::DispatchGroupLootTypeChanged(const TC9EventGroupLootTypeChanged& event) {
    std::lock_guard<std::mutex> lock(mutex_);
    if (on_group_loot_type_changed_) {
        try {
            on_group_loot_type_changed_(event);
        } catch (const std::exception& e) {
            spdlog::error("Error in OnGroupLootTypeChanged hook: {}", e.what());
        } catch (...) {
            spdlog::error("Unknown error in OnGroupLootTypeChanged hook");
        }
    }
}

void EventHooks::DispatchGroupDungeonDifficultyChanged(const TC9EventGroupDungeonDifficultyChanged& event) {
    std::lock_guard<std::mutex> lock(mutex_);
    if (on_group_dungeon_difficulty_changed_) {
        try {
            on_group_dungeon_difficulty_changed_(event);
        } catch (const std::exception& e) {
            spdlog::error("Error in OnGroupDungeonDifficultyChanged hook: {}", e.what());
        } catch (...) {
            spdlog::error("Unknown error in OnGroupDungeonDifficultyChanged hook");
        }
    }
}

void EventHooks::DispatchGroupRaidDifficultyChanged(const TC9EventGroupRaidDifficultyChanged& event) {
    std::lock_guard<std::mutex> lock(mutex_);
    if (on_group_raid_difficulty_changed_) {
        try {
            on_group_raid_difficulty_changed_(event);
        } catch (const std::exception& e) {
            spdlog::error("Error in OnGroupRaidDifficultyChanged hook: {}", e.what());
        } catch (...) {
            spdlog::error("Unknown error in OnGroupRaidDifficultyChanged hook");
        }
    }
}

void EventHooks::DispatchGroupConvertedToRaid(const TC9EventGroupConvertedToRaid& event) {
    std::lock_guard<std::mutex> lock(mutex_);
    if (on_group_converted_to_raid_) {
        try {
            on_group_converted_to_raid_(event);
        } catch (const std::exception& e) {
            spdlog::error("Error in OnGroupConvertedToRaid hook: {}", e.what());
        } catch (...) {
            spdlog::error("Unknown error in OnGroupConvertedToRaid hook");
        }
    }
}

// Guild event dispatching

void EventHooks::DispatchGuildMemberAdded(const TC9EventGuildMemberAdded& event) {
    std::lock_guard<std::mutex> lock(mutex_);
    if (on_guild_member_added_) {
        try {
            on_guild_member_added_(event);
        } catch (const std::exception& e) {
            spdlog::error("Error in OnGuildMemberAdded hook: {}", e.what());
        } catch (...) {
            spdlog::error("Unknown error in OnGuildMemberAdded hook");
        }
    }
}

void EventHooks::DispatchGuildMemberLeft(const TC9EventGuildMemberLeft& event) {
    std::lock_guard<std::mutex> lock(mutex_);
    if (on_guild_member_left_) {
        try {
            on_guild_member_left_(event);
        } catch (const std::exception& e) {
            spdlog::error("Error in OnGuildMemberLeft hook: {}", e.what());
        } catch (...) {
            spdlog::error("Unknown error in OnGuildMemberLeft hook");
        }
    }
}

void EventHooks::DispatchGuildMemberRemoved(const TC9EventGuildMemberRemoved& event) {
    std::lock_guard<std::mutex> lock(mutex_);
    if (on_guild_member_removed_) {
        try {
            on_guild_member_removed_(event);
        } catch (const std::exception& e) {
            spdlog::error("Error in OnGuildMemberRemoved hook: {}", e.what());
        } catch (...) {
            spdlog::error("Unknown error in OnGuildMemberRemoved hook");
        }
    }
}

void EventHooks::DispatchGuildCreated(const TC9EventGuildCreated& event) {
    std::lock_guard<std::mutex> lock(mutex_);
    if (on_guild_created_) {
        try {
            on_guild_created_(event);
        } catch (const std::exception& e) {
            spdlog::error("Error in OnGuildCreated hook: {}", e.what());
        } catch (...) {
            spdlog::error("Unknown error in OnGuildCreated hook");
        }
    }
}

// Registry event dispatching

void EventHooks::DispatchMapsReassigned(const TC9EventMapsReassigned& event) {
    std::lock_guard<std::mutex> lock(mutex_);
    if (on_maps_reassigned_) {
        try {
            on_maps_reassigned_(event);
        } catch (const std::exception& e) {
            spdlog::error("Error in OnMapsReassigned hook: {}", e.what());
        } catch (...) {
            spdlog::error("Unknown error in OnMapsReassigned hook");
        }
    }
}

}  // namespace tc9
