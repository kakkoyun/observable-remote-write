package middleware

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type MetricsMiddleware struct {
	requestDuration *prometheus.HistogramVec
	requestSize     *prometheus.SummaryVec
	requestsTotal   *prometheus.CounterVec
	responseSize    *prometheus.SummaryVec
}

// NewMetricsMiddleware provides default MetricsMiddleware.
func NewMetricsMiddleware(reg prometheus.Registerer) *MetricsMiddleware {
	ins := MetricsMiddleware{
		requestDuration: promauto.With(reg).NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "http_request_duration_seconds",
				Help:    "Tracks the latencies for HTTP requests.",
				Buckets: []float64{0.001, 0.01, 0.1, 0.3, 0.6, 1, 3, 6, 9, 20, 30, 60, 90, 120},
			},
			[]string{"code", "handler", "method"},
		),

		requestSize: promauto.With(reg).NewSummaryVec(
			prometheus.SummaryOpts{
				Name: "http_request_size_bytes",
				Help: "Tracks the size of HTTP requests.",
			},
			[]string{"code", "handler", "method"},
		),

		requestsTotal: promauto.With(reg).NewCounterVec(
			prometheus.CounterOpts{
				Name: "http_requests_total",
				Help: "Tracks the number of HTTP requests.",
			}, []string{"code", "handler", "method"},
		),

		responseSize: promauto.With(reg).NewSummaryVec(
			prometheus.SummaryOpts{
				Name: "http_response_size_bytes",
				Help: "Tracks the size of HTTP responses.",
			},
			[]string{"code", "handler", "method"},
		),
	}

	return &ins
}

// NewHandler wraps the given HTTP handler for instrumentation. It
// registers four metric collectors (if not already done) and reports HTTP
// metrics to the (newly or already) registered collectors: http_requests_total
// (CounterVec), http_request_duration_seconds (Histogram),
// http_request_size_bytes (Summary), http_response_size_bytes (Summary). Each
// has a constant label named "handler" with the provided handlerName as
// value. http_requests_total is a metric vector partitioned by HTTP method
// (label name "method") and HTTP status code (label name "code").
func (ins *MetricsMiddleware) NewHandler(handlerName string) func(next http.Handler) http.Handler {
	return func(handler http.Handler) http.Handler {
		return promhttp.InstrumentHandlerDuration(
			ins.requestDuration.MustCurryWith(prometheus.Labels{"handler": handlerName}),
			promhttp.InstrumentHandlerRequestSize(
				ins.requestSize.MustCurryWith(prometheus.Labels{"handler": handlerName}),
				promhttp.InstrumentHandlerCounter(
					ins.requestsTotal.MustCurryWith(prometheus.Labels{"handler": handlerName}),
					promhttp.InstrumentHandlerResponseSize(
						ins.responseSize.MustCurryWith(prometheus.Labels{"handler": handlerName}),
						handler,
					),
				),
			),
		)
	}
}
