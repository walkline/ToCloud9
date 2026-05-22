package main

/*
#include "monitoring.h"
*/
import "C"

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/walkline/ToCloud9/game-server/libsidecar/queue"
	"github.com/walkline/ToCloud9/shared/healthandmetrics"
)

const worldLoopHealthCheckTimeout = time.Second * 5

type monitoringResponse struct {
	diffMean          int
	diffMedian        int
	diff95Percentile  int
	diff99Percentile  int
	diffMax           int
	activeConnections int
}

// TC9SetMonitoringDataCollectorHandler sets handler for getting data to handle monitoring request.
//
//export TC9SetMonitoringDataCollectorHandler
func TC9SetMonitoringDataCollectorHandler(h C.MonitoringDataCollectorHandler) {
	C.SetMonitoringDataCollectorHandler(h)
}

func worldLoopHealthProbe(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, worldLoopHealthCheckTimeout)
	defer cancel()

	done := make(chan struct{}, 1)
	readRequestsQueue.Push(queue.HandlerFunc(func() {
		done <- struct{}{}
	}))

	select {
	case <-ctx.Done():
		return fmt.Errorf("world loop did not process health probe: %w", ctx.Err())
	case <-done:
		return nil
	}
}

func collectMonitoringData(ctx context.Context) (monitoringResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, worldLoopHealthCheckTimeout)
	defer cancel()

	respChan := make(chan struct {
		resp monitoringResponse
		err  error
	}, 1)

	readRequestsQueue.Push(queue.HandlerFunc(func() {
		res := C.CallMonitoringDataCollectorHandler()
		if res.errorCode != C.MonitoringErrorCodeNoError {
			respChan <- struct {
				resp monitoringResponse
				err  error
			}{
				err: errors.New("failed to process monitoring data collection"),
			}
			return
		}

		respChan <- struct {
			resp monitoringResponse
			err  error
		}{
			resp: monitoringResponse{
				diffMean:          int(res.diffMean),
				diffMedian:        int(res.diffMedian),
				diff95Percentile:  int(res.diff95Percentile),
				diff99Percentile:  int(res.diff99Percentile),
				diffMax:           int(res.diffMaxPercentile),
				activeConnections: int(res.connectedPlayers),
			},
		}
	}))

	select {
	case <-ctx.Done():
		return monitoringResponse{}, ctx.Err()
	case res := <-respChan:
		return res.resp, res.err
	}
}

func monitoringHttpHandler() http.Handler {
	promHandler := promhttp.Handler()
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp, err := collectMonitoringData(r.Context())
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
			w.WriteHeader(http.StatusGatewayTimeout)
			_, _ = w.Write([]byte("timeout"))
			return
		}

		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(err.Error()))
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
