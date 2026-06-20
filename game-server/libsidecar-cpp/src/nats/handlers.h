#ifndef TC9_NATS_HANDLERS_H
#define TC9_NATS_HANDLERS_H

#include "../queue/handlers_queue.h"
#include <string>
#include <memory>

namespace tc9 {

// Group event handler factories
std::unique_ptr<Handler> CreateGroupCreatedHandler(const std::string& data, uint32_t realm_id);
std::unique_ptr<Handler> CreateGroupMemberAddedHandler(const std::string& data, uint32_t realm_id);
std::unique_ptr<Handler> CreateGroupMemberRemovedHandler(const std::string& data, uint32_t realm_id);
std::unique_ptr<Handler> CreateGroupDisbandedHandler(const std::string& data, uint32_t realm_id);
std::unique_ptr<Handler> CreateGroupLootTypeChangedHandler(const std::string& data, uint32_t realm_id);
std::unique_ptr<Handler> CreateGroupDungeonDifficultyChangedHandler(const std::string& data, uint32_t realm_id);
std::unique_ptr<Handler> CreateGroupRaidDifficultyChangedHandler(const std::string& data, uint32_t realm_id);
std::unique_ptr<Handler> CreateGroupConvertedToRaidHandler(const std::string& data, uint32_t realm_id);

// Guild event handler factories
std::unique_ptr<Handler> CreateGuildMemberAddedHandler(const std::string& data, uint32_t realm_id);
std::unique_ptr<Handler> CreateGuildMemberLeftHandler(const std::string& data, uint32_t realm_id);
std::unique_ptr<Handler> CreateGuildMemberRemovedHandler(const std::string& data, uint32_t realm_id);

// Registry event handler factory
std::unique_ptr<Handler> CreateMapsReassignedHandler(const std::string& data);

}  // namespace tc9

#endif  // TC9_NATS_HANDLERS_H
