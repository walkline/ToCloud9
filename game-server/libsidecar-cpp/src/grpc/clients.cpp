#include "clients.h"
#include "servers-registry/registry.grpc.pb.h"
#include "guid/guid.grpc.pb.h"
#include "matchmaking/matchmaking.grpc.pb.h"
#include "group/group.grpc.pb.h"
#include "guilds/guilds.grpc.pb.h"
#include "tc9_version.h"
#include <spdlog/spdlog.h>

namespace tc9 {

namespace {
const char* LIB_VERSION = TC9_VERSION_STRING;
}

GrpcClients::GrpcClients() {
    spdlog::debug("GrpcClients created");
}

GrpcClients::~GrpcClients() {
    Shutdown();
}

void GrpcClients::Connect(const std::string& registry_addr,
                          const std::string& guid_addr,
                          const std::string& matchmaking_addr,
                          const std::string& group_addr,
                          const std::string& guild_addr) {
    std::lock_guard<std::mutex> lock(mutex_);

    spdlog::info("Connecting to gRPC services:");
    spdlog::info("  - Registry: {}", registry_addr);
    spdlog::info("  - GUID: {}", guid_addr);
    spdlog::info("  - Matchmaking: {}", matchmaking_addr);
    spdlog::info("  - Group: {}", group_addr);
    spdlog::info("  - Guild: {}", guild_addr);

    // Create channels (using insecure credentials for now)
    registry_channel_ = grpc::CreateChannel(
        registry_addr, grpc::InsecureChannelCredentials());
    guid_channel_ = grpc::CreateChannel(
        guid_addr, grpc::InsecureChannelCredentials());
    matchmaking_channel_ = grpc::CreateChannel(
        matchmaking_addr, grpc::InsecureChannelCredentials());
    group_channel_ = grpc::CreateChannel(
        group_addr, grpc::InsecureChannelCredentials());
    guild_channel_ = grpc::CreateChannel(
        guild_addr, grpc::InsecureChannelCredentials());

    // Create stubs
    registry_stub_ = v1::ServersRegistryService::NewStub(registry_channel_);
    guid_stub_ = v1::GuidService::NewStub(guid_channel_);
    matchmaking_stub_ = v1::MatchmakingService::NewStub(matchmaking_channel_);
    group_stub_ = v1::GroupService::NewStub(group_channel_);
    guild_stub_ = v1::GuildService::NewStub(guild_channel_);

    connected_ = true;
    spdlog::info("✅ All gRPC clients connected");
}

bool GrpcClients::AcceptGroupInvite(uint32_t realm_id, uint64_t player_guid) {
    if (!connected_ || !group_stub_) {
        spdlog::error("Group client not connected");
        return false;
    }

    v1::AcceptInviteParams request;
    request.set_api(LIB_VERSION);
    request.set_realmid(realm_id);
    request.set_player(player_guid);

    v1::AcceptInviteResponse response;
    grpc::ClientContext context;
    context.set_deadline(Deadline());

    grpc::Status status = group_stub_->AcceptInvite(&context, request, &response);
    if (!status.ok()) {
        spdlog::error("AcceptGroupInvite failed for player {}: {}",
                      player_guid, status.error_message());
        return false;
    }

    if (response.status() != v1::AcceptInviteResponse::Ok) {
        spdlog::warn("AcceptGroupInvite rejected for player {}: status {}",
                     player_guid, int(response.status()));
        return false;
    }

    return true;
}

bool GrpcClients::LeaveGroup(uint32_t realm_id, uint64_t player_guid) {
    if (!connected_ || !group_stub_) {
        spdlog::error("Group client not connected");
        return false;
    }

    v1::GroupLeaveParams request;
    request.set_api(LIB_VERSION);
    request.set_realmid(realm_id);
    request.set_player(player_guid);

    v1::GroupLeaveResponse response;
    grpc::ClientContext context;
    context.set_deadline(Deadline());

    grpc::Status status = group_stub_->Leave(&context, request, &response);
    if (!status.ok()) {
        spdlog::error("LeaveGroup failed for player {}: {}",
                      player_guid, status.error_message());
        return false;
    }

    return true;
}

void GrpcClients::Shutdown() {
    std::lock_guard<std::mutex> lock(mutex_);

    if (!connected_) {
        return;
    }

    spdlog::info("Shutting down gRPC clients");

    registry_stub_.reset();
    guid_stub_.reset();
    matchmaking_stub_.reset();
    group_stub_.reset();
    guild_stub_.reset();

    registry_channel_.reset();
    guid_channel_.reset();
    matchmaking_channel_.reset();
    group_channel_.reset();
    guild_channel_.reset();

    connected_ = false;
}

std::chrono::system_clock::time_point GrpcClients::Deadline(int seconds) {
    return std::chrono::system_clock::now() + std::chrono::seconds(seconds);
}

bool GrpcClients::RegisterGameServer(
    uint32_t game_port,
    uint32_t health_port,
    uint32_t grpc_port,
    uint32_t realm_id,
    bool is_cross_realm,
    const std::string& available_maps,
    const std::string& preferred_hostname,
    std::string& out_server_id,
    std::vector<uint32_t>& out_assigned_maps) {

    if (!connected_ || !registry_stub_) {
        spdlog::error("Registry client not connected");
        return false;
    }

    v1::RegisterGameServerRequest request;
    request.set_api(LIB_VERSION);
    request.set_gameport(game_port);
    request.set_healthport(health_port);
    request.set_grpcport(grpc_port);
    request.set_realmid(realm_id);
    request.set_iscrossrealm(is_cross_realm);
    request.set_availablemaps(available_maps);
    request.set_preferredhostname(preferred_hostname);

    v1::RegisterGameServerResponse response;
    grpc::ClientContext context;
    context.set_deadline(Deadline());

    grpc::Status status = registry_stub_->RegisterGameServer(&context, request, &response);

    if (!status.ok()) {
        spdlog::error("RegisterGameServer RPC failed: {} - {}",
                     status.error_code(), status.error_message());
        return false;
    }

    out_server_id = response.id();
    out_assigned_maps.clear();
    for (const auto& map_id : response.assignedmaps()) {
        out_assigned_maps.push_back(map_id);
    }

    spdlog::info("✅ Registered game server: ID={}, assigned {} maps",
                 out_server_id, out_assigned_maps.size());
    return true;
}

bool GrpcClients::GameServerMapsLoaded(
    const std::string& server_id,
    const std::vector<uint32_t>& maps_loaded) {

    if (!connected_ || !registry_stub_) {
        spdlog::error("Registry client not connected");
        return false;
    }

    v1::GameServerMapsLoadedRequest request;
    request.set_api(LIB_VERSION);
    request.set_gameserverid(server_id);
    for (const auto& map_id : maps_loaded) {
        request.add_mapsloaded(map_id);
    }

    v1::GameServerMapsLoadedResponse response;
    grpc::ClientContext context;
    context.set_deadline(Deadline());

    grpc::Status status = registry_stub_->GameServerMapsLoaded(&context, request, &response);

    if (!status.ok()) {
        spdlog::error("GameServerMapsLoaded RPC failed: {} - {}",
                     status.error_code(), status.error_message());
        return false;
    }

    spdlog::info("✅ Notified registry: {} maps loaded", maps_loaded.size());
    return true;
}

bool GrpcClients::RequestGUIDPool(
    uint32_t realm_id,
    int guid_type,
    uint64_t desired_pool_size,
    std::vector<std::pair<uint64_t, uint64_t>>& out_ranges) {

    if (!connected_ || !guid_stub_) {
        spdlog::error("GUID client not connected");
        return false;
    }

    v1::GetGUIDPoolRequest request;
    request.set_api(LIB_VERSION);
    request.set_realmid(realm_id);
    request.set_guidtype(static_cast<v1::GuidType>(guid_type));
    request.set_desiredpoolsize(desired_pool_size);

    v1::GetGUIDPoolRequestResponse response;
    grpc::ClientContext context;
    context.set_deadline(Deadline());

    grpc::Status status = guid_stub_->GetGUIDPool(&context, request, &response);

    if (!status.ok()) {
        spdlog::error("RequestGUIDPool RPC failed: {} - {}",
                     status.error_code(), status.error_message());
        return false;
    }

    out_ranges.clear();
    for (const auto& range : response.receiverguid()) {
        out_ranges.push_back({range.start(), range.end()});
    }

    uint64_t total_guids = 0;
    for (const auto& [start, end] : out_ranges) {
        total_guids += (end - start + 1);
    }

    spdlog::info("✅ Received GUID pool: {} ranges, {} total GUIDs",
                 out_ranges.size(), total_guids);
    return true;
}

bool GrpcClients::PlayerLeftBattleground(
    uint32_t realm_id,
    uint64_t player_guid,
    uint32_t instance_id,
    bool is_cross_realm) {

    if (!connected_ || !matchmaking_stub_) {
        spdlog::error("Matchmaking client not connected");
        return false;
    }

    v1::PlayerLeftBattlegroundRequest request;
    request.set_api(LIB_VERSION);
    request.set_realmid(realm_id);
    request.set_playerguid(player_guid);
    request.set_instanceid(instance_id);
    request.set_iscrossrealm(is_cross_realm);

    v1::PlayerLeftBattlegroundResponse response;
    grpc::ClientContext context;
    context.set_deadline(Deadline(2));  // Shorter timeout for async notification

    grpc::Status status = matchmaking_stub_->PlayerLeftBattleground(&context, request, &response);

    if (!status.ok()) {
        spdlog::warn("PlayerLeftBattleground RPC failed: {} - {}",
                    status.error_code(), status.error_message());
        return false;
    }

    spdlog::debug("Notified matchmaking: player {} left BG instance {}",
                 player_guid, instance_id);
    return true;
}

bool GrpcClients::BattlegroundStatusChanged(
    uint32_t realm_id,
    uint32_t instance_id,
    bool is_cross_realm,
    uint8_t status) {

    if (!connected_ || !matchmaking_stub_) {
        spdlog::error("Matchmaking client not connected");
        return false;
    }

    v1::BattlegroundStatusChangedRequest request;
    request.set_api(LIB_VERSION);
    request.set_realmid(realm_id);
    request.set_instanceid(instance_id);
    request.set_iscrossrealm(is_cross_realm);
    request.set_status(static_cast<v1::BattlegroundStatusChangedRequest_Status>(status));

    v1::BattlegroundStatusChangedResponse response;
    grpc::ClientContext context;
    context.set_deadline(Deadline(2));  // Shorter timeout for async notification

    grpc::Status status_result = matchmaking_stub_->BattlegroundStatusChanged(&context, request, &response);

    if (!status_result.ok()) {
        spdlog::warn("BattlegroundStatusChanged RPC failed: {} - {}",
                    status_result.error_code(), status_result.error_message());
        return false;
    }

    spdlog::debug("Notified matchmaking: BG instance {} status changed to {}",
                 instance_id, status);
    return true;
}

bool GrpcClients::AcceptGuildInvite(
    uint32_t realm_id,
    uint64_t guid,
    const std::string& name,
    uint32_t lvl,
    uint32_t race,
    uint32_t class_id,
    uint32_t gender,
    uint32_t area_id,
    uint64_t account_id) {

    if (!connected_ || !guild_stub_) {
        spdlog::error("Guild client not connected");
        return false;
    }

    v1::InviteAcceptedParams request;
    request.set_api(LIB_VERSION);
    request.set_realmid(realm_id);

    auto* character = request.mutable_character();
    character->set_guid(guid);
    character->set_name(name);
    character->set_lvl(lvl);
    character->set_race(race);
    character->set_classid(class_id);
    character->set_gender(gender);
    character->set_areaid(area_id);
    character->set_accountid(account_id);

    v1::InviteAcceptedResponse response;
    grpc::ClientContext context;
    context.set_deadline(Deadline());

    grpc::Status status = guild_stub_->InviteAccepted(&context, request, &response);
    if (!status.ok()) {
        spdlog::error("AcceptGuildInvite failed for player {}: {} - {}",
                      guid, status.error_code(), status.error_message());
        return false;
    }

    return true;
}

}  // namespace tc9
