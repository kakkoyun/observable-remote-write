package http

import (
	"io"
	"net/http"
	"sync/atomic"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
)

type check func() bool

// Probe represents health and readiness status of given component, and provides HTTP integration.
type Probe struct {
	logger log.Logger

	ready   uint32
	healthy uint32
}

// NewProbe returns Probe representing readiness and healthiness of given component.
func NewProbe(logger log.Logger) *Probe {
	return &Probe{logger: logger}
}

// HealthyHandler returns a HTTP Handler which responds health checks.
func (p *Probe) HealthyHandler() http.HandlerFunc {
	return p.handler(p.isHealthy)
}

// ReadyHandler returns a HTTP Handler which responds readiness checks.
func (p *Probe) ReadyHandler() http.HandlerFunc {
	return p.handler(p.isReady)
}

func (p *Probe) handler(c check) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		if !c() {
			http.Error(w, "NOT OK", http.StatusServiceUnavailable)
			return
		}

		if _, err := io.WriteString(w, "OK"); err != nil {
			level.Error(p.logger).Log("msg", "failed to write probe response", "err", err)
		}
	}
}

// isReady returns true if component is ready.
func (p *Probe) isReady() bool {
	ready := atomic.LoadUint32(&p.ready)
	return ready > 0
}

// isHealthy returns true if component is healthy.
func (p *Probe) isHealthy() bool {
	healthy := atomic.LoadUint32(&p.healthy)
	return healthy > 0
}

// Ready sets components status to ready.
func (p *Probe) Ready() {
	atomic.SwapUint32(&p.ready, 1)
}

// NotReady sets components status to not ready with given error as a cause.
func (p *Probe) NotReady(err error) {
	atomic.SwapUint32(&p.ready, 0)
}

// Healthy sets components status to healthy.
func (p *Probe) Healthy() {
	atomic.SwapUint32(&p.healthy, 1)
}

// NotHealthy sets components status to not healthy with given error as a cause.
func (p *Probe) NotHealthy(err error) {
	atomic.SwapUint32(&p.healthy, 0)
}
