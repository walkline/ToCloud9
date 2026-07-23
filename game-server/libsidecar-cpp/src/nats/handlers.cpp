#include "handlers.h"
#include "../events/event_hooks.h"
#include <nlohmann/json.hpp>
#include <spdlog/spdlog.h>
#include <algorithm>
#include <vector>
#include <memory>
#include <cstring>

using json = nlohmann::json;

namespace tc9 {

namespace {

// Every Go producer wraps events in the EventToSendGenericPayload envelope
// {"v":version,"t":eventType,"p":payload}; handlers consume the payload.
json ParseEventPayload(const std::string& data) {
    auto j = json::parse(data);
    auto it = j.find("p");
    if (it != j.end() && it->is_object()) {
        return *it;
    }
    return j;
}

}  // anonymous namespace

// Group event handlers

std::unique_ptr<Handler> CreateGroupCreatedHandler(const std::string& data, uint32_t realm_id) {
    // Parse and capture members in the closure
    try {
        auto j = ParseEventPayload(data);

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
        uint8_t loot_threshold = j.value("LootThreshold", uint8_t(0));
        uint8_t group_type = j.value("GroupType", uint8_t(0));
        uint8_t difficulty = j.value("Difficulty", uint8_t(0));
        uint8_t raid_difficulty = j.value("RaidDifficulty", uint8_t(0));
        uint64_t master_looter_guid = j.value("MasterLooterGuid", 0ull);

        return std::make_unique<FunctionHandler>([members, group_guid, leader_guid, loot_method, looter_guid,
                                                  loot_threshold, group_type, difficulty, raid_difficulty,
                                                  master_looter_guid]() {
            TC9EventGroupCreated event{};
            event.groupGuid = group_guid;
            event.leaderGuid = leader_guid;
            event.lootMethod = loot_method;
            event.looterGuid = looter_guid;
            event.lootThreshold = loot_threshold;
            event.groupType = group_type;
            event.difficulty = difficulty;
            event.raidDifficulty = raid_difficulty;
            event.masterLooterGuid = master_looter_guid;
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
            auto j = ParseEventPayload(data);

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
            auto j = ParseEventPayload(data);

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
            auto j = ParseEventPayload(data);

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
            auto j = ParseEventPayload(data);

            if (j.value("RealmID", 0u) != realm_id) {
                return;
            }

            TC9EventGroupLootTypeChanged event{};
            event.groupGuid = j.value("GroupID", 0u);
            event.lootMethod = j.value("NewLootType", uint8_t(0));
            event.looterGuid = j.value("NewLooterGUID", 0ull);
            event.lootThreshold = j.value("NewLooterThreshold", uint8_t(0));

            EventHooks::Instance().DispatchGroupLootTypeChanged(event);
        } catch (const std::exception& e) {
            spdlog::error("Failed to parse GroupLootTypeChanged event: {}", e.what());
        }
    });
}

std::unique_ptr<Handler> CreateGroupDungeonDifficultyChangedHandler(const std::string& data, uint32_t realm_id) {
    return std::make_unique<FunctionHandler>([data, realm_id]() {
        try {
            auto j = ParseEventPayload(data);

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
            auto j = ParseEventPayload(data);

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
            auto j = ParseEventPayload(data);

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
            auto j = ParseEventPayload(data);

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

std::unique_ptr<Handler> CreateGuildCreatedHandler(const std::string& data, uint32_t realm_id) {
    return std::make_unique<FunctionHandler>([data, realm_id]() {
        try {
            auto j = ParseEventPayload(data);

            if (j.value("RealmID", 0u) != realm_id) {
                return;
            }

            std::string guildName = j.value("GuildName", std::string());
            std::vector<uint64_t> memberGuids;
            if (j.contains("MemberGUIDs") && j["MemberGUIDs"].is_array()) {
                memberGuids = j["MemberGUIDs"].get<std::vector<uint64_t>>();
            }

            TC9EventGuildCreated event{};
            event.guildGuid = j.value("GuildID", 0ull);
            event.guildName = guildName.c_str();
            event.leaderGuid = j.value("LeaderGUID", 0ull);
            event.memberGuids = memberGuids.empty() ? nullptr : memberGuids.data();
            event.memberGuidsCount = static_cast<int>(memberGuids.size());

            // Dispatch is synchronous, the hook must copy name/members.
            EventHooks::Instance().DispatchGuildCreated(event);
        } catch (const std::exception& e) {
            spdlog::error("Failed to parse GuildCreated event: {}", e.what());
        }
    });
}

std::unique_ptr<Handler> CreateGuildMemberLeftHandler(const std::string& data, uint32_t realm_id) {
    return std::make_unique<FunctionHandler>([data, realm_id]() {
        try {
            auto j = ParseEventPayload(data);

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
            auto j = ParseEventPayload(data);

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

std::unique_ptr<Handler> CreateMapsReassignedHandler(const std::string& data, const std::string& own_server_id) {
    try {
        auto j = ParseEventPayload(data);

        // The payload carries every server's assignment: apply only our own
        // entry's old->new diff. A server that applies foreign maps believes
        // it owns the whole cluster and breaks the maps partition.
        auto added = std::make_shared<std::vector<uint32_t>>();
        auto removed = std::make_shared<std::vector<uint32_t>>();

        if (!own_server_id.empty() && j.contains("Servers")) {
            for (const auto& server : j["Servers"]) {
                if (server.value("ID", std::string()) != own_server_id) {
                    continue;
                }

                std::vector<uint32_t> old_maps;
                std::vector<uint32_t> new_maps;
                if (server.contains("OldAssignedMapsToHandle") && server["OldAssignedMapsToHandle"].is_array()) {
                    for (const auto& map_id : server["OldAssignedMapsToHandle"]) {
                        old_maps.push_back(map_id.get<uint32_t>());
                    }
                }
                if (server.contains("NewAssignedMapsToHandle") && server["NewAssignedMapsToHandle"].is_array()) {
                    for (const auto& map_id : server["NewAssignedMapsToHandle"]) {
                        new_maps.push_back(map_id.get<uint32_t>());
                    }
                }

                // Startup assignment already comes from the registration
                // response (same rule as the Go libsidecar handler).
                if (old_maps.empty()) {
                    break;
                }

                for (uint32_t map_id : new_maps) {
                    if (std::find(old_maps.begin(), old_maps.end(), map_id) == old_maps.end()) {
                        added->push_back(map_id);
                    }
                }
                for (uint32_t map_id : old_maps) {
                    if (std::find(new_maps.begin(), new_maps.end(), map_id) == new_maps.end()) {
                        removed->push_back(map_id);
                    }
                }
                break;
            }
        }

        if (added->empty() && removed->empty()) {
            return std::make_unique<FunctionHandler>([]() {});
        }

        return std::make_unique<FunctionHandler>([added, removed]() {
            TC9EventMapsReassigned event{};
            event.assignedMaps = added->empty() ? nullptr : added->data();
            event.assignedMapsCount = static_cast<int>(added->size());
            event.removedMaps = removed->empty() ? nullptr : removed->data();
            event.removedMapsCount = static_cast<int>(removed->size());

            EventHooks::Instance().DispatchMapsReassigned(event);
        });
    } catch (const std::exception& e) {
        spdlog::error("Failed to parse MapsReassigned event: {}", e.what());
        return std::make_unique<FunctionHandler>([]() {});
    }
}

}  // namespace tc9
