#include "libsidecar.h"
#include "events-group.h"
#include "events-guild.h"
#include "events-servers-registry.h"
#include "battleground-api.h"
#include "monitoring.h"
#include "player-interactions-api.h"
#include "player-items-api.h"
#include "player-money-api.h"

#include "libsidecar/tc9_types.h"
#include "libsidecar/tc9_events.h"
#include "core/config.h"
#include "core/logger.h"
#include "core/error.h"
#include "core/thread_pool.h"
#include "queue/handlers_queue.h"
#include "grpc/grpc_manager.h"
#include "grpc/clients.h"
#include "nats/consumer.h"
#include "nats/publisher.h"
#include "http/health_server.h"
#include "metrics/prometheus.h"
#include "guids/guid_manager.h"
#include "events/event_hooks.h"

#include <spdlog/spdlog.h>
#include <memory>
#include <cstring>

namespace {

// Global state
struct TC9State {
    std::unique_ptr<tc9::ThreadPool> read_thread_pool;
    std::unique_ptr<tc9::HandlersQueue> read_queue;
    std::unique_ptr<tc9::HandlersQueue> write_queue;
    std::unique_ptr<tc9::HandlersQueue> event_queue;
    std::unique_ptr<tc9::GrpcManager> grpc_manager;
    std::unique_ptr<tc9::GrpcClients> grpc_clients;
    std::unique_ptr<tc9::NatsConsumer> nats_consumer;
    std::unique_ptr<tc9::NatsPublisher> nats_publisher;
    std::unique_ptr<tc9::HealthServer> health_server;

    // Registered callbacks
    tc9::CppBindings bindings;

    bool initialized = false;
    std::string assigned_server_id;
};

TC9State g_state;

}  // anonymous namespace

