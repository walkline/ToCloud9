#include "prometheus.h"
#include <spdlog/spdlog.h>
#include <prometheus/serializer.h>
#include <prometheus/text_serializer.h>
#include <sstream>

namespace tc9 {

MetricsRegistry& MetricsRegistry::Instance() {
    static MetricsRegistry instance;
    return instance;
}

MetricsRegistry::MetricsRegistry() {
    registry_ = std::make_shared<prometheus::Registry>();
    InitializeMetrics();
    spdlog::debug("Prometheus metrics registry initialized");
}

void MetricsRegistry::InitializeMetrics() {
    // Counter: total number of requests by method
    requests_total_ = &prometheus::BuildCounter()
        .Name("libsidecar_requests_total")
        .Help("Total number of requests processed")
        .Register(*registry_);

    // Gauge: current active connections
    active_connections_ = &prometheus::BuildGauge()
        .Name("active_connections")
        .Help("The number of active connections")
        .Register(*registry_);

    // Histogram: request processing delay in seconds
    request_delay_seconds_ = &prometheus::BuildHistogram()
        .Name("libsidecar_request_delay_seconds")
        .Help("Request processing delay in seconds")
        .Register(*registry_);

    // Gauge: queue depth
    queue_depth_ = &prometheus::BuildGauge()
        .Name("libsidecar_queue_depth")
        .Help("Number of items in queue")
        .Register(*registry_);

    // Player latency metrics (from AzerothCore monitoring)
    // These match the Go library metric names exactly
    delay_mean_ms_ = &prometheus::BuildGauge()
        .Name("delay_mean")
        .Help("The mean delay in ms")
        .Register(*registry_);

    delay_median_ms_ = &prometheus::BuildGauge()
        .Name("delay_median")
        .Help("The median delay in ms")
        .Register(*registry_);

    delay_95percentile_ms_ = &prometheus::BuildGauge()
        .Name("delay_95_percentile")
        .Help("The 95 percentile delay in ms")
        .Register(*registry_);

    delay_99percentile_ms_ = &prometheus::BuildGauge()
        .Name("delay_99_percentile")
        .Help("The 99 percentile delay in ms")
        .Register(*registry_);

    delay_max_ms_ = &prometheus::BuildGauge()
        .Name("delay_max")
        .Help("The max delay in ms")
        .Register(*registry_);
}

std::shared_ptr<prometheus::Registry> MetricsRegistry::GetRegistry() {
    return registry_;
}

std::string MetricsRegistry::Serialize() {
    prometheus::TextSerializer serializer;
    return serializer.Serialize(registry_->Collect());
}

void MetricsRegistry::IncrementRequests(const std::string& method) {
    if (requests_total_) {
        requests_total_->Add({{"method", method}}).Increment();
    }
}

void MetricsRegistry::SetActiveConnections(int count) {
    if (active_connections_) {
        active_connections_->Add({}).Set(count);
    }
}

void MetricsRegistry::ObserveRequestDelay(const std::string& method, double seconds) {
    if (request_delay_seconds_) {
        request_delay_seconds_->Add({{"method", method}},
            prometheus::Histogram::BucketBoundaries{0.001, 0.01, 0.1, 0.5, 1.0, 5.0})
            .Observe(seconds);
    }
}

void MetricsRegistry::SetQueueDepth(const std::string& queue_name, int depth) {
    if (queue_depth_) {
        queue_depth_->Add({{"queue", queue_name}}).Set(depth);
    }
}

void MetricsRegistry::SetDelayMean(uint32_t milliseconds) {
    if (delay_mean_ms_) {
        delay_mean_ms_->Add({}).Set(milliseconds);
    }
}

void MetricsRegistry::SetDelayMedian(uint32_t milliseconds) {
    if (delay_median_ms_) {
        delay_median_ms_->Add({}).Set(milliseconds);
    }
}

void MetricsRegistry::SetDelay95Percentile(uint32_t milliseconds) {
    if (delay_95percentile_ms_) {
        delay_95percentile_ms_->Add({}).Set(milliseconds);
    }
}

void MetricsRegistry::SetDelay99Percentile(uint32_t milliseconds) {
    if (delay_99percentile_ms_) {
        delay_99percentile_ms_->Add({}).Set(milliseconds);
    }
}

void MetricsRegistry::SetDelayMax(uint32_t milliseconds) {
    if (delay_max_ms_) {
        delay_max_ms_->Add({}).Set(milliseconds);
    }
}

}  // namespace tc9
