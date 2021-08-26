package healthandmetrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

const activeConnectionMetricsName = "active_connections"

var ActiveConnectionsMetrics prometheus.Gauge

func EnableActiveConnectionsMetrics() {
	ActiveConnectionsMetrics = promauto.NewGauge(prometheus.GaugeOpts{
		Name: activeConnectionMetricsName,
		Help: "The number of active connections",
	})
}
