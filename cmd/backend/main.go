package main

import (
	"context"
	"flag"
	"fmt"
	stdlog "log"
	"net/http"
	"os"
	"runtime"
	"time"

	"github.com/go-kit/kit/log/level"
	"github.com/oklog/run"
	"github.com/povilasv/prommod"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/version"
	"go.opentelemetry.io/otel/api/kv"
	"go.opentelemetry.io/otel/exporters/trace/jaeger"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"

	"github.com/kakkoyun/observable-remote-write/internal"
	internalhttp "github.com/kakkoyun/observable-remote-write/internal/http"
	"github.com/kakkoyun/observable-remote-write/internal/http/middleware"
	"github.com/kakkoyun/observable-remote-write/internal/receiver"
)

// gracePeriod specify graceful shutdown period.
const (
	gracePeriod = 10 * time.Second
	serviceName = "observable_remote_write_backend"
)

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
	healthcheckURL string
}

func main() {
	fmt.Println("Hello World from the Backend!")

	// Parse flags and initialize config struct.
	cfg := parseFlags()

	debug := os.Getenv("DEBUG") != ""
	if debug {
		runtime.SetMutexProfileFraction(cfg.debug.mutexProfileFraction)
		runtime.SetBlockProfileRate(cfg.debug.blockProfileRate)
	}

	// Start metric registry.
	reg := prometheus.NewRegistry()

	// Register standard Go metric collectors, which are by default registered when using global registry.
	reg.MustRegister(
		prometheus.NewGoCollector(),
		version.NewCollector(serviceName),
		prommod.NewCollector(serviceName),
		prometheus.NewProcessCollector(prometheus.ProcessCollectorOpts{}),
	)

	// Create and install Jaeger export pipeline.
	traceProvider, closer, err := jaeger.NewExportPipeline(
		// jaeger.WithAgentEndpoint("http://127.0.0.1:6831"), // OR: 6832
		jaeger.WithCollectorEndpoint("http://127.0.0.1:14268/api/traces"),
		jaeger.WithProcess(jaeger.Process{
			ServiceName: serviceName,
			Tags: []kv.KeyValue{
				kv.String("exporter", "jaeger"),
			},
		}),
		jaeger.WithSDK(&sdktrace.Config{DefaultSampler: sdktrace.AlwaysSample()}),
	)
	if err != nil {
		stdlog.Fatalf("failed to initialize tracer, err: %v", err)
	}

	defer closer()

	// Initialize OpenTelemetry tracer.
	tracer := traceProvider.Tracer(serviceName)

	// Initialize structured logger.
	logger := internal.NewLogger(cfg.logLevel, cfg.logFormat, cfg.debug.name)
	defer level.Info(logger).Log("msg", "exiting")

	// Initialize run group.
	g := &run.Group{}
	{
		metrics := middleware.NewMetricsMiddleware(reg)
		// Main server to listen for public APIs.
		mux := http.NewServeMux()
		mux.Handle("/receive",
			metrics.NewHandler("receive")(
				middleware.Tracer(logger, tracer, "receive-proxy")(
					middleware.RequestID(
						middleware.Logger(logger)(
							// othttp.NewHandler(
							receiver.Receive(logger, tracer),
						),
						// "receive-proxy", othttp.WithTracer(tracer),
					),
				),
			),
		)
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
				level.Error(logger).Log("msg", "server shutdown")
			}
		})
	}

	// Listen for termination signals.
	{
		cancel := make(chan struct{})
		g.Add(func() error {
			return internal.Interrupt(logger, cancel)
		}, func(error) {
			close(cancel)
		})
	}

	// Add internal server.
	{
		internalSrv := internalhttp.NewServer(reg, cfg.server.listenInternal, cfg.server.healthcheckURL)
		g.Add(func() error {
			level.Info(logger).Log("msg", "starting internal server")
			return internalSrv.ListenAndServe()
		}, func(error) {
			ctx, cancel := context.WithTimeout(context.Background(), gracePeriod)
			defer cancel()

			if err := internalSrv.Shutdown(ctx); err != nil {
				level.Error(logger).Log("msg", "internal server shutdown")
			}
		})
	}

	if err := g.Run(); err != nil {
		level.Error(logger).Log("msg", "run group", "err", err)
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
	flag.StringVar(&cfg.server.listen, "web.listen", ":8080",
		"The address on which the public server listens.")
	flag.StringVar(&cfg.server.listenInternal, "web.internal.listen", ":8081",
		"The address on which the internal server listens.")
	flag.StringVar(&cfg.server.healthcheckURL, "web.healthchecks.url", "http://127.0.0.1:8080",
		"The URL against which to run healthchecks.")
	flag.Parse()

	return cfg
}
