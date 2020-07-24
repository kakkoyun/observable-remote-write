package middleware

import (
	"net/http"
	"time"

	chimiddleware "github.com/go-chi/chi/middleware"
	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
)

// Logger returns a middleware to log HTTP requests.
func Logger(logger log.Logger) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			ww := chimiddleware.NewWrapResponseWriter(w, r.ProtoMajor)
			next.ServeHTTP(ww, r)

			keyvals := []interface{}{
				"request", RequestIDFromContext(r.Context()),
				"proto", r.Proto,
				"method", r.Method,
				"status", ww.Status(),
				"content-type", r.Header.Get("Content-Type"),
				"path", r.URL.Path,
				"duration", time.Since(start),
				"bytes", ww.BytesWritten(),
			}

			if ww.Status()/100 == 5 { //nolint:gomnd
				level.Warn(logger).Log(keyvals...)
				return
			}
			level.Debug(logger).Log(keyvals...)
		})
	}
}
