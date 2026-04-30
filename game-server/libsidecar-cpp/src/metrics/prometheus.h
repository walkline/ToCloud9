#ifndef TC9_PROMETHEUS_H
#define TC9_PROMETHEUS_H

#include <memory>
#include <string>
#include <prometheus/registry.h>
#include <prometheus/counter.h>
#include <prometheus/gauge.h>
#include <prometheus/histogram.h>

namespace tc9 {

class MetricsRegistry {
public:
    static MetricsRegistry& Instance();

    std::shared_ptr<prometheus::Registry> GetRegistry();

    // Serialize all metrics in Prometheus format
    std::string Serialize();

    // Pre-defined metrics
    void IncrementRequests(const std::string& method);
    void SetActiveConnections(int count);
    void ObserveRequestDelay(const std::string& method, double seconds);
    void SetQueueDepth(const std::string& queue_name, int depth);

    // Monitoring metrics (from AzerothCore)
    void SetDelayMean(uint32_t milliseconds);
    void SetDelayMedian(uint32_t milliseconds);
    void SetDelay95Percentile(uint32_t milliseconds);
    void SetDelay99Percentile(uint32_t milliseconds);
    void SetDelayMax(uint32_t milliseconds);

    // Delete copy/move
    MetricsRegistry(const MetricsRegistry&) = delete;
    MetricsRegistry& operator=(const MetricsRegistry&) = delete;
    MetricsRegistry(MetricsRegistry&&) = delete;
    MetricsRegistry& operator=(MetricsRegistry&&) = delete;

private:
    MetricsRegistry();
    ~MetricsRegistry() = default;

    void InitializeMetrics();

    std::shared_ptr<prometheus::Registry> registry_;

    // Metric families
    prometheus::Family<prometheus::Counter>* requests_total_ = nullptr;
    prometheus::Family<prometheus::Gauge>* active_connections_ = nullptr;
    prometheus::Family<prometheus::Histogram>* request_delay_seconds_ = nullptr;
    prometheus::Family<prometheus::Gauge>* queue_depth_ = nullptr;

    // Monitoring metric families (player latency)
    prometheus::Family<prometheus::Gauge>* delay_mean_ms_ = nullptr;
    prometheus::Family<prometheus::Gauge>* delay_median_ms_ = nullptr;
    prometheus::Family<prometheus::Gauge>* delay_95percentile_ms_ = nullptr;
    prometheus::Family<prometheus::Gauge>* delay_99percentile_ms_ = nullptr;
    prometheus::Family<prometheus::Gauge>* delay_max_ms_ = nullptr;
};

}  // namespace tc9

#endif  // TC9_PROMETHEUS_H
