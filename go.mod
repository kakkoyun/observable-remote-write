module github.com/kakkoyun/observable-remote-write

go 1.14

require (
	github.com/go-chi/chi v4.1.2+incompatible
	github.com/go-kit/kit v0.10.0
	github.com/gogo/protobuf v1.3.1
	github.com/golang/snappy v0.0.1
	github.com/observatorium/observable-demo v0.0.0-20200126103321-15a3f707e7aa
	github.com/oklog/run v1.1.0
	github.com/oklog/ulid v1.3.1
	github.com/pkg/errors v0.9.1
	github.com/prometheus/client_golang v1.7.1
	github.com/prometheus/common v0.10.0
	github.com/prometheus/prometheus v1.8.2-0.20200724102142-6b7ac2ac1b66
	go.opentelemetry.io/otel v0.9.0
	go.opentelemetry.io/otel/exporters/trace/jaeger v0.9.0
)