extern "C" {

TC9_API void TC9InitLib(
    uint16_t port,
    uint32_t realmID,
    uint8_t isCrossRealm,
    char* availableMaps,
    uint32_t** assignedMaps,
    int* assignedMapsSize) {

    try {
        // Load configuration
        auto& config = tc9::Config::Instance();

        // Initialize logger
        tc9::InitLogger(config.log_level());
        spdlog::info("🚀 Initializing libsidecar v0.0.1");
        spdlog::info("Realm ID: {}, Cross-realm: {}, Port: {}",
                     realmID, isCrossRealm, port);
        spdlog::info("Available maps: {}", availableMaps ? availableMaps : "");

        // Create thread pools and queues
        spdlog::info("Creating thread pool with {} threads", config.read_threads());
        spdlog::info("Read processing mode: {}",
                     config.parallel_read_processing() ? "parallel" : "sequential (default)");
        g_state.read_thread_pool = std::make_unique<tc9::ThreadPool>(config.read_threads());
        g_state.read_queue = std::make_unique<tc9::HandlersQueue>();
        g_state.write_queue = std::make_unique<tc9::HandlersQueue>();
        g_state.event_queue = std::make_unique<tc9::HandlersQueue>();

        // Initialize metrics
        tc9::MetricsRegistry::Instance();

        // Create gRPC manager with bindings
        g_state.grpc_manager = std::make_unique<tc9::GrpcManager>(
            config.grpc_port(),
            g_state.bindings,
            *g_state.read_queue,
            *g_state.write_queue
        );

        // Create other services
        g_state.grpc_clients = std::make_unique<tc9::GrpcClients>();
        g_state.nats_consumer = std::make_unique<tc9::NatsConsumer>(config.nats_url());
        g_state.nats_publisher = std::make_unique<tc9::NatsPublisher>(config.nats_url());
        g_state.health_server = std::make_unique<tc9::HealthServer>(config.health_check_port());

        // Connect to external services
        g_state.grpc_clients->Connect(
            config.servers_registry_address(),
            config.guid_provider_address(),
            config.matchmaking_address()
        );

        // Start NATS consumer
        g_state.nats_consumer->SetEventQueue(g_state.event_queue.get());
        g_state.nats_consumer->SetRealmID(realmID);
        g_state.nats_consumer->Start();
        g_state.nats_publisher->Start();

        // Start gRPC server
        g_state.grpc_manager->Start();

        // Start health check server
        g_state.health_server->SetMonitoringDataCollector(g_state.bindings.monitoring_data_collector);
        g_state.health_server->SetReadQueue(g_state.read_queue.get());
        g_state.health_server->Start();

        // Register with servers-registry and get assigned maps
        std::string server_id;
        std::vector<uint32_t> assigned_maps_vec;

        bool registered = g_state.grpc_clients->RegisterGameServer(
            port,
            std::stoul(config.health_check_port()),
            std::stoul(config.grpc_port()),
            realmID,
            isCrossRealm,
            availableMaps ? availableMaps : "",
            "",  // preferred hostname (empty = auto)
            server_id,
            assigned_maps_vec
        );

        if (!registered) {
            spdlog::error("Failed to register with servers-registry");
            return;
        }

        g_state.assigned_server_id = server_id;

        // Initialize GUID manager
        auto& guid_mgr = tc9::GuidManager::Instance();
        guid_mgr.Initialize(g_state.grpc_clients.get(), realmID);

        // Convert vector to C array
        if (!assigned_maps_vec.empty()) {
            *assignedMaps = new uint32_t[assigned_maps_vec.size()];
            std::copy(assigned_maps_vec.begin(), assigned_maps_vec.end(), *assignedMaps);
            *assignedMapsSize = static_cast<int>(assigned_maps_vec.size());
        } else {
            *assignedMaps = nullptr;
            *assignedMapsSize = 0;
        }

        g_state.initialized = true;
        spdlog::info("✅ libsidecar initialized successfully (Server ID: {})", server_id);

    } catch (const tc9::TC9Exception& e) {
        spdlog::error("Initialization failed: {} (code: {})", e.what(), e.code());
    } catch (const std::exception& e) {
        spdlog::error("Initialization failed: {}", e.what());
    } catch (...) {
        spdlog::error("Initialization failed: unknown error");
    }
}

TC9_API void TC9GracefulShutdown() {
    try {
        if (!g_state.initialized) {
            return;
        }

        spdlog::info("🧨 Starting graceful shutdown");

        // Stop accepting new requests
        if (g_state.grpc_manager) {
            g_state.grpc_manager->Shutdown();
        }

        if (g_state.health_server) {
            g_state.health_server->Stop();
        }

        // Process remaining handlers
        spdlog::info("Processing remaining queued handlers");
        TC9ProcessGRPCOrHTTPRequests();
        TC9ProcessEventsHooks();

        // Shutdown services
        if (g_state.nats_consumer) {
            g_state.nats_consumer->Stop();
        }

        if (g_state.nats_publisher) {
            g_state.nats_publisher->Stop();
        }

        if (g_state.grpc_clients) {
            g_state.grpc_clients->Shutdown();
        }

        if (g_state.read_thread_pool) {
            g_state.read_thread_pool->Shutdown();
        }

        // Cleanup state
        g_state = TC9State();

        spdlog::info("👍 Graceful shutdown complete");
        tc9::ShutdownLogger();

    } catch (const std::exception& e) {
        spdlog::error("Error during shutdown: {}", e.what());
    }
}

TC9_API void TC9ProcessGRPCOrHTTPRequests() {
    if (!g_state.initialized) {
        return;
    }

    try {
        auto& config = tc9::Config::Instance();

        // Phase 1: Process read requests
        //
        // Configurable processing mode via TC9_PARALLEL_READ_PROCESSING:
        //   0 (default) = Sequential processing - optimal for fast operations (< 1μs each)
        //   1           = Parallel processing - beneficial only if operations are slow (> 100μs each)
        //
        // Benchmarks show that for typical memory lookups:
        //   - Sequential: ~0.1μs per request (100 requests = 11μs total)
        //   - Parallel:   ~1.0μs per request (100 requests = 97μs total, due to overhead)
        //
        // Parallel is only faster when individual operations are slow enough that
        // the speedup from parallelization outweighs the thread pool overhead.

        if (config.parallel_read_processing()) {
            // Parallel mode: Use thread pool
            std::vector<std::function<void()>> read_tasks;

            while (auto handler = g_state.read_queue->Pop()) {
                // Convert unique_ptr to shared_ptr for lambda capture
                auto h = std::shared_ptr<tc9::Handler>(std::move(handler));
                read_tasks.push_back([h]() {
                    h->Handle();
                });
            }

            if (!read_tasks.empty()) {
                g_state.read_thread_pool->ExecuteAll(read_tasks);
            }
        } else {
            // Sequential mode (default): Process on main thread
            while (auto handler = g_state.read_queue->Pop()) {
                handler->Handle();
            }
        }

        // Phase 2: Process write requests sequentially
        // Write operations are NOT thread-safe in AzerothCore, so they must
        // always be processed sequentially on the game loop thread.
        while (auto handler = g_state.write_queue->Pop()) {
            handler->Handle();
        }

    } catch (const std::exception& e) {
        spdlog::error("Error processing requests: {}", e.what());
    }
}

TC9_API void TC9ProcessEventsHooks() {
    if (!g_state.initialized) {
        return;
    }

    try {
        while (auto handler = g_state.event_queue->Pop()) {
            handler->Handle();
        }
    } catch (const std::exception& e) {
        spdlog::error("Error processing events: {}", e.what());
    }
}

TC9_API void TC9ReadyToAcceptPlayersFromMaps(uint32_t* maps, int mapsLen) {
    if (!g_state.initialized) {
        return;
    }

    try {
        std::vector<uint32_t> maps_vec(maps, maps + mapsLen);
        g_state.grpc_clients->GameServerMapsLoaded(g_state.assigned_server_id, maps_vec);
    } catch (const std::exception& e) {
        spdlog::error("Error notifying maps loaded: {}", e.what());
    }
}

TC9_API int TC9NatsPublish(const char* subject, const char* payload, int payloadLen) {
    if (!subject || payloadLen < 0 || (payloadLen > 0 && !payload)) {
        return -1;
    }
    if (!g_state.initialized || !g_state.nats_publisher) {
        return -1;
    }

    std::string data = payloadLen > 0 ? std::string(payload, payloadLen) : std::string();
    return g_state.nats_publisher->Publish(subject, data) ? 0 : -1;
}

TC9_API int TC9NatsSubscribe(const char* subject, TC9NatsMessageHandler handler) {
    if (!subject || !handler) {
        return -1;
    }
    if (!g_state.nats_consumer) {
        return -1;
    }

    return g_state.nats_consumer->SubscribeGeneric(
        subject,
        [handler](const std::string& subj, const std::string& payload) {
            handler(subj.c_str(), payload.data(), static_cast<int>(payload.size()));
        }) ? 0 : -1;
}

TC9_API void TC9PlayerLeftBattleground(
    uint64_t playerGUID,
    uint32_t realmID,
    uint32_t instanceID) {

    if (!g_state.initialized) {
        return;
    }

    try {
        g_state.grpc_clients->PlayerLeftBattleground(realmID, playerGUID, instanceID, false);
    } catch (const std::exception& e) {
        spdlog::error("Error notifying player left BG: {}", e.what());
    }
}

TC9_API void TC9BattlegroundStatusChanged(uint32_t instanceID, uint8_t status) {
    if (!g_state.initialized) {
        return;
    }

    try {
        // Realm ID is stored in config
        auto& config = tc9::Config::Instance();
        // Note: We'd need to pass realm ID to this function or store it globally
        // For now, using 0 as placeholder - this should be fixed in the API
        g_state.grpc_clients->BattlegroundStatusChanged(0, instanceID, false, status);
    } catch (const std::exception& e) {
        spdlog::error("Error notifying BG status changed: {}", e.what());
    }
}

// Handler registration functions - these accept old Go-style handlers
// and store them. The actual gRPC handlers will call these stored callbacks
// after converting from protocol buffer types to old Go types.

TC9_API void TC9SetGetPlayerItemsByGuidsHandler(GetPlayerItemsByGuidsHandler handler) {
    static GetPlayerItemsByGuidsHandler stored_handler = nullptr;
    stored_handler = handler;

    // Bridge to internal TC9 implementation
    g_state.bindings.get_player_items = [](uint64_t playerGuid, uint64_t* itemGuids, int itemGuidsSize) -> TC9GetPlayerItemsResponse {
        if (stored_handler) {
            GetPlayerItemsByGuidsResponse old_resp = stored_handler(playerGuid, itemGuids, itemGuidsSize);

            TC9GetPlayerItemsResponse resp;
            resp.errorCode = old_resp.errorCode;
            resp.itemsSize = old_resp.itemsSize;

            if (old_resp.itemsSize > 0 && old_resp.items) {
                resp.items = new TC9PlayerItem[old_resp.itemsSize];
                for (int i = 0; i < old_resp.itemsSize; i++) {
                    resp.items[i].guid = old_resp.items[i].guid;
                    resp.items[i].entry = old_resp.items[i].entry;
                    resp.items[i].owner = old_resp.items[i].owner;
                    resp.items[i].bagSlot = old_resp.items[i].bagSlot;
                    resp.items[i].slot = old_resp.items[i].slot;
                    resp.items[i].isTradable = old_resp.items[i].isTradable;
                    resp.items[i].count = old_resp.items[i].count;
                    resp.items[i].flags = old_resp.items[i].flags;
                    resp.items[i].durability = old_resp.items[i].durability;
                    resp.items[i].randomPropertyID = old_resp.items[i].randomPropertyID;
                    resp.items[i].text = old_resp.items[i].text;
                }
            } else {
                resp.items = nullptr;
            }

            return resp;
        }

        TC9GetPlayerItemsResponse resp{};
        resp.errorCode = TC9_ERROR_NO_HANDLER;
        return resp;
    };

    spdlog::debug("GetPlayerItemsByGuids handler registered");
}

TC9_API void TC9SetRemoveItemsWithGuidsFromPlayerHandler(RemoveItemsWithGuidsFromPlayerHandler handler) {
    static RemoveItemsWithGuidsFromPlayerHandler stored_handler = nullptr;
    stored_handler = handler;

    g_state.bindings.remove_items = [](uint64_t playerGuid, uint64_t* itemGuids, int itemGuidsSize, uint64_t assignToPlayerGuid) -> TC9RemoveItemsResponse {
        if (stored_handler) {
            RemoveItemsWithGuidsFromPlayerResponse old_resp = stored_handler(playerGuid, itemGuids, itemGuidsSize, assignToPlayerGuid);

            TC9RemoveItemsResponse resp;
            resp.errorCode = old_resp.errorCode;
            resp.updatedItemsSize = old_resp.updatedItemsSize;

            if (old_resp.updatedItemsSize > 0 && old_resp.updatedItems) {
                resp.updatedItems = new uint64_t[old_resp.updatedItemsSize];
                std::memcpy(resp.updatedItems, old_resp.updatedItems, old_resp.updatedItemsSize * sizeof(uint64_t));
            } else {
                resp.updatedItems = nullptr;
            }

            return resp;
        }

        TC9RemoveItemsResponse resp{};
        resp.errorCode = TC9_ERROR_NO_HANDLER;
        return resp;
    };

    spdlog::debug("RemoveItemsWithGuidsFromPlayer handler registered");
}

TC9_API void TC9SetAddExistingItemToPlayerHandler(AddExistingItemToPlayerHandler handler) {
    static AddExistingItemToPlayerHandler stored_handler = nullptr;
    stored_handler = handler;

    g_state.bindings.add_item = [](uint64_t playerGuid, TC9ItemToAdd* item) -> TC9ErrorCode {
        if (stored_handler) {
            AddExistingItemToPlayerRequest old_req;
            old_req.playerGuid = playerGuid;
            old_req.itemGuid = item->guid;
            old_req.itemEntry = item->entry;
            old_req.itemCount = item->count;
            old_req.itemFlags = item->flags;
            old_req.itemDurability = item->durability;
            old_req.itemRandomPropertyID = item->randomPropertyID;

            return static_cast<TC9ErrorCode>(stored_handler(&old_req));
        }

        return TC9_ERROR_NO_HANDLER;
    };

    spdlog::debug("AddExistingItemToPlayer handler registered");
}

TC9_API void TC9SetGetMoneyForPlayerHandler(GetMoneyForPlayerHandler handler) {
    static GetMoneyForPlayerHandler stored_handler = nullptr;
    stored_handler = handler;

    g_state.bindings.get_money = [](uint64_t playerGuid, int* errorCode) -> uint32_t {
        if (stored_handler) {
            GetMoneyForPlayerResponse old_resp = stored_handler(playerGuid);
            *errorCode = old_resp.errorCode;
            return old_resp.money;
        }

        *errorCode = TC9_ERROR_NO_HANDLER;
        return 0;
    };

    spdlog::debug("GetMoneyForPlayer handler registered");
}

TC9_API void TC9SetModifyMoneyForPlayerHandler(ModifyMoneyForPlayerHandler handler) {
    static ModifyMoneyForPlayerHandler stored_handler = nullptr;
    stored_handler = handler;

    g_state.bindings.modify_money = [](uint64_t playerGuid, int32_t value, int* errorCode) -> uint32_t {
        if (stored_handler) {
            ModifyMoneyForPlayerResponse old_resp = stored_handler(playerGuid, value);
            *errorCode = old_resp.errorCode;
            return old_resp.newMoneyValue;
        }

        *errorCode = TC9_ERROR_NO_HANDLER;
        return 0;
    };

    spdlog::debug("ModifyMoneyForPlayer handler registered");
}

TC9_API void TC9SetCanPlayerInteractWithNPCAndFlagsHandler(CanPlayerInteractWithNPCAndFlagsHandler handler) {
    static CanPlayerInteractWithNPCAndFlagsHandler stored_handler = nullptr;
    stored_handler = handler;

    g_state.bindings.interact_npc = [](uint64_t playerGuid, uint64_t npcGuid, uint32_t npcFlags, int* errorCode) -> bool {
        if (stored_handler) {
            CanPlayerInteractWithNPCAndFlagsResponse old_resp = stored_handler(playerGuid, npcGuid, npcFlags);
            *errorCode = old_resp.errorCode;
            return old_resp.canInteract;
        }

        *errorCode = TC9_ERROR_NO_HANDLER;
        return false;
    };

    spdlog::info("✅ CanPlayerInteractWithNPCAndFlags handler registered (handler ptr: {})",
                 handler ? "valid" : "NULL");
}

TC9_API void TC9SetCanPlayerInteractWithGOAndTypeHandler(CanPlayerInteractWithGOAndTypeHandler handler) {
    static CanPlayerInteractWithGOAndTypeHandler stored_handler = nullptr;
    stored_handler = handler;

    g_state.bindings.interact_go = [](uint64_t playerGuid, uint64_t goGuid, uint8_t goType, int* errorCode) -> bool {
        if (stored_handler) {
            CanPlayerInteractWithGOAndTypeResponse old_resp = stored_handler(playerGuid, goGuid, goType);
            *errorCode = old_resp.errorCode;
            return old_resp.canInteract;
        }

        *errorCode = TC9_ERROR_NO_HANDLER;
        return false;
    };

    spdlog::info("✅ CanPlayerInteractWithGOAndType handler registered (handler ptr: {})",
                 handler ? "valid" : "NULL");
}

TC9_API void TC9SetBattlegroundStartHandler(BattlegroundStartHandler handler) {
    static BattlegroundStartHandler stored_handler = nullptr;
    stored_handler = handler;

    g_state.bindings.start_bg = [](TC9BattlegroundStartRequest* req, int* errorCode) -> TC9BattlegroundStartResponse {
        if (stored_handler) {
            BattlegroundStartRequest old_req;
            old_req.battlegroundTypeID = req->battlegroundTypeID;
            old_req.arenaType = req->arenaType;
            old_req.isRated = req->isRated;
            old_req.mapID = req->mapID;
            old_req.bracketLvl = req->bracketLvl;
            old_req.hordePlayersToAdd = req->hordePlayerGUIDs;
            old_req.hordePlayersToAddSize = req->hordePlayerCount;
            old_req.alliancePlayersToAdd = req->alliancePlayerGUIDs;
            old_req.alliancePlayersToAddSize = req->alliancePlayerCount;
            old_req.randomBGPlayers = nullptr;
            old_req.randomBGPlayersSize = 0;

            BattlegroundStartResponse old_resp = stored_handler(&old_req);

            TC9BattlegroundStartResponse resp;
            resp.instanceID = old_resp.instanceID;
            resp.instanceClientID = old_resp.instanceClientID;
            *errorCode = old_resp.errorCode;
            return resp;
        }

        *errorCode = TC9_ERROR_NO_HANDLER;
        TC9BattlegroundStartResponse resp{};
        return resp;
    };

    spdlog::debug("BattlegroundStart handler registered");
}

TC9_API void TC9SetBattlegroundAddPlayersHandler(BattlegroundAddPlayersHandler handler) {
    static BattlegroundAddPlayersHandler stored_handler = nullptr;
    stored_handler = handler;

    g_state.bindings.add_players_bg = [](TC9BattlegroundAddPlayersRequest* req) -> TC9ErrorCode {
        if (stored_handler) {
            BattlegroundAddPlayersRequest old_req;
            old_req.battlegroundTypeID = req->battlegroundTypeID;
            old_req.instanceID = req->instanceID;
            old_req.hordePlayersToAdd = req->hordePlayerGUIDs;
            old_req.hordePlayersToAddSize = req->hordePlayerCount;
            old_req.alliancePlayersToAdd = req->alliancePlayerGUIDs;
            old_req.alliancePlayersToAddSize = req->alliancePlayerCount;
            old_req.randomBGPlayers = nullptr;
            old_req.randomBGPlayersSize = 0;

            return static_cast<TC9ErrorCode>(stored_handler(&old_req));
        }

        return TC9_ERROR_NO_HANDLER;
    };

    spdlog::debug("BattlegroundAddPlayers handler registered");
}

TC9_API void TC9SetCanPlayerJoinBattlegroundQueueHandler(CanPlayerJoinBattlegroundQueueHandler handler) {
    static CanPlayerJoinBattlegroundQueueHandler stored_handler = nullptr;
    stored_handler = handler;

    g_state.bindings.can_join_bg_queue = [](uint64_t playerGuid) -> TC9ErrorCode {
        if (stored_handler) {
            return static_cast<TC9ErrorCode>(stored_handler(playerGuid));
        }

        return TC9_ERROR_NO_HANDLER;
    };

    spdlog::debug("CanPlayerJoinBattlegroundQueue handler registered");
}

TC9_API void TC9SetCanPlayerTeleportToBattlegroundHandler(CanPlayerTeleportToBattlegroundHandler handler) {
    static CanPlayerTeleportToBattlegroundHandler stored_handler = nullptr;
    stored_handler = handler;

    g_state.bindings.can_teleport_bg = [](uint64_t playerGuid) -> TC9ErrorCode {
        if (stored_handler) {
            return static_cast<TC9ErrorCode>(stored_handler(playerGuid));
        }

        return TC9_ERROR_NO_HANDLER;
    };

    spdlog::debug("CanPlayerTeleportToBattleground handler registered");
}

// GUID generation functions

TC9_API uint64_t TC9GetNextAvailableCharacterGuid(int realmID) {
    if (!g_state.initialized) {
        return 0;
    }

    try {
        auto& guid_mgr = tc9::GuidManager::Instance();
        return guid_mgr.GetNextCharacterGuid(static_cast<uint32_t>(realmID));
    } catch (const std::exception& e) {
        spdlog::error("GetNextAvailableCharacterGuid failed: {}", e.what());
        return 0;
    }
}

TC9_API uint64_t TC9GetNextAvailableItemGuid(int realmID) {
    if (!g_state.initialized) {
        return 0;
    }

    try {
        auto& guid_mgr = tc9::GuidManager::Instance();
        return guid_mgr.GetNextItemGuid(static_cast<uint32_t>(realmID));
    } catch (const std::exception& e) {
        spdlog::error("GetNextAvailableItemGuid failed: {}", e.what());
        return 0;
    }
}

TC9_API uint64_t TC9GetNextAvailableInstanceGuid(int realmID) {
    if (!g_state.initialized) {
        return 0;
    }

    try {
        auto& guid_mgr = tc9::GuidManager::Instance();
        return guid_mgr.GetNextInstanceGuid(static_cast<uint32_t>(realmID));
    } catch (const std::exception& e) {
        spdlog::error("GetNextAvailableInstanceGuid failed: {}", e.what());
        return 0;
    }
}

// Event hook registration functions
// These accept old Go-style callbacks and register internal TC9 callbacks that bridge to them

TC9_API void TC9SetOnGroupCreatedHook(OnGroupCreatedHook hook) {
    // Store the old-style hook
    static OnGroupCreatedHook stored_hook = nullptr;
    stored_hook = hook;

    // Register internal hook that converts TC9 events to old format
    tc9::EventHooks::Instance().RegisterGroupCreated([](TC9EventGroupCreated event) {
        if (stored_hook) {
            EventObjectGroup group;
            group.guid = event.groupGuid;
            group.leader = event.leaderGuid;
            group.lootMethod = event.lootMethod;
            group.looterGuid = event.looterGuid;
            group.lootThreshold = 0;
            group.groupType = 0;
            group.difficulty = 0;
            group.raidDifficulty = 0;
            group.masterLooterGuid = 0;
            group.members = event.memberGuids;
            group.membersSize = static_cast<uint8_t>(event.memberCount);
            stored_hook(&group);
        }
    });
}

TC9_API void TC9SetOnGroupMemberAddedHook(OnGroupMemberAddedHook hook) {
    static OnGroupMemberAddedHook stored_hook = nullptr;
    stored_hook = hook;

    tc9::EventHooks::Instance().RegisterGroupMemberAdded([](TC9EventGroupMemberAdded event) {
        if (stored_hook) {
            stored_hook(event.groupGuid, event.memberGuid);
        }
    });
}

TC9_API void TC9SetOnGroupMemberRemovedHook(OnGroupMemberRemovedHook hook) {
    static OnGroupMemberRemovedHook stored_hook = nullptr;
    stored_hook = hook;

    tc9::EventHooks::Instance().RegisterGroupMemberRemoved([](TC9EventGroupMemberRemoved event) {
        if (stored_hook) {
            stored_hook(event.groupGuid, event.memberGuid, 0);  // newLeaderGuid not in TC9
        }
    });
}

TC9_API void TC9SetOnGroupDisbandedHook(OnGroupDisbandedHook hook) {
    static OnGroupDisbandedHook stored_hook = nullptr;
    stored_hook = hook;

    tc9::EventHooks::Instance().RegisterGroupDisbanded([](TC9EventGroupDisbanded event) {
        if (stored_hook) {
            stored_hook(event.groupGuid);
        }
    });
}

TC9_API void TC9SetOnGroupLootTypeChangedHook(OnGroupLootTypeChangedHook hook) {
    static OnGroupLootTypeChangedHook stored_hook = nullptr;
    stored_hook = hook;

    tc9::EventHooks::Instance().RegisterGroupLootTypeChanged([](TC9EventGroupLootTypeChanged event) {
        if (stored_hook) {
            stored_hook(event.groupGuid, event.lootMethod, event.looterGuid, 0);
        }
    });
}

TC9_API void TC9SetOnGroupDungeonDifficultyChangedHook(OnGroupDungeonDifficultyChangedHook hook) {
    static OnGroupDungeonDifficultyChangedHook stored_hook = nullptr;
    stored_hook = hook;

    tc9::EventHooks::Instance().RegisterGroupDungeonDifficultyChanged([](TC9EventGroupDungeonDifficultyChanged event) {
        if (stored_hook) {
            stored_hook(event.groupGuid, event.difficulty);
        }
    });
}

TC9_API void TC9SetOnGroupRaidDifficultyChangedHook(OnGroupRaidDifficultyChangedHook hook) {
    static OnGroupRaidDifficultyChangedHook stored_hook = nullptr;
    stored_hook = hook;

    tc9::EventHooks::Instance().RegisterGroupRaidDifficultyChanged([](TC9EventGroupRaidDifficultyChanged event) {
        if (stored_hook) {
            stored_hook(event.groupGuid, event.difficulty);
        }
    });
}

TC9_API void TC9SetOnGroupConvertedToRaidHook(OnGroupConvertedToRaidHook hook) {
    static OnGroupConvertedToRaidHook stored_hook = nullptr;
    stored_hook = hook;

    tc9::EventHooks::Instance().RegisterGroupConvertedToRaid([](TC9EventGroupConvertedToRaid event) {
        if (stored_hook) {
            stored_hook(event.groupGuid);
        }
    });
}

TC9_API void TC9SetOnGuildMemberAddedHook(OnGuildMemberAddedHook hook) {
    static OnGuildMemberAddedHook stored_hook = nullptr;
    stored_hook = hook;

    tc9::EventHooks::Instance().RegisterGuildMemberAdded([](TC9EventGuildMemberAdded event) {
        if (stored_hook) {
            stored_hook(event.guildGuid, event.memberGuid);
        }
    });
}

TC9_API void TC9SetOnGuildMemberLeftHook(OnGuildMemberLeftHook hook) {
    static OnGuildMemberLeftHook stored_hook = nullptr;
    stored_hook = hook;

    tc9::EventHooks::Instance().RegisterGuildMemberLeft([](TC9EventGuildMemberLeft event) {
        if (stored_hook) {
            stored_hook(event.guildGuid, event.memberGuid);
        }
    });
}

TC9_API void TC9SetOnGuildMemberRemovedHook(OnGuildMemberRemovedHook hook) {
    static OnGuildMemberRemovedHook stored_hook = nullptr;
    stored_hook = hook;

    tc9::EventHooks::Instance().RegisterGuildMemberRemoved([](TC9EventGuildMemberRemoved event) {
        if (stored_hook) {
            stored_hook(event.guildGuid, event.memberGuid);
        }
    });
}

TC9_API void TC9SetOnMapsReassignedHook(OnMapsReassignedHook hook) {
    static OnMapsReassignedHook stored_hook = nullptr;
    stored_hook = hook;

    tc9::EventHooks::Instance().RegisterMapsReassigned([](TC9EventMapsReassigned event) {
        if (stored_hook) {
            stored_hook(event.assignedMaps, event.assignedMapsCount, nullptr, 0);
        }
    });
}

// Monitoring data collector registration

TC9_API void TC9SetMonitoringDataCollectorHandler(MonitoringDataCollectorHandler handler) {
    static MonitoringDataCollectorHandler stored_handler = nullptr;
    stored_handler = handler;

    // Convert old-style handler to new TC9 style
    g_state.bindings.monitoring_data_collector = []() -> TC9MonitoringDataCollectorResponse {
        if (stored_handler) {
            MonitoringDataCollectorResponse old_resp = stored_handler();
            TC9MonitoringDataCollectorResponse resp;
            resp.errorCode = old_resp.errorCode;
            resp.connectedPlayers = old_resp.connectedPlayers;
            resp.diffMean = old_resp.diffMean;
            resp.diffMedian = old_resp.diffMedian;
            resp.diff95Percentile = old_resp.diff95Percentile;
            resp.diff99Percentile = old_resp.diff99Percentile;
            resp.diffMaxPercentile = old_resp.diffMaxPercentile;
            return resp;
        }
        TC9MonitoringDataCollectorResponse resp{};
        resp.errorCode = TC9_MONITORING_ERROR_NO_HANDLER;
        return resp;
    };

    // Update the health server with the new handler (if already initialized)
    if (g_state.health_server) {
        g_state.health_server->SetMonitoringDataCollector(g_state.bindings.monitoring_data_collector);
    }

    spdlog::debug("MonitoringDataCollector handler registered");
}

}  // extern "C"
