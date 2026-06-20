#ifndef TC9_GRPC_CLIENTS_H
#define TC9_GRPC_CLIENTS_H

#include <string>
#include <vector>
#include <memory>
#include <mutex>
#include <grpcpp/grpcpp.h>
#include "servers-registry/registry.grpc.pb.h"
#include "guid/guid.grpc.pb.h"
#include "matchmaking/matchmaking.grpc.pb.h"

namespace tc9 {

class GrpcClients {
public:
    GrpcClients();
    ~GrpcClients();

    // Initialize connections to all services
    void Connect(const std::string& registry_addr,
                 const std::string& guid_addr,
                 const std::string& matchmaking_addr);

    // Servers Registry Client
    bool RegisterGameServer(
        uint32_t game_port,
        uint32_t health_port,
        uint32_t grpc_port,
        uint32_t realm_id,
        bool is_cross_realm,
        const std::string& available_maps,
        const std::string& preferred_hostname,
        std::string& out_server_id,
        std::vector<uint32_t>& out_assigned_maps);

    bool GameServerMapsLoaded(
        const std::string& server_id,
        const std::vector<uint32_t>& maps_loaded);

    // GUID Provider Client
    bool RequestGUIDPool(
        uint32_t realm_id,
        int guid_type,  // 0=Character, 1=Item, 2=Instance
        uint64_t desired_pool_size,
        std::vector<std::pair<uint64_t, uint64_t>>& out_ranges);

    // Matchmaking Client (async notifications)
    bool PlayerLeftBattleground(
        uint32_t realm_id,
        uint64_t player_guid,
        uint32_t instance_id,
        bool is_cross_realm);

    bool BattlegroundStatusChanged(
        uint32_t realm_id,
        uint32_t instance_id,
        bool is_cross_realm,
        uint8_t status);

    void Shutdown();

    GrpcClients(const GrpcClients&) = delete;
    GrpcClients& operator=(const GrpcClients&) = delete;

private:
    std::mutex mutex_;
    bool connected_ = false;

    // gRPC channels
    std::shared_ptr<grpc::Channel> registry_channel_;
    std::shared_ptr<grpc::Channel> guid_channel_;
    std::shared_ptr<grpc::Channel> matchmaking_channel_;

    // gRPC stubs
    std::unique_ptr<v1::ServersRegistryService::Stub> registry_stub_;
    std::unique_ptr<v1::GuidService::Stub> guid_stub_;
    std::unique_ptr<v1::MatchmakingService::Stub> matchmaking_stub_;

    // Helper to create deadline for requests
    std::chrono::system_clock::time_point Deadline(int seconds = 5);
};

}  // namespace tc9

#endif  // TC9_GRPC_CLIENTS_H
