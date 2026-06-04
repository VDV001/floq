package metrics_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

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
