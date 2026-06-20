#include "handlers.h"
#include "../events/event_hooks.h"
#include <nlohmann/json.hpp>
#include <spdlog/spdlog.h>
#include <vector>
#include <memory>
#include <cstring>

using json = nlohmann::json;

namespace tc9 {

// Group event handlers

std::unique_ptr<Handler> CreateGroupCreatedHandler(const std::string& data, uint32_t realm_id) {
    // Parse and capture members in the closure
    try {
        auto j = json::parse(data);

        // Check realm ID
        if (j.value("RealmID", 0u) != realm_id) {
            return std::make_unique<FunctionHandler>([]() {});
        }

        // Parse members array and capture in shared_ptr
        auto members_json = j.value("Members", json::array());
        auto members = std::make_shared<std::vector<uint64_t>>();
        for (const auto& member : members_json) {
            members->push_back(member.value("MemberGUID", 0ull));
        }

        uint32_t group_guid = j.value("GroupID", 0u);
        uint64_t leader_guid = j.value("LeaderGUID", 0ull);
        uint8_t loot_method = j.value("LootMethod", uint8_t(0));
        uint64_t looter_guid = j.value("LooterGUID", 0ull);

        return std::make_unique<FunctionHandler>([members, group_guid, leader_guid, loot_method, looter_guid]() {
            TC9EventGroupCreated event{};
            event.groupGuid = group_guid;
            event.leaderGuid = leader_guid;
            event.lootMethod = loot_method;
            event.looterGuid = looter_guid;
            event.memberGuids = members->empty() ? nullptr : members->data();
            event.memberCount = static_cast<int>(members->size());

            EventHooks::Instance().DispatchGroupCreated(event);
        });
    } catch (const std::exception& e) {
        spdlog::error("Failed to parse GroupCreated event: {}", e.what());
        return std::make_unique<FunctionHandler>([]() {});
    }
}

std::unique_ptr<Handler> CreateGroupMemberAddedHandler(const std::string& data, uint32_t realm_id) {
    return std::make_unique<FunctionHandler>([data, realm_id]() {
        try {
            auto j = json::parse(data);

            if (j.value("RealmID", 0u) != realm_id) {
                return;
            }

            TC9EventGroupMemberAdded event{};
            event.groupGuid = j.value("GroupID", 0u);
            event.memberGuid = j.value("MemberGUID", 0ull);

            EventHooks::Instance().DispatchGroupMemberAdded(event);
        } catch (const std::exception& e) {
            spdlog::error("Failed to parse GroupMemberAdded event: {}", e.what());
        }
    });
}

std::unique_ptr<Handler> CreateGroupMemberRemovedHandler(const std::string& data, uint32_t realm_id) {
    return std::make_unique<FunctionHandler>([data, realm_id]() {
        try {
            auto j = json::parse(data);

            if (j.value("RealmID", 0u) != realm_id) {
                return;
            }

            TC9EventGroupMemberRemoved event{};
            event.groupGuid = j.value("GroupID", 0u);
            event.memberGuid = j.value("MemberGUID", 0ull);

            EventHooks::Instance().DispatchGroupMemberRemoved(event);
        } catch (const std::exception& e) {
            spdlog::error("Failed to parse GroupMemberRemoved event: {}", e.what());
        }
    });
}

std::unique_ptr<Handler> CreateGroupDisbandedHandler(const std::string& data, uint32_t realm_id) {
    return std::make_unique<FunctionHandler>([data, realm_id]() {
        try {
            auto j = json::parse(data);

            if (j.value("RealmID", 0u) != realm_id) {
                return;
            }

            TC9EventGroupDisbanded event{};
            event.groupGuid = j.value("GroupID", 0u);

            EventHooks::Instance().DispatchGroupDisbanded(event);
        } catch (const std::exception& e) {
            spdlog::error("Failed to parse GroupDisbanded event: {}", e.what());
        }
    });
}

std::unique_ptr<Handler> CreateGroupLootTypeChangedHandler(const std::string& data, uint32_t realm_id) {
    return std::make_unique<FunctionHandler>([data, realm_id]() {
        try {
            auto j = json::parse(data);

            if (j.value("RealmID", 0u) != realm_id) {
                return;
            }

            TC9EventGroupLootTypeChanged event{};
            event.groupGuid = j.value("GroupID", 0u);
            event.lootMethod = j.value("NewLootType", uint8_t(0));
            event.looterGuid = j.value("NewLooterGUID", 0ull);

            EventHooks::Instance().DispatchGroupLootTypeChanged(event);
        } catch (const std::exception& e) {
            spdlog::error("Failed to parse GroupLootTypeChanged event: {}", e.what());
        }
    });
}

std::unique_ptr<Handler> CreateGroupDungeonDifficultyChangedHandler(const std::string& data, uint32_t realm_id) {
    return std::make_unique<FunctionHandler>([data, realm_id]() {
        try {
            auto j = json::parse(data);

            if (j.value("RealmID", 0u) != realm_id) {
                return;
            }

            // Check if DungeonDifficulty is present
            if (!j.contains("DungeonDifficulty")) {
                return;
            }

            TC9EventGroupDungeonDifficultyChanged event{};
            event.groupGuid = j.value("GroupID", 0u);
            event.difficulty = j.value("DungeonDifficulty", uint8_t(0));

            EventHooks::Instance().DispatchGroupDungeonDifficultyChanged(event);
        } catch (const std::exception& e) {
            spdlog::error("Failed to parse GroupDungeonDifficultyChanged event: {}", e.what());
        }
    });
}

std::unique_ptr<Handler> CreateGroupRaidDifficultyChangedHandler(const std::string& data, uint32_t realm_id) {
    return std::make_unique<FunctionHandler>([data, realm_id]() {
        try {
            auto j = json::parse(data);

            if (j.value("RealmID", 0u) != realm_id) {
                return;
            }

            // Check if RaidDifficulty is present
            if (!j.contains("RaidDifficulty")) {
                return;
            }

            TC9EventGroupRaidDifficultyChanged event{};
            event.groupGuid = j.value("GroupID", 0u);
            event.difficulty = j.value("RaidDifficulty", uint8_t(0));

            EventHooks::Instance().DispatchGroupRaidDifficultyChanged(event);
        } catch (const std::exception& e) {
            spdlog::error("Failed to parse GroupRaidDifficultyChanged event: {}", e.what());
        }
    });
}

std::unique_ptr<Handler> CreateGroupConvertedToRaidHandler(const std::string& data, uint32_t realm_id) {
    return std::make_unique<FunctionHandler>([data, realm_id]() {
        try {
            auto j = json::parse(data);

            if (j.value("RealmID", 0u) != realm_id) {
                return;
            }

            TC9EventGroupConvertedToRaid event{};
            event.groupGuid = j.value("GroupID", 0u);

            EventHooks::Instance().DispatchGroupConvertedToRaid(event);
        } catch (const std::exception& e) {
            spdlog::error("Failed to parse GroupConvertedToRaid event: {}", e.what());
        }
    });
}

// Guild event handlers

std::unique_ptr<Handler> CreateGuildMemberAddedHandler(const std::string& data, uint32_t realm_id) {
    return std::make_unique<FunctionHandler>([data, realm_id]() {
        try {
            auto j = json::parse(data);

            if (j.value("RealmID", 0u) != realm_id) {
                return;
            }

            TC9EventGuildMemberAdded event{};
            event.guildGuid = j.value("GuildID", 0ull);
            event.memberGuid = j.value("MemberGUID", 0ull);

            EventHooks::Instance().DispatchGuildMemberAdded(event);
        } catch (const std::exception& e) {
            spdlog::error("Failed to parse GuildMemberAdded event: {}", e.what());
        }
    });
}

std::unique_ptr<Handler> CreateGuildMemberLeftHandler(const std::string& data, uint32_t realm_id) {
    return std::make_unique<FunctionHandler>([data, realm_id]() {
        try {
            auto j = json::parse(data);

            if (j.value("RealmID", 0u) != realm_id) {
                return;
            }

            TC9EventGuildMemberLeft event{};
            event.guildGuid = j.value("GuildID", 0ull);
            event.memberGuid = j.value("MemberGUID", 0ull);

            EventHooks::Instance().DispatchGuildMemberLeft(event);
        } catch (const std::exception& e) {
            spdlog::error("Failed to parse GuildMemberLeft event: {}", e.what());
        }
    });
}

std::unique_ptr<Handler> CreateGuildMemberRemovedHandler(const std::string& data, uint32_t realm_id) {
    return std::make_unique<FunctionHandler>([data, realm_id]() {
        try {
            auto j = json::parse(data);

            if (j.value("RealmID", 0u) != realm_id) {
                return;
            }

            TC9EventGuildMemberRemoved event{};
            event.guildGuid = j.value("GuildID", 0ull);
            event.memberGuid = j.value("MemberGUID", 0ull);

            EventHooks::Instance().DispatchGuildMemberRemoved(event);
        } catch (const std::exception& e) {
            spdlog::error("Failed to parse GuildMemberRemoved event: {}", e.what());
        }
    });
}

// Registry event handlers

std::unique_ptr<Handler> CreateMapsReassignedHandler(const std::string& data) {
    try {
        auto j = json::parse(data);

        // Parse Servers array and find assigned maps
        auto assigned_maps = std::make_shared<std::vector<uint32_t>>();

        if (j.contains("Servers")) {
            for (const auto& server : j["Servers"]) {
                if (server.contains("NewAssignedMapsToHandle")) {
                    for (const auto& map_id : server["NewAssignedMapsToHandle"]) {
                        assigned_maps->push_back(map_id.get<uint32_t>());
                    }
                }
            }
        }

        return std::make_unique<FunctionHandler>([assigned_maps]() {
            TC9EventMapsReassigned event{};
            event.assignedMaps = assigned_maps->empty() ? nullptr : assigned_maps->data();
            event.assignedMapsCount = static_cast<int>(assigned_maps->size());

            EventHooks::Instance().DispatchMapsReassigned(event);
        });
    } catch (const std::exception& e) {
        spdlog::error("Failed to parse MapsReassigned event: {}", e.what());
        return std::make_unique<FunctionHandler>([]() {});
    }
}

}  // namespace tc9
