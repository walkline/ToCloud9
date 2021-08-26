package healthandmetrics

import (
	"context"
	"net/http"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	HealthCheckURL = "/healthcheck"
	MetricsURL     = "/metrics"

	healthCheckOKPayload = []byte(`{"status":"OK"}`)
)

type Server interface {
	ListenAndServe() error
	Shutdown(ctx context.Context) error
	Port() string
}

type server struct {
	http.Server
	port string
}

func NewServer(port string, enableMetrics bool) Server {
	mux := http.NewServeMux()
	mux.HandleFunc(HealthCheckURL, func(w http.ResponseWriter, r *http.Request) {
		w.Write(healthCheckOKPayload)
	})

	if enableMetrics {
		mux.Handle(MetricsURL, promhttp.Handler())
	}

	return &server{
		port: port,
		Server: http.Server{
			Addr:    ":" + port,
			Handler: mux,
		},
	}
}

func (s server) Port() string {
	return s.port
}
