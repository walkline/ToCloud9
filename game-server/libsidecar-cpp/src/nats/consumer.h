#ifndef TC9_NATS_CONSUMER_H
#define TC9_NATS_CONSUMER_H

#include <string>
#include <memory>
#include <mutex>
#include <vector>
#include <nats.h>

namespace tc9 {

class HandlersQueue;

class NatsConsumer {
public:
    explicit NatsConsumer(const std::string& url);
    ~NatsConsumer();

    void Start();
    void Stop();

    // Set the event queue where events will be pushed
    void SetEventQueue(HandlersQueue* queue) { event_queue_ = queue; }

    // Set the realm ID for filtering events
    void SetRealmID(uint32_t realm_id) { realm_id_ = realm_id; }

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
