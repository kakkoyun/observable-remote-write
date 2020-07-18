package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/go-kit/kit/log/level"
	"github.com/oklog/run"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/common/version"

	"github.com/kakkoyun/observable-remote-write/internal"
	"github.com/kakkoyun/observable-remote-write/pkg/exthttp"
)

// gracePeriod specify graceful shutdown period.
const gracePeriod = 10 * time.Second

type config struct {
	logLevel  string
	logFormat string

	debug  debugConfig
	server serverConfig
}

type debugConfig struct {
	mutexProfileFraction int
	blockProfileRate     int
	name                 string
}

type serverConfig struct {
	listen         string
	listenInternal string
	healthCheckURL string
}

func main() {
	fmt.Println("Hello World from the Proxy!")

	// Parse flags and initialize config struct.
	cfg := parseFlags()

	// Initialize structured logger.
	logger := internal.NewLogger(cfg.logLevel, cfg.logFormat, cfg.debug.name)
	defer level.Info(logger).Log("msg", "exiting")

	// Start our metric registry.
	reg := prometheus.NewRegistry()

	// Register standard Go metric collectors, which are by default registered when using global registry.
	reg.MustRegister(
		version.NewCollector("observable_remote_write_proxy"),
		prometheus.NewGoCollector(),
		prometheus.NewProcessCollector(prometheus.ProcessCollectorOpts{}),
	)

	// Initialize run group.
	g := &run.Group{}
	{
		// Server listen for proxy.
		mux := http.NewServeMux()
		mux.Handle("/metrics", exthttp.NewMetricsMiddleware(reg).NewHandler(
			"/metrics", promhttp.HandlerFor(reg, promhttp.HandlerOpts{}),
		))

		srv := &http.Server{
			Addr:    cfg.server.listen,
			Handler: mux,
		}

		g.Add(func() error {
			level.Info(logger).Log("msg", "starting server")
			return srv.ListenAndServe()
		}, func(error) {
			ctx, cancel := context.WithTimeout(context.Background(), gracePeriod)
			defer cancel()

			if err := srv.Shutdown(ctx); err != nil {
				level.Error(logger).Log("msg", "server shutdown failed")
			}
		})
	}

	if err := g.Run(); err != nil {
		level.Error(logger).Log("msg", "run group failed", "err", err)
		os.Exit(1)
	}
}

// Helpers

func parseFlags() config {
	cfg := config{}

	flag.StringVar(&cfg.debug.name, "debug.name", "observable-remote-write-backend",
		"A name to add as a prefix to log lines.")
	flag.IntVar(&cfg.debug.mutexProfileFraction, "debug.mutex-profile-fraction", 10,
		"The percentage of mutex contention events that are reported in the mutex profile.")
	flag.IntVar(&cfg.debug.blockProfileRate, "debug.block-profile-rate", 10,
		"The percentage of goroutine blocking events that are reported in the blocking profile.")
	flag.StringVar(&cfg.logLevel, "log.level", "info",
		"The log filtering level. Options: 'error', 'warn', 'info', 'debug'.")
	flag.StringVar(&cfg.logFormat, "log.format", internal.LogFormatLogfmt,
		"The log format to use. Options: 'logfmt', 'json'.")
	flag.StringVar(&cfg.server.listen, "web.listen", ":8090",
		"The address on which the public server listens.")
	flag.StringVar(&cfg.server.listenInternal, "web.internal.listen", ":8091",
		"The address on which the internal server listens.")
	flag.StringVar(&cfg.server.healthCheckURL, "web.health-check.url", "http://localhost:8090",
		"The URL against which to run health checks.")
	flag.Parse()

	return cfg
}
