package healthandmetrics

import (
	"context"
	"net/http"
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

func NewServer(port string, metricsHandler http.Handler) Server {
	mux := http.NewServeMux()
	mux.HandleFunc(HealthCheckURL, func(w http.ResponseWriter, r *http.Request) {
		w.Write(healthCheckOKPayload)
	})

	if metricsHandler != nil {
		mux.Handle(MetricsURL, metricsHandler)
	}

	return &server{
		port: port,
		Server: http.Server{
			Addr:    ":" + port,
			Handler: mux,
		},
	}
}

func (s *server) Port() string {
	return s.port
}
