#include "publisher.h"
#include <spdlog/spdlog.h>

namespace tc9 {

NatsPublisher::NatsPublisher(const std::string& url) : url_(url) {
    spdlog::debug("NATS publisher created for URL: {}", url);
}

NatsPublisher::~NatsPublisher() {
    Stop();
}

void NatsPublisher::Start() {
    std::lock_guard<std::mutex> lock(mutex_);

    if (connected_) {
        spdlog::warn("NATS publisher already connected");
        return;
    }

    natsOptions* opts = nullptr;
    natsStatus status;

    status = natsOptions_Create(&opts);
    if (status != NATS_OK) {
        spdlog::error("Failed to create NATS options: {}", natsStatus_GetText(status));
        return;
    }

    status = natsOptions_SetURL(opts, url_.c_str());
    if (status != NATS_OK) {
        spdlog::error("Failed to set NATS URL: {}", natsStatus_GetText(status));
        natsOptions_Destroy(opts);
        return;
    }

    natsOptions_SetReconnectWait(opts, 2000);  // 2 seconds
    natsOptions_SetMaxReconnect(opts, -1);     // Infinite reconnects

    natsOptions_SetDisconnectedCB(opts, OnDisconnected, this);
    natsOptions_SetReconnectedCB(opts, OnReconnected, this);

    status = natsConnection_Connect(&conn_, opts);
    natsOptions_Destroy(opts);

    if (status != NATS_OK) {
        spdlog::error("Failed to connect NATS publisher: {}", natsStatus_GetText(status));
        return;
    }

    connected_ = true;
    spdlog::info("NATS publisher connected to {}", url_);
}

void NatsPublisher::Stop() {
    std::lock_guard<std::mutex> lock(mutex_);

    if (conn_) {
        // Flush pending messages before closing so logged-out events
        // emitted during shutdown are not lost.
        natsConnection_FlushTimeout(conn_, 2000);
        natsConnection_Destroy(conn_);
        conn_ = nullptr;
    }
    connected_ = false;
}

bool NatsPublisher::Publish(const std::string& subject, const std::string& payload) {
    if (!connected_ || !conn_) {
        spdlog::warn("NATS publisher not connected, dropping event on {}", subject);
        return false;
    }

    natsStatus status = natsConnection_Publish(
        conn_, subject.c_str(), payload.data(), static_cast<int>(payload.size()));

    if (status != NATS_OK) {
        spdlog::error("Failed to publish on {}: {}", subject, natsStatus_GetText(status));
        return false;
    }

    return true;
}

void NatsPublisher::OnDisconnected(natsConnection* /*nc*/, void* /*closure*/) {
    spdlog::warn("NATS publisher disconnected, will reconnect...");
}

void NatsPublisher::OnReconnected(natsConnection* /*nc*/, void* /*closure*/) {
    spdlog::info("NATS publisher reconnected");
}

}  // namespace tc9
