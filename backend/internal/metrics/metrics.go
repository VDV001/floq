// Package metrics is the application's Prometheus instrumentation. It
// owns a private registry (not the global default) so collectors are
// explicit, tests are isolated, and nothing leaks in via a stray
// init(). The HTTP server exposes it at GET /metrics via Handler();
// Prometheus scrapes on its own schedule (pull model).
package metrics

import (
	"net/http"
	"sync"
	"time"

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
	queueDepth   *prometheus.GaugeVec
	matviewRefresh prometheus.Histogram
	registryEnrichments *prometheus.CounterVec
	webhookDeliveries   *prometheus.CounterVec
	intakeQuarantines   *prometheus.CounterVec

	mu        sync.Mutex          // guards prevKinds
	prevKinds map[string]struct{} // queue-depth kinds published last scan
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
			Name: "ai_call_duration_seconds",
			Help: "AI provider call latency by provider, model and request type.",
			// AI calls routinely run for tens of seconds (reasoning
			// models, image analysis, long drafts), so the default
			// buckets (cap 10s) would collapse the tail into +Inf and
			// make p95/p99 useless. Buckets extend to 2 minutes.
			Buckets: []float64{0.5, 1, 2.5, 5, 10, 20, 30, 60, 120},
		}, aiLabels),
		queueDepth: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "pending_replies_queue_depth",
			Help: "Number of pending HITL replies awaiting an operator decision, by kind.",
		}, []string{"kind"}),
		matviewRefresh: prometheus.NewHistogram(prometheus.HistogramOpts{
			Name: "analytics_matview_refresh_duration_seconds",
			Help: "Duration of a full analytics materialized-view refresh pass (the background cron). Alert when it approaches ANALYTICS_REFRESH_INTERVAL — the scale-path trigger.",
			// Refreshes range from milliseconds to many seconds as volume
			// grows; default buckets cap at 10s, so extend toward the 5m
			// default interval to keep the slow tail visible.
			Buckets: []float64{0.1, 0.5, 1, 2.5, 5, 10, 30, 60, 120, 300},
		}),
		registryEnrichments: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "enrichment_registry_attempts_total",
			Help: "Total company-registry (DaData) enrichment attempts by result: hit (confident match), miss (no/ambiguous match), error (DaData API/transport failure), rate_limited (skipped: daily quota exhausted), limiter_error (skipped: rate-limiter backend unavailable). Only hit/miss/error reach the API, so actual DaData calls = hit+miss+error; rate_limited and limiter_error send no request. hit-rate = hit / (hit+miss).",
		}, []string{"result"}),
		webhookDeliveries: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "webhook_deliveries_total",
			Help: "Total outgoing webhook delivery attempts by event type and result: delivered (2xx) or failed (transport error / non-2xx). Each attempt is one HTTP request; a delivery retried N times counts N attempts.",
		}, []string{"event_type", "result"}),
		intakeQuarantines: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "inbox_intake_quarantined_total",
			Help: "Total inbound intake sources quarantined by channel (email / telegram): a fail-closed poller (#206) exhausted its retry cap on a deterministically-failing source and consumed it to stop hot-looping (#208). Any non-zero rate warrants investigation — a quarantined source's lead was not ingested.",
		}, []string{"channel"}),
	}
	reg.MustRegister(
		m.httpRequests,
		m.httpDuration,
		m.aiCalls,
		m.aiCost,
		m.aiDuration,
		m.queueDepth,
		m.matviewRefresh,
		m.registryEnrichments,
		m.webhookDeliveries,
		m.intakeQuarantines,
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
	)
	return m
}

// RegisterDropsSource wires a CounterFunc that reports the cumulative
// audit-recorder drop count (buffer overflow / record-after-stop). The
// source is read live on each scrape, so it always reflects the current
// atomic counter without any push wiring. CounterFunc (not GaugeFunc) so
// the monotonic source matches the _total suffix and rate() works in
// PromQL. Call once at startup.
func (m *Metrics) RegisterDropsSource(get func() float64) {
	m.registry.MustRegister(prometheus.NewCounterFunc(prometheus.CounterOpts{
		Name: "audit_log_drops_total",
		Help: "Cumulative audit_log entries dropped by the async recorder (buffer overflow or record-after-stop).",
	}, get))
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

// ObserveMatviewRefresh records the wall-clock duration of one analytics
// matview refresh pass. Wired as the analytics RefreshCron observer so the
// /metrics histogram reflects refresh latency without the analytics context
// importing this package.
func (m *Metrics) ObserveMatviewRefresh(d time.Duration) {
	m.matviewRefresh.Observe(d.Seconds())
}

// OnRegistryEnrichment records the outcome of one company-registry
// (DaData) enrichment attempt. Wired as the daDataEnricher observer so the
// counter reflects every attempt that had a signal to look up. The `result`
// label is a small bounded enum (hit / miss / error / rate_limited) — no
// tenant or company dimension, so /metrics stays public-safe and low
// cardinality. No-signal early returns are deliberately never observed: no
// lookup was attempted, so they are not registry requests.
func (m *Metrics) OnRegistryEnrichment(result string) {
	m.registryEnrichments.WithLabelValues(result).Inc()
}

// OnWebhookDelivery records the outcome of one outgoing webhook delivery
// attempt (#181), wired as the delivery worker's observer. Labels are the event
// type and a bounded result enum (delivered / failed) — no URL or tenant
// dimension, so /metrics stays public-safe and low cardinality.
func (m *Metrics) OnWebhookDelivery(eventType string, success bool) {
	result := "failed"
	if success {
		result = "delivered"
	}
	m.webhookDeliveries.WithLabelValues(eventType, result).Inc()
}

// OnIntakeQuarantine records that one inbound intake source was quarantined on
// the given channel ("email" / "telegram") after exhausting its retry cap (#208).
// Wired as the poller's quarantine observer so the inbox package never imports
// this one. The channel label is a small bounded enum — no source or tenant
// dimension, so /metrics stays public-safe and low cardinality.
func (m *Metrics) OnIntakeQuarantine(channel string) {
	m.intakeQuarantines.WithLabelValues(channel).Inc()
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
