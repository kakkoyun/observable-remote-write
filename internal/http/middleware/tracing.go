package middleware

import (
	"fmt"
	"net/http"

	"github.com/go-kit/kit/log"
	"go.opentelemetry.io/otel/api/correlation"
	"go.opentelemetry.io/otel/api/standard"
	"go.opentelemetry.io/otel/api/trace"
	"go.opentelemetry.io/otel/instrumentation/httptrace"
)

// HTTPMiddleware returns an HTTP handler that injects the given tracer and starts a new server span.
// If any client span is fetched from the wire, we include that as our parent.
func Tracer(logger log.Logger, tracer trace.Tracer, name string) func(next http.Handler) http.Handler {
	operation := fmt.Sprintf("/%s HTTP[server]", name)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()
			attrs, entries, spanCtx := httptrace.Extract(ctx, r)
			r = r.WithContext(correlation.ContextWithMap(ctx, correlation.NewMap(correlation.MapUpdate{
				MultiKV: entries,
			})))

			// TODO:?
			// ctx, req = httptrace.W3C(ctx, req)
			// httptrace.Inject(ctx, r)

			ctx, span := tracer.Start(
				trace.ContextWithRemoteSpanContext(ctx, spanCtx),
				name,
				trace.WithAttributes(attrs...),
				trace.WithAttributes(standard.HTTPServerAttributesFromHTTPRequest(operation, "", r)...),
			)
			defer span.End()

			span.AddEvent(ctx, operation)
			// TODO: request id?
			// ext.HTTPMethod.Set(span, r.Method)
			// ext.HTTPUrl.Set(span, r.URL.String())

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
