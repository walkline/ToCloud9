#include <httplib.h>
#include "health_server.h"
#include "../metrics/prometheus.h"
#include "../queue/handlers_queue.h"
#include <spdlog/spdlog.h>
#include <future>
#include <chrono>

namespace tc9 {

// Pimpl to hide httplib dependency from header
class HttpServerImpl {
public:
    httplib::Server server;
};

HealthServer::HealthServer(const std::string& port)
    : port_(port) {
    spdlog::debug("HealthServer created for port {}", port);
}

HealthServer::~HealthServer() {
    Stop();
}

void HealthServer::SetMonitoringDataCollector(TC9MonitoringDataCollectorHandler handler) {
    monitoring_handler_ = handler;
}

void HealthServer::SetReadQueue(HandlersQueue* queue) {
    read_queue_ = queue;
}

void HealthServer::Start() {
    if (running_) {
        spdlog::warn("Health server already running");
        return;
    }

    spdlog::info("Starting health check server on port {}", port_);

    impl_ = std::make_unique<HttpServerImpl>();

    // GET /health endpoint
    impl_->server.Get("/healthcheck", [](const httplib::Request& /*req*/, httplib::Response& res) {
        res.set_content(R"({"status":"OK"})", "application/json");
    });

        printf("!!!!!!\n");

    // GET /metrics endpoint (Prometheus format)
    impl_->server.Get("/metrics", [this](const httplib::Request& /*req*/, httplib::Response& res) {
        auto& registry = MetricsRegistry::Instance();

        // Call monitoring data collector if registered
        // IMPORTANT: Queue to read_queue so it runs on game loop thread (thread-safe)
        if (monitoring_handler_ && read_queue_) {
            auto promise = std::make_shared<std::promise<TC9MonitoringDataCollectorResponse>>();
            auto future = promise->get_future();

            // Queue handler to be executed on game loop thread
            read_queue_->Push(MakeHandler([promise, handler = monitoring_handler_]() {
                try {
                    TC9MonitoringDataCollectorResponse response = handler();
                    promise->set_value(response);
                } catch (const std::exception& e) {
                    spdlog::error("Monitoring handler threw: {}", e.what());
                    TC9MonitoringDataCollectorResponse error_response{};
                    error_response.errorCode = TC9_MONITORING_ERROR_NO_HANDLER;
                    promise->set_value(error_response);
                }
            }));

            // Wait for response with 5 second timeout (matches Go implementation)
            using namespace std::chrono_literals;
            if (future.wait_for(5s) == std::future_status::timeout) {
                spdlog::warn("Monitoring data collector timeout");
            } else {
                try {
                    TC9MonitoringDataCollectorResponse response = future.get();
                    if (response.errorCode == TC9_MONITORING_ERROR_NO_ERROR) {
                        // Update Prometheus metrics with fresh data
                        registry.SetActiveConnections(response.connectedPlayers);
                        registry.SetDelayMean(response.diffMean);
                        registry.SetDelayMedian(response.diffMedian);
                        registry.SetDelay95Percentile(response.diff95Percentile);
                        registry.SetDelay99Percentile(response.diff99Percentile);
                        registry.SetDelayMax(response.diffMaxPercentile);
                    } else {
                        spdlog::warn("Monitoring data collector returned error code: {}", response.errorCode);
                    }
                } catch (const std::exception& e) {
                    spdlog::error("Error getting monitoring data: {}", e.what());
                }
            }
        }

        std::string metrics = registry.Serialize();
        res.set_content(metrics, "text/plain; version=0.0.4");
    });

    // GET /ready endpoint
    impl_->server.Get("/ready", [](const httplib::Request& /*req*/, httplib::Response& res) {
        res.set_content(R"({"ready":true})", "application/json");
    });

    running_ = true;

    // Start server in background thread
    server_thread_ = std::thread([this]() {
        int port_num = std::stoi(port_);
        spdlog::info("✅ Health server listening on 0.0.0.0:{}", port_num);
        spdlog::info("   Endpoints: /healthcheck, /ready, /metrics");

        if (!impl_->server.listen("0.0.0.0", port_num)) {
            spdlog::error("Failed to start health server on port {}", port_);
            running_ = false;
        }
    });
}

void HealthServer::Stop() {
    if (!running_) {
        return;
    }

    spdlog::info("Stopping health check server");

    if (impl_) {
        impl_->server.stop();
    }

    if (server_thread_.joinable()) {
        server_thread_.join();
    }

    impl_.reset();
    running_ = false;
}

}  // namespace tc9
