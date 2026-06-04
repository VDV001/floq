// Package metrics is the application's Prometheus instrumentation. It
// owns a private registry (not the global default) so collectors are
// explicit, tests are isolated, and nothing leaks in via a stray
// init(). The HTTP server exposes it at GET /metrics via Handler();
// Prometheus scrapes on its own schedule (pull model).
package metrics

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Metrics holds every collector and the registry they are registered
// with. Construct once at startup and thread it where instrumentation
// is needed (HTTP middleware today; AI-cost and queue-depth hooks land
// in follow-up slices of #94).
type Metrics struct {
	registry     *prometheus.Registry
	httpRequests *prometheus.CounterVec
	httpDuration *prometheus.HistogramVec
}

// New builds the registry, registers the HTTP collectors plus the Go
// runtime and process collectors, and returns the ready Metrics.
func New() *Metrics {
	reg := prometheus.NewRegistry()
	m := &Metrics{
		registry: reg,
		httpRequests: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total HTTP requests by matched route pattern, method and status code.",
		}, []string{"route", "method", "status"}),
		httpDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "HTTP request latency by matched route pattern, method and status code.",
			Buckets: prometheus.DefBuckets,
		}, []string{"route", "method", "status"}),
	}
	reg.MustRegister(
		m.httpRequests,
		m.httpDuration,
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
	)
	return m
}

// Handler serves the registry in the Prometheus text exposition format.
// Mount at GET /metrics, public (no auth) — Prometheus scrapes it.
func (m *Metrics) Handler() http.Handler {
	return promhttp.HandlerFor(m.registry, promhttp.HandlerOpts{})
}

// RequestsCounter exposes the http_requests_total child for a given
// label set. Intended for tests asserting the middleware wired the
// right labels; production code increments via the middleware.
func (m *Metrics) RequestsCounter(route, method, status string) prometheus.Counter {
	return m.httpRequests.WithLabelValues(route, method, status)
}
