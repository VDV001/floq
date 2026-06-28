package metrics_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/daniil/floq/internal/audit/domain"
	"github.com/daniil/floq/internal/metrics"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// scrape renders the registry in the Prometheus text exposition format
// so tests can assert on exact metric+label lines.
func scrape(t *testing.T, m *metrics.Metrics) string {
	t.Helper()
	rec := httptest.NewRecorder()
	m.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	require.Equal(t, http.StatusOK, rec.Code)
	body, err := io.ReadAll(rec.Body)
	require.NoError(t, err)
	return string(body)
}

func TestHandler_ExposesGoAndProcessCollectors(t *testing.T) {
	m := metrics.New()

	rec := httptest.NewRecorder()
	m.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	require.Equal(t, http.StatusOK, rec.Code)

	body, err := io.ReadAll(rec.Body)
	require.NoError(t, err)
	out := string(body)

	// Runtime + process collectors must be wired into the same registry.
	assert.True(t, strings.Contains(out, "go_goroutines"), "Go runtime collector must be registered")
	assert.True(t, strings.Contains(out, "process_"), "process collector must be registered")
}

func TestOnAuditEntry_RecordsCostCallsAndDuration(t *testing.T) {
	m := metrics.New()
	entry := &domain.Entry{
		UserID:       uuid.New(),
		Provider:     "openai",
		Model:        "gpt-4o-mini",
		RequestType:  domain.RequestTypeQualification,
		CostUSDMicro: 45,
		LatencyMS:    1200,
		Status:       domain.StatusSuccess,
	}
	m.OnAuditEntry(entry)
	m.OnAuditEntry(entry)

	body := scrape(t, m)
	// Prometheus sorts label names alphabetically: model, provider, request_type.
	assert.Contains(t, body, `ai_calls_total{model="gpt-4o-mini",provider="openai",request_type="qualification"} 2`)
	assert.Contains(t, body, `ai_cost_micro_usd_total{model="gpt-4o-mini",provider="openai",request_type="qualification"} 90`)
	assert.Contains(t, body, `ai_call_duration_seconds_count{model="gpt-4o-mini",provider="openai",request_type="qualification"} 2`)
}

func TestOnAuditEntry_LatencyHistogramCapturesSlowCalls(t *testing.T) {
	m := metrics.New()
	m.OnAuditEntry(&domain.Entry{
		UserID:      uuid.New(),
		Provider:    "openai",
		Model:       "o1",
		RequestType: domain.RequestTypeQualification,
		LatencyMS:   30_000, // 30s — realistic for reasoning / image analysis
		Status:      domain.StatusSuccess,
	})

	body := scrape(t, m)
	// A 30s call must land in a FINITE bucket, not only +Inf. The default
	// Prometheus buckets cap at 10s, which would collapse the AI latency
	// tail (exactly where p95/p99 matter) — so a >10s bucket must exist.
	assert.Contains(t, body, `ai_call_duration_seconds_bucket{model="o1",provider="openai",request_type="qualification",le="60"} 1`)
}

func TestRegisterDropsSource_ExposesCumulativeDropCount(t *testing.T) {
	m := metrics.New()
	dropped := 0
	m.RegisterDropsSource(func() float64 { return float64(dropped) })

	assert.Contains(t, scrape(t, m), "audit_log_drops_total 0")

	dropped = 7 // GaugeFunc reads live on each scrape
	assert.Contains(t, scrape(t, m), "audit_log_drops_total 7")
}

func TestSetPendingReplyDepth_ExposesGaugePerKind(t *testing.T) {
	m := metrics.New()
	m.SetPendingReplyDepth(map[string]int{"booking_link": 3})

	assert.Contains(t, scrape(t, m), `pending_replies_queue_depth{kind="booking_link"} 3`)
}

func TestSetPendingReplyDepth_ResetsDrainedKinds(t *testing.T) {
	m := metrics.New()
	m.SetPendingReplyDepth(map[string]int{"booking_link": 3})
	m.SetPendingReplyDepth(map[string]int{}) // queue drained

	// A kind that dropped to zero must not linger at its last value.
	assert.NotContains(t, scrape(t, m), `pending_replies_queue_depth{kind="booking_link"}`)
}

func TestOnAuditEntry_NeverLabelsByUserID(t *testing.T) {
	m := metrics.New()
	userID := uuid.New()
	m.OnAuditEntry(&domain.Entry{
		UserID:      userID,
		Provider:    "anthropic",
		Model:       "claude-haiku-4-5",
		RequestType: domain.RequestTypeColdMessage,
		LatencyMS:   10,
		Status:      domain.StatusSuccess,
	})

	body := scrape(t, m)
	// /metrics is public (no auth) — a per-user label would leak tenant
	// activity to anyone who can reach the scrape endpoint.
	assert.NotContains(t, body, userID.String(), "user_id must never appear as a label")
}

func TestOnRegistryEnrichment_RecordsByResult(t *testing.T) {
	m := metrics.New()
	m.OnRegistryEnrichment("hit")
	m.OnRegistryEnrichment("hit")
	m.OnRegistryEnrichment("miss")
	m.OnRegistryEnrichment("error")
	m.OnRegistryEnrichment("rate_limited")

	body := scrape(t, m)
	// Each result is its own series so ops read hit-rate and quota pressure
	// separately. Actual DaData API calls = hit+miss+error; rate_limited is a
	// pre-request skip — hence "attempts", not "requests".
	assert.Contains(t, body, `enrichment_registry_attempts_total{result="hit"} 2`)
	assert.Contains(t, body, `enrichment_registry_attempts_total{result="miss"} 1`)
	assert.Contains(t, body, `enrichment_registry_attempts_total{result="error"} 1`)
	assert.Contains(t, body, `enrichment_registry_attempts_total{result="rate_limited"} 1`)
}

func TestOnIntakeQuarantine_RecordsByChannel(t *testing.T) {
	m := metrics.New()
	m.OnIntakeQuarantine("email")
	m.OnIntakeQuarantine("email")
	m.OnIntakeQuarantine("telegram")

	body := scrape(t, m)
	// One series per intake channel so ops can alert on a poison-source spike
	// (a deterministic intake failure that exhausted its retry cap, #208) and
	// see which channel is affected. No tenant/source dimension keeps /metrics
	// public-safe and low cardinality.
	assert.Contains(t, body, `inbox_intake_quarantined_total{channel="email"} 2`)
	assert.Contains(t, body, `inbox_intake_quarantined_total{channel="telegram"} 1`)
}

func TestObserveMatviewRefresh_RecordsDuration(t *testing.T) {
	m := metrics.New()
	m.ObserveMatviewRefresh(2 * time.Second)
	m.ObserveMatviewRefresh(5 * time.Second)

	body := scrape(t, m)
	// The histogram must expose a count of the observed analytics matview
	// refreshes so ops can alert when refresh latency approaches the interval
	// (the scale-path trigger).
	assert.Contains(t, body, "analytics_matview_refresh_duration_seconds_count 2")
}
