package main

/*
#include "monitoring.h"
*/
import "C"

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/walkline/ToCloud9/game-server/libsidecar/queue"
	"github.com/walkline/ToCloud9/shared/healthandmetrics"
)

// TC9SetMonitoringDataCollectorHandler sets handler for getting data to handle monitoring request.
//
//export TC9SetMonitoringDataCollectorHandler
func TC9SetMonitoringDataCollectorHandler(h C.MonitoringDataCollectorHandler) {
	C.SetMonitoringDataCollectorHandler(h)
}

func monitoringHttpHandler() http.Handler {
	promHandler := promhttp.Handler()
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), time.Second*5)
		defer cancel()

		type respType struct {
			diffMean          int
			diffMedian        int
			diff95Percentile  int
			diff99Percentile  int
			diffMax           int
			activeConnections int

			err error
		}
		var resp respType

		respChan := make(chan respType, 1)

		readRequestsQueue.Push(queue.HandlerFunc(func() {
			res := C.CallMonitoringDataCollectorHandler()
			if res.errorCode != C.MonitoringErrorCodeNoError {
				respChan <- respType{
					err: errors.New("failed to process monitoring data collection"),
				}
				close(respChan)
				return
			}

			respChan <- respType{
				diffMean:          int(res.diffMean),
				diffMedian:        int(res.diffMedian),
				diff95Percentile:  int(res.diff95Percentile),
				diff99Percentile:  int(res.diff99Percentile),
				diffMax:           int(res.diffMaxPercentile),
				activeConnections: int(res.connectedPlayers),
				err:               nil,
			}
			close(respChan)
		}))

		select {
		case <-ctx.Done():
			w.WriteHeader(http.StatusGatewayTimeout)
			_, _ = w.Write([]byte("timeout"))
			return
		case resp = <-respChan:
		}

		if resp.err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(resp.err.Error()))
			return
		}

		healthandmetrics.ActiveConnectionsMetrics.Set(float64(resp.activeConnections))
		healthandmetrics.DelayMeanMetrics.Set(float64(resp.diffMean))
		healthandmetrics.DelayMedianMetrics.Set(float64(resp.diffMedian))
		healthandmetrics.Delay95PercentileMetrics.Set(float64(resp.diff95Percentile))
		healthandmetrics.Delay99PercentileMetrics.Set(float64(resp.diff99Percentile))
		healthandmetrics.DelayMaxMetrics.Set(float64(resp.diffMax))

		promHandler.ServeHTTP(w, r)
	})
}
