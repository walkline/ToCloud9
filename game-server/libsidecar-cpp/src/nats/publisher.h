#ifndef TC9_NATS_PUBLISHER_H
#define TC9_NATS_PUBLISHER_H

#include <string>
#include <mutex>
#include <nats.h>

namespace tc9 {

// Publishes worldserver-originated messages to NATS. Payloads are opaque
// bytes, so callers can reuse the JSON envelopes consumed by the Go
// services or define their own subjects.
class NatsPublisher {
public:
    explicit NatsPublisher(const std::string& url);
    ~NatsPublisher();

    void Start();
    void Stop();

    // Thread-safe: serialized with Start/Stop so the connection cannot
    // be destroyed mid-publish.
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
