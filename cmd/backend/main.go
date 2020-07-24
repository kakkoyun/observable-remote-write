package main

import (
	"context"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/pprof"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/gogo/protobuf/proto"
	"github.com/golang/snappy"
	"github.com/oklog/run"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/common/model"
	"github.com/prometheus/common/version"
	"github.com/prometheus/prometheus/prompb"

	"github.com/kakkoyun/observable-remote-write/internal"
	internalhttp "github.com/kakkoyun/observable-remote-write/internal/http"
)

// gracePeriod specify graceful shutdown period.
const gracePeriod = 10 * time.Second

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

	// Initialize structured logger.
	logger := internal.NewLogger(cfg.logLevel, cfg.logFormat, cfg.debug.name)
	defer level.Info(logger).Log("msg", "exiting")

	// Start metric registry.
	reg := prometheus.NewRegistry()

	// Register standard Go metric collectors, which are by default registered when using global registry.
	reg.MustRegister(
		version.NewCollector("observable_remote_write_backend"),
		prometheus.NewGoCollector(),
		prometheus.NewProcessCollector(prometheus.ProcessCollectorOpts{}),
	)

	// Initialize run group.
	g := &run.Group{}
	p := internalhttp.NewProbe(logger)
	{
		// Main server to listen for public APIs.
		mux := http.NewServeMux()
		mux.Handle("/receive", internalhttp.NewMetricsMiddleware(reg).NewHandler(
			"receive", http.HandlerFunc(receive(logger))),
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
				level.Error(logger).Log("msg", "server shutdown failed")
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
		// Internal server to expose introspection APIs.
		mux := http.NewServeMux()

		// Register metrics server.
		mux.Handle("/metrics", promhttp.HandlerFor(reg, promhttp.HandlerOpts{}))

		// Register pprof endpoints.
		registerProfiler(mux)

		// Register health checks.
		registerProbes(mux, p)

		internalSrv := &http.Server{
			Addr:    cfg.server.listenInternal,
			Handler: mux,
		}
		g.Add(func() error {
			return internalSrv.ListenAndServe()
		}, func(error) {
			ctx, cancel := context.WithTimeout(context.Background(), gracePeriod)
			defer cancel()

			if err := internalSrv.Shutdown(ctx); err != nil {
				level.Error(logger).Log("msg", "internal server shutdown failed")
			}
		})
	}

	if err := g.Run(); err != nil {
		level.Error(logger).Log("msg", "run group failed", "err", err)
		os.Exit(1)
	}
}

func receive(logger log.Logger) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		compressed, err := ioutil.ReadAll(r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		reqBuf, err := snappy.Decode(nil, compressed)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		var req prompb.WriteRequest
		if err := proto.Unmarshal(reqBuf, &req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		for _, ts := range req.Timeseries {
			m := make(model.Metric, len(ts.Labels))
			for _, l := range ts.Labels {
				m[model.LabelName(l.Name)] = model.LabelValue(l.Value)
			}

			level.Info(logger).Log("msg", m)

			for _, s := range ts.Samples {
				level.Info(logger).Log("msg", fmt.Sprintf("  %f %d", s.Value, s.Timestamp))
			}
		}
	}
}

func registerProbes(mux *http.ServeMux, p *internalhttp.Probe) {
	if p != nil {
		mux.Handle("/-/healthy", p.HealthyHandler())
		mux.Handle("/-/ready", p.ReadyHandler())
	}
}

func registerProfiler(mux *http.ServeMux) {
	mux.HandleFunc("/debug/pprof/", pprof.Index)
	mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	mux.HandleFunc("/debug/pprof/trace", pprof.Trace)
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
	flag.Parse()

	return cfg
}

func interrupt(logger log.Logger, cancel <-chan struct{}) error {
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)
	select {
	case s := <-c:
		level.Info(logger).Log("msg", "caught signal. Exiting.", "signal", s)
		return nil
	case <-cancel:
		return errCancelled
	}
}
