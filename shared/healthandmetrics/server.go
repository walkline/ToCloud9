package healthandmetrics

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

var (
	HealthCheckURL = "/healthcheck"
	MetricsURL     = "/metrics"

	healthCheckOKPayload = "OK"
)

type Server interface {
	ListenAndServe() error
	Shutdown(ctx context.Context) error
	Port() string
	StartedAtUnixMs() int64
}

type HealthProbe func(context.Context) error

type server struct {
	http.Server
	port            string
	startedAtUnixMs int64
}

type healthCheckPayload struct {
	Status          string `json:"status"`
	StartedAtUnixMs int64  `json:"startedAtUnixMs"`
}

func NewServer(port string, metricsHandler http.Handler) Server {
	return NewServerWithHealthProbe(port, metricsHandler, nil)
}

func NewServerWithHealthProbe(port string, metricsHandler http.Handler, healthProbe HealthProbe) Server {
	startedAtUnixMs := time.Now().UnixMilli()
	payload, _ := json.Marshal(healthCheckPayload{
		Status:          healthCheckOKPayload,
		StartedAtUnixMs: startedAtUnixMs,
	})

	mux := http.NewServeMux()
	mux.HandleFunc(HealthCheckURL, healthCheckHandler(payload, healthProbe))

	if metricsHandler != nil {
		mux.Handle(MetricsURL, metricsHandler)
	}

	return &server{
		port:            port,
		startedAtUnixMs: startedAtUnixMs,
		Server: http.Server{
			Addr:    ":" + port,
			Handler: mux,
		},
	}
}

func healthCheckHandler(okPayload []byte, healthProbe HealthProbe) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if healthProbe != nil {
			if err := healthProbe(r.Context()); err != nil {
				w.WriteHeader(http.StatusServiceUnavailable)
				_, _ = w.Write([]byte(fmt.Sprintf(`{"status":"UNHEALTHY","error":%q}`, err.Error())))
				return
			}
		}

		w.Write(okPayload)
	}
}

func (s *server) Port() string {
	return s.port
}

func (s *server) StartedAtUnixMs() int64 {
	return s.startedAtUnixMs
}
