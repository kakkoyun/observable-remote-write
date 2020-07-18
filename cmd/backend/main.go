package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"time"

	"github.com/oklog/run"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/common/version"

	"github.com/kakkoyun/observable-remote-write/pkg/exthttp"
)

// gracePeriod specify graceful shutdown period.
const gracePeriod = 10 * time.Second

func main() {
	fmt.Println("Hello World from the Backend!")

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

		srv := &http.Server{Handler: mux}

		l, err := net.Listen("tcp", ":8080")
		if err != nil {
			log.Fatalf("new listener failed %v; exiting\n", err)
		}
		g.Add(func() error {
			return srv.Serve(l)
		}, func(error) {
			ctx, cancel := context.WithTimeout(context.Background(), gracePeriod)
			defer cancel()

			if err := srv.Shutdown(ctx); err != nil {
				log.Println("error: server shutdown failed")
			}
		})
	}

	if err := g.Run(); err != nil {
		log.Fatalf("running command failed %v; exiting\n", err)
	}

	log.Println("exiting")
}
