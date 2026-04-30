#ifndef TC9_HEALTH_SERVER_H
#define TC9_HEALTH_SERVER_H

#include <string>
#include <memory>
#include <atomic>
#include <thread>
#include "libsidecar/tc9_types.h"

namespace tc9 {

// Forward declaration to avoid including httplib in header
class HttpServerImpl;
class HandlersQueue;
struct CppBindings;

class HealthServer {
public:
    explicit HealthServer(const std::string& port);
    ~HealthServer();

    // Set monitoring data collector for /metrics endpoint
    void SetMonitoringDataCollector(TC9MonitoringDataCollectorHandler handler);

    // Set read queue for queuing monitoring handler
    void SetReadQueue(HandlersQueue* queue);

    void Start();
    void Stop();

    // Delete copy/move
    HealthServer(const HealthServer&) = delete;
    HealthServer& operator=(const HealthServer&) = delete;
    HealthServer(HealthServer&&) = delete;
    HealthServer& operator=(HealthServer&&) = delete;

private:
    std::string port_;
    std::unique_ptr<HttpServerImpl> impl_;
    std::atomic<bool> running_{false};
    std::thread server_thread_;
    TC9MonitoringDataCollectorHandler monitoring_handler_ = nullptr;
    HandlersQueue* read_queue_ = nullptr;
};

}  // namespace tc9

#endif  // TC9_HEALTH_SERVER_H
