#ifndef TC9_NATS_PUBLISHER_H
#define TC9_NATS_PUBLISHER_H

#include <string>
#include <mutex>
#include <nats.h>

namespace tc9 {

// Publishes worldserver-originated events to NATS using the same JSON
// envelopes as the gateway, so existing consumers (charserver, chatserver,
// friends cache) receive them transparently. First use case: online status
// for in-process sessions (e.g. server-side bots) that never cross a
// gateway and therefore never trigger gateway login/logout events.
class NatsPublisher {
public:
    explicit NatsPublisher(const std::string& url);
    ~NatsPublisher();

    void Start();
    void Stop();

    // Thread-safe once Start() succeeded (cnats connections are
    // thread-safe for publishing).
    bool Publish(const std::string& subject, const std::string& payload);

    // Delete copy/move
    NatsPublisher(const NatsPublisher&) = delete;
    NatsPublisher& operator=(const NatsPublisher&) = delete;
    NatsPublisher(NatsPublisher&&) = delete;
    NatsPublisher& operator=(NatsPublisher&&) = delete;

private:
    std::string url_;
    bool connected_ = false;
    std::mutex mutex_;

    natsConnection* conn_ = nullptr;

    static void OnDisconnected(natsConnection* nc, void* closure);
    static void OnReconnected(natsConnection* nc, void* closure);
};

}  // namespace tc9

#endif  // TC9_NATS_PUBLISHER_H
