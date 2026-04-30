#include "worldserver_service.h"
#include <spdlog/spdlog.h>
#include <future>
#include <cstring>

namespace tc9 {

WorldServerServiceImpl::WorldServerServiceImpl(
    const CppBindings& bindings,
    std::chrono::milliseconds timeout,
    HandlersQueue& read_queue,
    HandlersQueue& write_queue)
    : bindings_(bindings)
    , timeout_(timeout)
    , read_queue_(read_queue)
    , write_queue_(write_queue) {
}

// Helper to convert C string to std::string (handles null)
static std::string SafeString(const char* str) {
    return str ? std::string(str) : std::string();
}

grpc::Status WorldServerServiceImpl::GetPlayerItemsByGuids(
    grpc::ServerContext* context,
    const v1::GetPlayerItemsByGuidsRequest* request,
    v1::GetPlayerItemsByGuidsResponse* response) {

    response->set_api(lib_version_);

    if (request->playerguid() == 0 || request->guids_size() == 0) {
        return grpc::Status::OK;
    }

    if (!bindings_.get_player_items) {
        spdlog::error("GetPlayerItemsByGuids: No handler registered");
        return grpc::Status(grpc::StatusCode::UNIMPLEMENTED, "No handler registered");
    }

    // Create promise/future for response
    auto promise = std::make_shared<std::promise<TC9GetPlayerItemsResponse>>();
    auto future = promise->get_future();

    // Convert request guids to C array
    std::vector<uint64_t> guids_vec;
    guids_vec.reserve(request->guids_size());
    for (const auto& guid : request->guids()) {
        guids_vec.push_back(guid);
    }

    // Push to read queue
    read_queue_.Push(MakeHandler([=]() {
        try {
            TC9GetPlayerItemsResponse resp = bindings_.get_player_items(
                request->playerguid(),
                const_cast<uint64_t*>(guids_vec.data()),
                static_cast<int>(guids_vec.size())
            );
            promise->set_value(resp);
        } catch (const std::exception& e) {
            spdlog::error("GetPlayerItemsByGuids handler threw: {}", e.what());
            promise->set_exception(std::current_exception());
        }
    }));

    // Wait for response with timeout
    if (future.wait_for(timeout_) == std::future_status::timeout) {
        spdlog::warn("GetPlayerItemsByGuids: Request timeout");
        return grpc::Status(grpc::StatusCode::DEADLINE_EXCEEDED, "Request timeout");
    }

    try {
        auto resp = future.get();

        if (resp.errorCode != TC9_ERROR_SUCCESS) {
            spdlog::error("GetPlayerItemsByGuids: Error code {}", resp.errorCode);
            return grpc::Status(grpc::StatusCode::INTERNAL, "Handler returned error");
        }

        // Convert items to protobuf
        for (int i = 0; i < resp.itemsSize; ++i) {
            auto* item = response->add_items();
            item->set_guid(resp.items[i].guid);
            item->set_entry(resp.items[i].entry);
            item->set_owner(resp.items[i].owner);
            item->set_bagslot(resp.items[i].bagSlot);
            item->set_slot(resp.items[i].slot);
            item->set_istradable(resp.items[i].isTradable);
            item->set_count(resp.items[i].count);
            item->set_flags(resp.items[i].flags);
            item->set_durability(resp.items[i].durability);
            item->set_randompropertyid(resp.items[i].randomPropertyID);
            item->set_text(SafeString(resp.items[i].text));

            // Free C string if allocated
            if (resp.items[i].text) {
                free(const_cast<char*>(resp.items[i].text));
            }
        }

        // Free items array
        if (resp.items) {
            free(resp.items);
        }

        return grpc::Status::OK;

    } catch (const std::exception& e) {
        spdlog::error("GetPlayerItemsByGuids: Exception: {}", e.what());
        return grpc::Status(grpc::StatusCode::INTERNAL, e.what());
    }
}

grpc::Status WorldServerServiceImpl::RemoveItemsWithGuidsFromPlayer(
    grpc::ServerContext* context,
    const v1::RemoveItemsWithGuidsFromPlayerRequest* request,
    v1::RemoveItemsWithGuidsFromPlayerResponse* response) {

    response->set_api(lib_version_);

    if (request->playerguid() == 0 || request->guids_size() == 0) {
        return grpc::Status::OK;
    }

    if (!bindings_.remove_items) {
        return grpc::Status(grpc::StatusCode::UNIMPLEMENTED, "No handler registered");
    }

    auto promise = std::make_shared<std::promise<TC9RemoveItemsResponse>>();
    auto future = promise->get_future();

    std::vector<uint64_t> guids_vec;
    guids_vec.reserve(request->guids_size());
    for (const auto& guid : request->guids()) {
        guids_vec.push_back(guid);
    }

    // Write operation - push to write queue
    write_queue_.Push(MakeHandler([=]() {
        try {
            TC9RemoveItemsResponse resp = bindings_.remove_items(
                request->playerguid(),
                const_cast<uint64_t*>(guids_vec.data()),
                static_cast<int>(guids_vec.size()),
                request->assigntoplayerguid()
            );
            promise->set_value(resp);
        } catch (...) {
            promise->set_exception(std::current_exception());
        }
    }));

    if (future.wait_for(timeout_) == std::future_status::timeout) {
        return grpc::Status(grpc::StatusCode::DEADLINE_EXCEEDED, "Request timeout");
    }

    try {
        auto resp = future.get();

        if (resp.errorCode != TC9_ERROR_SUCCESS) {
            return grpc::Status(grpc::StatusCode::INTERNAL, "Handler returned error");
        }

        for (int i = 0; i < resp.updatedItemsSize; ++i) {
            response->add_updateditemsguids(resp.updatedItems[i]);
        }

        if (resp.updatedItems) {
            free(resp.updatedItems);
        }

        return grpc::Status::OK;
    } catch (const std::exception& e) {
        return grpc::Status(grpc::StatusCode::INTERNAL, e.what());
    }
}

grpc::Status WorldServerServiceImpl::AddExistingItemToPlayer(
    grpc::ServerContext* context,
    const v1::AddExistingItemToPlayerRequest* request,
    v1::AddExistingItemToPlayerResponse* response) {

    response->set_api(lib_version_);

    if (request->playerguid() == 0) {
        response->set_status(v1::AddExistingItemToPlayerResponse::Success);
        return grpc::Status::OK;
    }

    if (!bindings_.add_item) {
        return grpc::Status(grpc::StatusCode::UNIMPLEMENTED, "No handler registered");
    }

    auto promise = std::make_shared<std::promise<TC9ErrorCode>>();
    auto future = promise->get_future();

    // Convert item
    TC9ItemToAdd item;
    item.guid = request->item().guid();
    item.entry = request->item().entry();
    item.count = request->item().count();
    item.flags = request->item().flags();
    item.durability = request->item().durability();
    item.randomPropertyID = request->item().randompropertyid();
    item.text = request->item().text().empty() ? nullptr : request->item().text().c_str();

    write_queue_.Push(MakeHandler([=]() {
        try {
            TC9ErrorCode err = bindings_.add_item(request->playerguid(), const_cast<TC9ItemToAdd*>(&item));
            promise->set_value(err);
        } catch (...) {
            promise->set_exception(std::current_exception());
        }
    }));

    if (future.wait_for(timeout_) == std::future_status::timeout) {
        return grpc::Status(grpc::StatusCode::DEADLINE_EXCEEDED, "Request timeout");
    }

    try {
        auto err = future.get();

        if (err == TC9_ERROR_NO_INVENTORY_SPACE) {
            response->set_status(v1::AddExistingItemToPlayerResponse::NoSpace);
        } else if (err == TC9_ERROR_SUCCESS) {
            response->set_status(v1::AddExistingItemToPlayerResponse::Success);
        } else {
            return grpc::Status(grpc::StatusCode::INTERNAL, "Handler returned error");
        }

        return grpc::Status::OK;
    } catch (const std::exception& e) {
        return grpc::Status(grpc::StatusCode::INTERNAL, e.what());
    }
}

grpc::Status WorldServerServiceImpl::GetMoneyForPlayer(
    grpc::ServerContext* context,
    const v1::GetMoneyForPlayerRequest* request,
    v1::GetMoneyForPlayerResponse* response) {

    response->set_api(lib_version_);

    if (!bindings_.get_money) {
        return grpc::Status(grpc::StatusCode::UNIMPLEMENTED, "No handler registered");
    }

    auto promise = std::make_shared<std::promise<std::pair<uint32_t, int>>>();
    auto future = promise->get_future();

    read_queue_.Push(MakeHandler([=]() {
        try {
            int error_code = 0;
            uint32_t money = bindings_.get_money(request->playerguid(), &error_code);
            promise->set_value({money, error_code});
        } catch (...) {
            promise->set_exception(std::current_exception());
        }
    }));

    if (future.wait_for(timeout_) == std::future_status::timeout) {
        return grpc::Status(grpc::StatusCode::DEADLINE_EXCEEDED, "Request timeout");
    }

    try {
        auto [money, error_code] = future.get();

        if (error_code != TC9_ERROR_SUCCESS) {
            return grpc::Status(grpc::StatusCode::INTERNAL, "Handler returned error");
        }

        response->set_money(money);
        return grpc::Status::OK;
    } catch (const std::exception& e) {
        return grpc::Status(grpc::StatusCode::INTERNAL, e.what());
    }
}

grpc::Status WorldServerServiceImpl::ModifyMoneyForPlayer(
    grpc::ServerContext* context,
    const v1::ModifyMoneyForPlayerRequest* request,
    v1::ModifyMoneyForPlayerResponse* response) {

    response->set_api(lib_version_);

    if (!bindings_.modify_money) {
        return grpc::Status(grpc::StatusCode::UNIMPLEMENTED, "No handler registered");
    }

    auto promise = std::make_shared<std::promise<std::pair<uint32_t, int>>>();
    auto future = promise->get_future();

    write_queue_.Push(MakeHandler([=]() {
        try {
            int error_code = 0;
            uint32_t new_money = bindings_.modify_money(
                request->playerguid(),
                request->value(),
                &error_code
            );
            promise->set_value({new_money, error_code});
        } catch (...) {
            promise->set_exception(std::current_exception());
        }
    }));

    if (future.wait_for(timeout_) == std::future_status::timeout) {
        return grpc::Status(grpc::StatusCode::DEADLINE_EXCEEDED, "Request timeout");
    }

    try {
        auto [new_money, error_code] = future.get();

        if (error_code != TC9_ERROR_SUCCESS) {
            return grpc::Status(grpc::StatusCode::INTERNAL, "Handler returned error");
        }

        response->set_newmoneyvalue(new_money);
        return grpc::Status::OK;
    } catch (const std::exception& e) {
        return grpc::Status(grpc::StatusCode::INTERNAL, e.what());
    }
}

grpc::Status WorldServerServiceImpl::CanPlayerInteractWithNPC(
    grpc::ServerContext* context,
    const v1::CanPlayerInteractWithNPCRequest* request,
    v1::CanPlayerInteractWithNPCResponse* response) {

    response->set_api(lib_version_);

    if (!bindings_.interact_npc) {
        return grpc::Status(grpc::StatusCode::UNIMPLEMENTED, "No handler registered");
    }

    auto promise = std::make_shared<std::promise<std::pair<bool, int>>>();
    auto future = promise->get_future();

    read_queue_.Push(MakeHandler([=]() {
        try {
            int error_code = 0;
            bool can_interact = bindings_.interact_npc(
                request->playerguid(),
                request->npcguid(),
                request->npcflags(),
                &error_code
            );
            promise->set_value({can_interact, error_code});
        } catch (...) {
            promise->set_exception(std::current_exception());
        }
    }));

    if (future.wait_for(timeout_) == std::future_status::timeout) {
        return grpc::Status(grpc::StatusCode::DEADLINE_EXCEEDED, "Request timeout");
    }

    try {
        auto [can_interact, error_code] = future.get();

        if (error_code != TC9_ERROR_SUCCESS) {
            return grpc::Status(grpc::StatusCode::INTERNAL, "Handler returned error");
        }

        response->set_caninteract(can_interact);
        return grpc::Status::OK;
    } catch (const std::exception& e) {
        return grpc::Status(grpc::StatusCode::INTERNAL, e.what());
    }
}

grpc::Status WorldServerServiceImpl::CanPlayerInteractWithGameObject(
    grpc::ServerContext* context,
    const v1::CanPlayerInteractWithGameObjectRequest* request,
    v1::CanPlayerInteractWithGameObjectResponse* response) {

    response->set_api(lib_version_);

    if (!bindings_.interact_go) {
        return grpc::Status(grpc::StatusCode::UNIMPLEMENTED, "No handler registered");
    }

    auto promise = std::make_shared<std::promise<std::pair<bool, int>>>();
    auto future = promise->get_future();

    read_queue_.Push(MakeHandler([=]() {
        try {
            int error_code = 0;
            bool can_interact = bindings_.interact_go(
                request->playerguid(),
                request->gameobjectguid(),
                request->gameobjecttype(),
                &error_code
            );
            promise->set_value({can_interact, error_code});
        } catch (...) {
            promise->set_exception(std::current_exception());
        }
    }));

    if (future.wait_for(timeout_) == std::future_status::timeout) {
        return grpc::Status(grpc::StatusCode::DEADLINE_EXCEEDED, "Request timeout");
    }

    try {
        auto [can_interact, error_code] = future.get();

        if (error_code != TC9_ERROR_SUCCESS) {
            return grpc::Status(grpc::StatusCode::INTERNAL, "Handler returned error");
        }

        response->set_caninteract(can_interact);
        return grpc::Status::OK;
    } catch (const std::exception& e) {
        return grpc::Status(grpc::StatusCode::INTERNAL, e.what());
    }
}

grpc::Status WorldServerServiceImpl::StartBattleground(
    grpc::ServerContext* context,
    const v1::StartBattlegroundRequest* request,
    v1::StartBattlegroundResponse* response) {

    response->set_api(lib_version_);

    if (!bindings_.start_bg) {
        return grpc::Status(grpc::StatusCode::UNIMPLEMENTED, "No handler registered");
    }

    auto promise = std::make_shared<std::promise<std::pair<TC9BattlegroundStartResponse, int>>>();
    auto future = promise->get_future();

    // Convert request
    TC9BattlegroundStartRequest bg_request;
    bg_request.battlegroundTypeID = request->battlegroundtypeid();
    bg_request.arenaType = request->arenatype();
    bg_request.isRated = request->israted();
    bg_request.mapID = request->mapid();
    bg_request.bracketLvl = request->bracketlvl();

    std::vector<uint64_t> horde_guids;
    std::vector<uint64_t> alliance_guids;

    for (const auto& guid : request->playerstoaddalliance()) {
        alliance_guids.push_back(guid);
    }
    for (const auto& guid : request->playerstoaddhorde()) {
        horde_guids.push_back(guid);
    }

    bg_request.hordePlayerGUIDs = horde_guids.data();
    bg_request.hordePlayerCount = static_cast<int>(horde_guids.size());
    bg_request.alliancePlayerGUIDs = alliance_guids.data();
    bg_request.alliancePlayerCount = static_cast<int>(alliance_guids.size());

    write_queue_.Push(MakeHandler([=]() mutable {
        try {
            int error_code = 0;
            TC9BattlegroundStartResponse bg_resp = bindings_.start_bg(&bg_request, &error_code);
            promise->set_value({bg_resp, error_code});
        } catch (...) {
            promise->set_exception(std::current_exception());
        }
    }));

    if (future.wait_for(timeout_) == std::future_status::timeout) {
        return grpc::Status(grpc::StatusCode::DEADLINE_EXCEEDED, "Request timeout");
    }

    try {
        auto [bg_resp, error_code] = future.get();

        if (error_code != TC9_ERROR_SUCCESS) {
            return grpc::Status(grpc::StatusCode::INTERNAL, "Handler returned error");
        }

        response->set_instanceid(bg_resp.instanceID);
        response->set_clientinstanceid(bg_resp.instanceClientID);
        return grpc::Status::OK;
    } catch (const std::exception& e) {
        return grpc::Status(grpc::StatusCode::INTERNAL, e.what());
    }
}

grpc::Status WorldServerServiceImpl::AddPlayersToBattleground(
    grpc::ServerContext* context,
    const v1::AddPlayersToBattlegroundRequest* request,
    v1::AddPlayersToBattlegroundResponse* response) {

    response->set_api(lib_version_);

    if (!bindings_.add_players_bg) {
        return grpc::Status(grpc::StatusCode::UNIMPLEMENTED, "No handler registered");
    }

    auto promise = std::make_shared<std::promise<TC9ErrorCode>>();
    auto future = promise->get_future();

    TC9BattlegroundAddPlayersRequest bg_request;
    bg_request.battlegroundTypeID = request->battlegroundtypeid();
    bg_request.instanceID = request->instanceid();

    std::vector<uint64_t> horde_guids;
    std::vector<uint64_t> alliance_guids;

    for (const auto& guid : request->playerstoaddalliance()) {
        alliance_guids.push_back(guid);
    }
    for (const auto& guid : request->playerstoaddhorde()) {
        horde_guids.push_back(guid);
    }

    bg_request.hordePlayerGUIDs = horde_guids.data();
    bg_request.hordePlayerCount = static_cast<int>(horde_guids.size());
    bg_request.alliancePlayerGUIDs = alliance_guids.data();
    bg_request.alliancePlayerCount = static_cast<int>(alliance_guids.size());

    write_queue_.Push(MakeHandler([=]() mutable {
        try {
            TC9ErrorCode err = bindings_.add_players_bg(&bg_request);
            promise->set_value(err);
        } catch (...) {
            promise->set_exception(std::current_exception());
        }
    }));

    if (future.wait_for(timeout_) == std::future_status::timeout) {
        return grpc::Status(grpc::StatusCode::DEADLINE_EXCEEDED, "Request timeout");
    }

    try {
        auto err = future.get();

        if (err != TC9_ERROR_SUCCESS) {
            return grpc::Status(grpc::StatusCode::INTERNAL, "Handler returned error");
        }

        return grpc::Status::OK;
    } catch (const std::exception& e) {
        return grpc::Status(grpc::StatusCode::INTERNAL, e.what());
    }
}

grpc::Status WorldServerServiceImpl::CanPlayerJoinBattlegroundQueue(
    grpc::ServerContext* context,
    const v1::CanPlayerJoinBattlegroundQueueRequest* request,
    v1::CanPlayerJoinBattlegroundQueueResponse* response) {

    response->set_api(lib_version_);

    if (!bindings_.can_join_bg_queue) {
        return grpc::Status(grpc::StatusCode::UNIMPLEMENTED, "No handler registered");
    }

    auto promise = std::make_shared<std::promise<TC9ErrorCode>>();
    auto future = promise->get_future();

    write_queue_.Push(MakeHandler([=]() {
        try {
            TC9ErrorCode err = bindings_.can_join_bg_queue(request->playerguid());
            promise->set_value(err);
        } catch (...) {
            promise->set_exception(std::current_exception());
        }
    }));

    if (future.wait_for(timeout_) == std::future_status::timeout) {
        return grpc::Status(grpc::StatusCode::DEADLINE_EXCEEDED, "Request timeout");
    }

    try {
        auto err = future.get();

        if (err != TC9_ERROR_SUCCESS) {
            return grpc::Status(grpc::StatusCode::INTERNAL, "Handler returned error");
        }

        response->set_status(v1::CanPlayerJoinBattlegroundQueueResponse::Success);
        return grpc::Status::OK;
    } catch (const std::exception& e) {
        return grpc::Status(grpc::StatusCode::INTERNAL, e.what());
    }
}

grpc::Status WorldServerServiceImpl::CanPlayerTeleportToBattleground(
    grpc::ServerContext* context,
    const v1::CanPlayerTeleportToBattlegroundRequest* request,
    v1::CanPlayerTeleportToBattlegroundResponse* response) {

    response->set_api(lib_version_);

    if (!bindings_.can_teleport_bg) {
        return grpc::Status(grpc::StatusCode::UNIMPLEMENTED, "No handler registered");
    }

    auto promise = std::make_shared<std::promise<TC9ErrorCode>>();
    auto future = promise->get_future();

    write_queue_.Push(MakeHandler([=]() {
        try {
            TC9ErrorCode err = bindings_.can_teleport_bg(request->playerguid());
            promise->set_value(err);
        } catch (...) {
            promise->set_exception(std::current_exception());
        }
    }));

    if (future.wait_for(timeout_) == std::future_status::timeout) {
        return grpc::Status(grpc::StatusCode::DEADLINE_EXCEEDED, "Request timeout");
    }

    try {
        auto err = future.get();

        if (err != TC9_ERROR_SUCCESS) {
            return grpc::Status(grpc::StatusCode::INTERNAL, "Handler returned error");
        }

        response->set_status(v1::CanPlayerTeleportToBattlegroundResponse::Success);
        return grpc::Status::OK;
    } catch (const std::exception& e) {
        return grpc::Status(grpc::StatusCode::INTERNAL, e.what());
    }
}

}  // namespace tc9
