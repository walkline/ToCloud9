#ifndef TC9_WORLDSERVER_SERVICE_H
#define TC9_WORLDSERVER_SERVICE_H

#include "worldserver/worldserver.grpc.pb.h"
#include "worldserver/worldserver.pb.h"
#include "libsidecar/tc9_types.h"
#include "../queue/handlers_queue.h"
#include <memory>
#include <chrono>

namespace tc9 {

// C++ bindings struct to hold C callback pointers
struct CppBindings {
    TC9GetPlayerItemsByGuidsHandler get_player_items = nullptr;
    TC9GetPlayerItemByPosHandler get_player_item_by_pos = nullptr;
    TC9RemoveItemsWithGuidsFromPlayerHandler remove_items = nullptr;
    TC9AddExistingItemToPlayerHandler add_item = nullptr;
    TC9GetMoneyForPlayerHandler get_money = nullptr;
    TC9ModifyMoneyForPlayerHandler modify_money = nullptr;
    TC9SetPlayerGuildFieldsHandler set_player_guild_fields = nullptr;
    TC9CanPlayerInteractWithNPCHandler interact_npc = nullptr;
    TC9CanPlayerInteractWithGOHandler interact_go = nullptr;
    TC9StartBattlegroundHandler start_bg = nullptr;
    TC9AddPlayersToBattlegroundHandler add_players_bg = nullptr;
    TC9CanPlayerJoinBattlegroundQueueHandler can_join_bg_queue = nullptr;
    TC9CanPlayerTeleportToBattlegroundHandler can_teleport_bg = nullptr;
    TC9MonitoringDataCollectorHandler monitoring_data_collector = nullptr;
};

class WorldServerServiceImpl : public v1::WorldServerService::Service {
public:
    WorldServerServiceImpl(
        const CppBindings& bindings,
        std::chrono::milliseconds timeout,
        HandlersQueue& read_queue,
        HandlersQueue& write_queue
    );

    // Items
    grpc::Status GetPlayerItemsByGuids(
        grpc::ServerContext* context,
        const v1::GetPlayerItemsByGuidsRequest* request,
        v1::GetPlayerItemsByGuidsResponse* response) override;

    grpc::Status GetPlayerItemByPos(
        grpc::ServerContext* context,
        const v1::GetPlayerItemByPosRequest* request,
        v1::GetPlayerItemByPosResponse* response) override;

    grpc::Status RemoveItemsWithGuidsFromPlayer(
        grpc::ServerContext* context,
        const v1::RemoveItemsWithGuidsFromPlayerRequest* request,
        v1::RemoveItemsWithGuidsFromPlayerResponse* response) override;

    grpc::Status AddExistingItemToPlayer(
        grpc::ServerContext* context,
        const v1::AddExistingItemToPlayerRequest* request,
        v1::AddExistingItemToPlayerResponse* response) override;

    // Money
    grpc::Status GetMoneyForPlayer(
        grpc::ServerContext* context,
        const v1::GetMoneyForPlayerRequest* request,
        v1::GetMoneyForPlayerResponse* response) override;

    grpc::Status ModifyMoneyForPlayer(
        grpc::ServerContext* context,
        const v1::ModifyMoneyForPlayerRequest* request,
        v1::ModifyMoneyForPlayerResponse* response) override;

    grpc::Status SetPlayerGuildFields(
        grpc::ServerContext* context,
        const v1::SetPlayerGuildFieldsRequest* request,
        v1::SetPlayerGuildFieldsResponse* response) override;

    // Interactions
    grpc::Status CanPlayerInteractWithNPC(
        grpc::ServerContext* context,
        const v1::CanPlayerInteractWithNPCRequest* request,
        v1::CanPlayerInteractWithNPCResponse* response) override;

    grpc::Status CanPlayerInteractWithGameObject(
        grpc::ServerContext* context,
        const v1::CanPlayerInteractWithGameObjectRequest* request,
        v1::CanPlayerInteractWithGameObjectResponse* response) override;

    // Battlegrounds
    grpc::Status StartBattleground(
        grpc::ServerContext* context,
        const v1::StartBattlegroundRequest* request,
        v1::StartBattlegroundResponse* response) override;

    grpc::Status AddPlayersToBattleground(
        grpc::ServerContext* context,
        const v1::AddPlayersToBattlegroundRequest* request,
        v1::AddPlayersToBattlegroundResponse* response) override;

    grpc::Status CanPlayerJoinBattlegroundQueue(
        grpc::ServerContext* context,
        const v1::CanPlayerJoinBattlegroundQueueRequest* request,
        v1::CanPlayerJoinBattlegroundQueueResponse* response) override;

    grpc::Status CanPlayerTeleportToBattleground(
        grpc::ServerContext* context,
        const v1::CanPlayerTeleportToBattlegroundRequest* request,
        v1::CanPlayerTeleportToBattlegroundResponse* response) override;

private:
    const CppBindings& bindings_;
    std::chrono::milliseconds timeout_;
    HandlersQueue& read_queue_;
    HandlersQueue& write_queue_;
    std::string lib_version_{"0.0.1"};
};

}  // namespace tc9

#endif  // TC9_WORLDSERVER_SERVICE_H
