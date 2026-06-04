// Package metrics is the application's Prometheus instrumentation. It
// owns a private registry (not the global default) so collectors are
// explicit, tests are isolated, and nothing leaks in via a stray
// init(). The HTTP server exposes it at GET /metrics via Handler();
// Prometheus scrapes on its own schedule (pull model).
package metrics

import (
	"net/http"

	"github.com/daniil/floq/internal/audit/domain"
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
	aiCalls      *prometheus.CounterVec
	aiCost       *prometheus.CounterVec
	aiDuration   *prometheus.HistogramVec
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
		aiCalls: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "ai_calls_total",
			Help: "Total AI provider calls by provider, model and request type.",
		}, aiLabels),
		aiCost: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "ai_cost_micro_usd_total",
			Help: "Cumulative AI spend in micro-USD by provider, model and request type.",
		}, aiLabels),
		aiDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "ai_call_duration_seconds",
			Help:    "AI provider call latency by provider, model and request type.",
			Buckets: prometheus.DefBuckets,
		}, aiLabels),
	}
	reg.MustRegister(
		m.httpRequests,
		m.httpDuration,
		m.aiCalls,
		m.aiCost,
		m.aiDuration,
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
	)
	return m
}

// aiLabels are intentionally limited to bounded, non-tenant dimensions.
// user_id is DELIBERATELY excluded: /metrics is public (no auth), so a
// per-user label would leak tenant activity and explode cardinality.
var aiLabels = []string{"provider", "model", "request_type"}

// OnAuditEntry records AI-call metrics from a constructed audit Entry.
// Wired as the RecordingProvider observer so it fires on every provider
// call (success or error), independent of whether the async recorder
// later persists or drops the row — this is the "calls made" signal.
func (m *Metrics) OnAuditEntry(e *domain.Entry) {
	labels := prometheus.Labels{
		"provider":     e.Provider,
		"model":        e.Model,
		"request_type": string(e.RequestType),
	}
	m.aiCalls.With(labels).Inc()
	m.aiCost.With(labels).Add(float64(e.CostUSDMicro))
	m.aiDuration.With(labels).Observe(float64(e.LatencyMS) / 1000.0)
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
