#ifndef TC9_NATS_CONSUMER_H
#define TC9_NATS_CONSUMER_H

#include <string>
#include <functional>
#include <memory>
#include <mutex>
#include <unordered_map>
#include <unordered_set>
#include <vector>
#include <nats.h>

namespace tc9 {

class HandlersQueue;

// Callback for generic subscriptions (TC9NatsSubscribe). Executed on the
// thread draining the event queue (TC9ProcessEventsHooks), not on the NATS
// delivery thread.
using GenericMessageCallback =
    std::function<void(const std::string& subject, const std::string& payload)>;

class NatsConsumer {
public:
    explicit NatsConsumer(const std::string& url);
    ~NatsConsumer();

    void Start();
    void Stop();

    // Subscribe to an arbitrary subject. Safe to call before Start() (the
    // NATS subscription is then created on connect). Multiple callbacks per
    // subject are allowed; built-in subjects can also be subscribed to.
    bool SubscribeGeneric(const std::string& subject, GenericMessageCallback callback);

    // Set the event queue where events will be pushed
    void SetEventQueue(HandlersQueue* queue) { event_queue_ = queue; }

    // Set the realm ID for filtering events
    void SetRealmID(uint32_t realm_id) { realm_id_ = realm_id; }

    // Registry-assigned game server ID: written once registration completes
    // (after Start), read from the NATS delivery thread.
    void SetServerID(const std::string& server_id) {
        std::lock_guard<std::mutex> lock(server_id_mutex_);
        server_id_ = server_id;
    }
    std::string GetServerID() {
        std::lock_guard<std::mutex> lock(server_id_mutex_);
        return server_id_;
    }

    // Delete copy/move
    NatsConsumer(const NatsConsumer&) = delete;
    NatsConsumer& operator=(const NatsConsumer&) = delete;
    NatsConsumer(NatsConsumer&&) = delete;
    NatsConsumer& operator=(NatsConsumer&&) = delete;

private:
    std::string url_;
    bool connected_ = false;
    std::mutex mutex_;

    // NATS connection and subscriptions
    natsConnection* conn_ = nullptr;
    std::vector<natsSubscription*> subscriptions_;

    HandlersQueue* event_queue_ = nullptr;
    uint32_t realm_id_ = 0;
    std::string server_id_;
    std::mutex server_id_mutex_;

    // Generic subscriptions (own mutex: read from the NATS delivery thread
    // in OnMessage while mutex_ may be held by Start/Stop).
    std::mutex generic_mutex_;
    std::unordered_map<std::string, std::vector<GenericMessageCallback>> generic_callbacks_;
    // Subjects with an active NATS subscription (built-in + generic), to
    // avoid double subscriptions that would deliver messages twice.
    std::unordered_set<std::string> subscribed_subjects_;

    // Creates the NATS subscription for a subject. mutex_ must be held.
    bool SubscribeSubjectLocked(const std::string& subject);

    // Static callbacks for NATS
    static void OnMessage(natsConnection* nc, natsSubscription* sub,
                         natsMsg* msg, void* closure);

    // Connection status callbacks
    static void OnDisconnected(natsConnection* nc, void* closure);
    static void OnReconnected(natsConnection* nc, void* closure);
    static void OnError(natsConnection* nc, natsSubscription* sub,
                       natsStatus err, void* closure);
};

}  // namespace tc9

#endif  // TC9_NATS_CONSUMER_H
