package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/golang/snappy"
	"github.com/oklog/run"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/common/model"
	"github.com/prometheus/common/version"
	"github.com/prometheus/prometheus/prompb"

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
		mux.Handle("/receive", exthttp.NewMetricsMiddleware(reg).NewHandler(
			"receive", http.HandlerFunc(receive)),
		)

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
