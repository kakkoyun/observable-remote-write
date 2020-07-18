package main

import (
	"context"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"time"

	"github.com/go-kit/kit/log/level"
	"github.com/gogo/protobuf/proto"
	"github.com/golang/snappy"
	"github.com/oklog/run"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/common/model"
	"github.com/prometheus/common/version"
	"github.com/prometheus/prometheus/prompb"

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
	fmt.Println("Hello World from the Backend!")

	// Parse flags and initialize config struct.
	cfg := parseFlags()

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
	{
		// Server listen for backend.
		mux := http.NewServeMux()
		mux.Handle("/metrics", exthttp.NewMetricsMiddleware(reg).NewHandler(
			"metrics", promhttp.HandlerFor(reg, promhttp.HandlerOpts{}),
		))
		mux.Handle("/receive", exthttp.NewMetricsMiddleware(reg).NewHandler(
			"receive", http.HandlerFunc(receive)),
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

	if err := g.Run(); err != nil {
		level.Error(logger).Log("msg", "run group failed", "err", err)
		os.Exit(1)
	}
}

func receive(w http.ResponseWriter, r *http.Request) {
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

		fmt.Println(m)

		for _, s := range ts.Samples {
			fmt.Printf("  %f %d\n", s.Value, s.Timestamp)
		}
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
	flag.StringVar(&cfg.server.healthCheckURL, "web.health-check.url", "http://localhost:8080",
		"The URL against which to run health checks.")
	flag.Parse()

	return cfg
}
