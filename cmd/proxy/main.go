package main

import (
	"context"
	"flag"
	"fmt"
	stdlog "log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/observatorium/observable-demo/pkg/conntrack"
	"github.com/observatorium/observable-demo/pkg/lbtransport"
	"github.com/oklog/run"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/version"
	"go.opentelemetry.io/otel/api/kv"
	"go.opentelemetry.io/otel/exporters/trace/jaeger"
	"go.opentelemetry.io/otel/instrumentation/othttp"
	"go.opentelemetry.io/otel/sdk/trace"

	"github.com/kakkoyun/observable-remote-write/internal"
	internalhttp "github.com/kakkoyun/observable-remote-write/internal/http"
	"github.com/kakkoyun/observable-remote-write/internal/http/middleware"
)

const (
	// gracePeriod specify graceful shutdown period.
	gracePeriod = 10 * time.Second

	// backoffDuration specify back-off duration of loadbalancer when backing connection fails.
	backoffDuration = 5 * time.Second

	serviceName = "observable_remote_write_proxy"
)

var errCancelled = errors.New("canceled")

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

	targets        []url.URL
	healthcheckURL string
}

func main() {
	fmt.Println("Hello World from the Proxy!")

	// Parse flags and initialize config struct.
	cfg := parseFlags()

	debug := os.Getenv("DEBUG") != ""
	if debug {
		runtime.SetMutexProfileFraction(cfg.debug.mutexProfileFraction)
		runtime.SetBlockProfileRate(cfg.debug.blockProfileRate)
	}

	// Start our metric registry.
	reg := prometheus.NewRegistry()

	// Register standard Go metric collectors, which are by default registered when using global registry.
	reg.MustRegister(
		version.NewCollector(serviceName),
		prometheus.NewGoCollector(),
		prometheus.NewProcessCollector(prometheus.ProcessCollectorOpts{}),
	)

	// Create and install Jaeger export pipeline.
	traceProvider, closer, err := jaeger.NewExportPipeline(
		// jaeger.WithAgentEndpoint("http://127.0.0.1:6831"), // TODO: 6832, configurable?
		jaeger.WithCollectorEndpoint("http://127.0.0.1:14268/api/traces"), // TODO: default port?
		jaeger.WithProcess(jaeger.Process{
			ServiceName: serviceName,
			Tags: []kv.KeyValue{
				kv.String("exporter", "jaeger"),
			},
		}),
		jaeger.WithSDK(&trace.Config{DefaultSampler: trace.AlwaysSample()}),
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
		// Main server to listen for public APIs.
		mux := http.NewServeMux()
		srv := &http.Server{
			Handler: mux,
		}

		ctx, pCancel := context.WithCancel(context.Background())
		static := lbtransport.NewStaticDiscovery(cfg.server.targets, reg)
		picker := lbtransport.NewRoundRobinPicker(ctx, reg, backoffDuration)
		l7LoadBalancer := &httputil.ReverseProxy{
			Director:       func(request *http.Request) {},
			ModifyResponse: func(response *http.Response) error { return nil },
			Transport: othttp.NewTransport(
				lbtransport.NewLoadBalancingTransport(static, picker, lbtransport.NewMetrics(reg)),
				othttp.WithTracer(tracer),
			),
		}

		metrics := middleware.NewMetricsMiddleware(reg)
		mux.Handle("/receive",
			metrics.NewHandler("receive-proxy")(
				middleware.Tracer(logger, tracer, "receive-proxy")(
					middleware.RequestID(
						middleware.Logger(logger)(
							// othttp.NewHandler(
							l7LoadBalancer,
							// "receive-proxy", othttp.WithTracer(tracer),
						),
					),
				),
			),
		)

		g.Add(func() error {
			level.Info(logger).Log("msg", "starting server")

			listener, err := net.Listen("tcp", cfg.server.listen)
			if err != nil {
				return errors.Wrap(err, "new listener")
			}

			return srv.Serve(
				conntrack.NewInstrumentedListener(listener,
					conntrack.NewListenerMetrics(prometheus.WrapRegistererWith(prometheus.Labels{"listener": "lb"}, reg)),
				),
			)
		}, func(err error) {
			defer pCancel()

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
			return interrupt(logger, cancel)
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
	var (
		cfg        = config{}
		rawTargets string
	)

	flag.StringVar(&cfg.debug.name, "debug.name", "observable-remote-write-proxy",
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
	flag.StringVar(&rawTargets, "web.targets", "",
		"Comma-separated URLs for target to load balance to.")
	flag.StringVar(&cfg.server.listenInternal, "web.internal.listen", ":8091",
		"The address on which the internal server listens.")
	flag.StringVar(&cfg.server.healthcheckURL, "web.healthchecks.url", "http://localhost:8090",
		"The URL against which to run healthchecks.")
	flag.Parse()

	for _, addr := range strings.Split(rawTargets, ",") {
		u, err := url.Parse(addr)
		if err != nil {
			stdlog.Fatalf("failed to parse target %v; err: %v", addr, err)
		}

		cfg.server.targets = append(cfg.server.targets, *u)
	}

	return cfg
}

func interrupt(logger log.Logger, cancel <-chan struct{}) error {
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)
	select {
	case s := <-c:
		level.Info(logger).Log("msg", "caught signal, shutting down", "signal", s)
		return nil
	case <-cancel:
		return errCancelled
	}
}
