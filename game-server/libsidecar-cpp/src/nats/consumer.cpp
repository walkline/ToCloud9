#include "consumer.h"
#include "handlers.h"
#include "../queue/handlers_queue.h"
#include <spdlog/spdlog.h>
#include <unordered_map>
#include <functional>

namespace tc9 {

NatsConsumer::NatsConsumer(const std::string& url) : url_(url) {
    spdlog::debug("NATS consumer created for URL: {}", url);
}

NatsConsumer::~NatsConsumer() {
    Stop();
}

void NatsConsumer::Start() {
    std::lock_guard<std::mutex> lock(mutex_);

    if (connected_) {
        spdlog::warn("NATS consumer already connected");
        return;
    }

    spdlog::info("Connecting to NATS at {}", url_);

    natsOptions* opts = nullptr;
    natsStatus status;

    // Create options
    status = natsOptions_Create(&opts);
    if (status != NATS_OK) {
        spdlog::error("Failed to create NATS options: {}", natsStatus_GetText(status));
        return;
    }

    // Set server URL
    status = natsOptions_SetURL(opts, url_.c_str());
    if (status != NATS_OK) {
        spdlog::error("Failed to set NATS URL: {}", natsStatus_GetText(status));
        natsOptions_Destroy(opts);
        return;
    }

    // Set reconnect options
    natsOptions_SetReconnectWait(opts, 2000);  // 2 seconds
    natsOptions_SetMaxReconnect(opts, -1);  // Infinite reconnects

    // Set callbacks
    natsOptions_SetDisconnectedCB(opts, OnDisconnected, this);
    natsOptions_SetReconnectedCB(opts, OnReconnected, this);
    natsOptions_SetErrorHandler(opts, OnError, this);

    // Connect
    status = natsConnection_Connect(&conn_, opts);
    natsOptions_Destroy(opts);

    if (status != NATS_OK) {
        spdlog::error("Failed to connect to NATS: {}", natsStatus_GetText(status));
        return;
    }

    // Subscribe to all event subjects
    // Group events
    const std::vector<std::string> subjects = {
        "group.created",
        "group.member.added",
        "group.member.left",
        "group.disband",
        "group.loot.changed",
        "group.difficulty.changed",
        "group.converted.raid",
        // Guild events
        "guild.member.added",
        "guild.member.left",
        "guild.member.kicked",
        // Registry events
        "sr.gs.maps.reassigned"
    };

    for (const auto& subject : subjects) {
        SubscribeSubjectLocked(subject);
    }

    // Generic subscriptions requested before Start()
    {
        std::lock_guard<std::mutex> generic_lock(generic_mutex_);
        for (const auto& entry : generic_callbacks_) {
            if (subscribed_subjects_.find(entry.first) == subscribed_subjects_.end()) {
                SubscribeSubjectLocked(entry.first);
            }
        }
    }

    connected_ = true;
    spdlog::info("✅ NATS consumer connected and subscribed to {} subjects", subscriptions_.size());
}

bool NatsConsumer::SubscribeSubjectLocked(const std::string& subject) {
    natsSubscription* sub = nullptr;
    natsStatus status = natsConnection_Subscribe(&sub, conn_, subject.c_str(), OnMessage, this);
    if (status != NATS_OK) {
        spdlog::warn("Failed to subscribe to {}: {}", subject, natsStatus_GetText(status));
        return false;
    }
    subscriptions_.push_back(sub);
    subscribed_subjects_.insert(subject);
    spdlog::info("✅ Subscribed to: {}", subject);
    return true;
}

bool NatsConsumer::SubscribeGeneric(const std::string& subject, GenericMessageCallback callback) {
    if (subject.empty() || !callback) {
        return false;
    }

    {
        std::lock_guard<std::mutex> generic_lock(generic_mutex_);
        generic_callbacks_[subject].push_back(std::move(callback));
    }

    std::lock_guard<std::mutex> lock(mutex_);
    if (!connected_) {
        // Applied on Start() from the registry.
        return true;
    }
    if (subscribed_subjects_.find(subject) != subscribed_subjects_.end()) {
        return true;
    }
    return SubscribeSubjectLocked(subject);
}

void NatsConsumer::Stop() {
    std::lock_guard<std::mutex> lock(mutex_);

    if (!connected_) {
        return;
    }

    spdlog::info("Stopping NATS consumer");

    // Unsubscribe all
    for (auto* sub : subscriptions_) {
        if (sub) {
            natsSubscription_Unsubscribe(sub);
            natsSubscription_Destroy(sub);
        }
    }
    subscriptions_.clear();
    subscribed_subjects_.clear();

    // Close connection
    if (conn_) {
        natsConnection_Close(conn_);
        natsConnection_Destroy(conn_);
        conn_ = nullptr;
    }

    connected_ = false;
    spdlog::info("NATS consumer stopped");
}

// Static callback implementation
void NatsConsumer::OnMessage(natsConnection* /*nc*/, natsSubscription* /*sub*/,
                             natsMsg* msg, void* closure) {
    auto* consumer = static_cast<NatsConsumer*>(closure);

    const char* subject = natsMsg_GetSubject(msg);
    const char* data = natsMsg_GetData(msg);
    int data_len = natsMsg_GetDataLength(msg);

    if (!data || data_len <= 0 || !subject) {
        natsMsg_Destroy(msg);
        return;
    }

    std::string event_data(data, data_len);
    std::string subject_str(subject);

    spdlog::debug("NATS event received: {} ({} bytes)", subject_str, data_len);

    // Route to appropriate handler based on subject
    std::unique_ptr<Handler> handler;

    // Group events
    if (subject_str == "group.created") {
        handler = CreateGroupCreatedHandler(event_data, consumer->realm_id_);
    } else if (subject_str == "group.member.added") {
        handler = CreateGroupMemberAddedHandler(event_data, consumer->realm_id_);
    } else if (subject_str == "group.member.left") {
        handler = CreateGroupMemberRemovedHandler(event_data, consumer->realm_id_);
    } else if (subject_str == "group.disband") {
        handler = CreateGroupDisbandedHandler(event_data, consumer->realm_id_);
    } else if (subject_str == "group.loot.changed") {
        handler = CreateGroupLootTypeChangedHandler(event_data, consumer->realm_id_);
    } else if (subject_str == "group.difficulty.changed") {
        // Parse and dispatch both dungeon and raid difficulty
        handler = CreateGroupDungeonDifficultyChangedHandler(event_data, consumer->realm_id_);
        if (handler && consumer->event_queue_) {
            consumer->event_queue_->Push(std::move(handler));
        }
        handler = CreateGroupRaidDifficultyChangedHandler(event_data, consumer->realm_id_);
    } else if (subject_str == "group.converted.raid") {
        handler = CreateGroupConvertedToRaidHandler(event_data, consumer->realm_id_);
    }
    // Guild events
    else if (subject_str == "guild.member.added") {
        handler = CreateGuildMemberAddedHandler(event_data, consumer->realm_id_);
    } else if (subject_str == "guild.member.left") {
        handler = CreateGuildMemberLeftHandler(event_data, consumer->realm_id_);
    } else if (subject_str == "guild.member.kicked") {
        handler = CreateGuildMemberRemovedHandler(event_data, consumer->realm_id_);
    }
    // Registry events
    else if (subject_str == "sr.gs.maps.reassigned") {
        handler = CreateMapsReassignedHandler(event_data, consumer->GetServerID());
    } else {
        spdlog::debug("Unhandled NATS subject: {}", subject_str);
    }

    if (handler && consumer->event_queue_) {
        consumer->event_queue_->Push(std::move(handler));
    }

    // Generic subscriptions (TC9NatsSubscribe) — also fired for built-in
    // subjects, after their handler.
    std::vector<GenericMessageCallback> generic;
    {
        std::lock_guard<std::mutex> generic_lock(consumer->generic_mutex_);
        auto it = consumer->generic_callbacks_.find(subject_str);
        if (it != consumer->generic_callbacks_.end()) {
            generic = it->second;
        }
    }
    if (!generic.empty() && consumer->event_queue_) {
        for (auto& cb : generic) {
            consumer->event_queue_->Push(MakeHandler(
                [cb, subject_str, event_data]() { cb(subject_str, event_data); }));
        }
    }

    natsMsg_Destroy(msg);
}

void NatsConsumer::OnDisconnected(natsConnection* /*nc*/, void* /*closure*/) {
    spdlog::warn("NATS connection lost - will attempt to reconnect");
}

void NatsConsumer::OnReconnected(natsConnection* /*nc*/, void* /*closure*/) {
    spdlog::info("✅ NATS reconnected");
}

void NatsConsumer::OnError(natsConnection* /*nc*/, natsSubscription* /*sub*/,
                          natsStatus err, void* /*closure*/) {
    spdlog::error("NATS error: {}", natsStatus_GetText(err));
}

}  // namespace tc9
