package receiver

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/gogo/protobuf/proto"
	"github.com/golang/snappy"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/prompb"
	"go.opentelemetry.io/otel/api/trace"

	"github.com/kakkoyun/observable-remote-write/internal"
	"github.com/kakkoyun/observable-remote-write/internal/http/middleware"
)

func Receive(logger log.Logger, tracer trace.Tracer) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, span := tracer.Start(r.Context(), "receive")
		defer span.End()

		logger = log.With(logger, "request-id", middleware.RequestIDFromContext(ctx))

		var compressed []byte

		if err := tracer.WithSpan(ctx, "read", func(ctx context.Context) error {
			var err error
			compressed, err = ioutil.ReadAll(r.Body)
			return err
		}); err != nil {
			level.Warn(logger).Log("msg", "http read", "err", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)

			return
		}

		defer internal.ExhaustCloseWithLogOnErr(logger, r.Body)

		var reqBuf []byte

		if err := tracer.WithSpan(ctx, "decode", func(ctx context.Context) error {
			var err error
			reqBuf, err = snappy.Decode(nil, compressed)
			return err
		}); err != nil {
			level.Warn(logger).Log("msg", "snappy decode", "err", err)
			http.Error(w, err.Error(), http.StatusBadRequest)

			return
		}

		var req prompb.WriteRequest

		if err := tracer.WithSpan(ctx, "decode", func(ctx context.Context) error {
			return proto.Unmarshal(reqBuf, &req)
		}); err != nil {
			level.Warn(logger).Log("msg", "proto unmarshalling", "err", err)
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
