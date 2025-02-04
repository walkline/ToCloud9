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

const delayMeanMetricsName = "delay_mean"

const delayMedianMetricsName = "delay_median"

const delay95PercentileMetricsName = "delay_95_percentile"

const delay99PercentileMetricsName = "delay_99_percentile"

const delayMaxMetricsName = "delay_max"

var DelayMeanMetrics prometheus.Gauge

var DelayMedianMetrics prometheus.Gauge

var Delay95PercentileMetrics prometheus.Gauge

var Delay99PercentileMetrics prometheus.Gauge

var DelayMaxMetrics prometheus.Gauge

func EnableDelayMetrics() {
	DelayMeanMetrics = promauto.NewGauge(prometheus.GaugeOpts{
		Name: delayMeanMetricsName,
		Help: "The mean delay in ms",
	})
	DelayMedianMetrics = promauto.NewGauge(prometheus.GaugeOpts{
		Name: delayMedianMetricsName,
		Help: "The median delay in ms",
	})
	Delay95PercentileMetrics = promauto.NewGauge(prometheus.GaugeOpts{
		Name: delay95PercentileMetricsName,
		Help: "The 95 percentile delay in ms",
	})
	Delay99PercentileMetrics = promauto.NewGauge(prometheus.GaugeOpts{
		Name: delay99PercentileMetricsName,
		Help: "The 99 percentile delay in ms",
	})
	DelayMaxMetrics = promauto.NewGauge(prometheus.GaugeOpts{
		Name: delayMaxMetricsName,
		Help: "The max delay in ms",
	})
}
