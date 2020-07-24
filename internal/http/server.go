package http

import (
	"net/http"
	"net/http/pprof"
	"time"

	"github.com/metalmatze/signal/healthcheck"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// NewServer creates a new internal server that exposes debug probes.
func NewServer(reg prometheus.Gatherer, listen, healthcheckURL string) *http.Server {
	// Internal server to expose introspection APIs.
	mux := http.NewServeMux()

	// Initialize health checks.
	healthchecks := healthcheck.NewHandler()

	// Register health check endpoints.
	mux.Handle("/-/healthy", http.HandlerFunc(healthchecks.LiveEndpoint))
	mux.Handle("/-/ready", http.HandlerFunc(healthchecks.ReadyEndpoint))

	// Register pprof endpoints.
	mux.HandleFunc("/debug/pprof/", pprof.Index)
	mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	mux.HandleFunc("/debug/pprof/trace", pprof.Trace)

	// Register metrics server.
	mux.Handle("/metrics", promhttp.HandlerFor(reg, promhttp.HandlerOpts{}))

	// Checks if public server is up
	healthchecks.AddLivenessCheck("http",
		healthcheck.HTTPCheckClient(
			&http.Client{},
			healthcheckURL,
			http.MethodGet,
			http.StatusNotFound,
			time.Second,
		),
	)

	return &http.Server{
		Addr:    listen,
		Handler: mux,
	}
}
