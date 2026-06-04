package analytics_test

import (
	"context"
	"encoding/json"
	"math"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/daniil/floq/internal/analytics"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stubInboxFlowReader records the window it was called with and returns
// a canned DTO — keeps the handler test DB-free.
type stubInboxFlowReader struct {
	dto       *analytics.InboxFlowDTO
	err       error
	gotUserID uuid.UUID
	gotFrom   time.Time
	gotTo     time.Time
	calls     int
}

func (s *stubInboxFlowReader) GetInboxFlow(_ context.Context, userID uuid.UUID, from, to time.Time) (*analytics.InboxFlowDTO, error) {
	s.calls++
	s.gotUserID = userID
	s.gotFrom = from
	s.gotTo = to
	if s.err != nil {
		return nil, s.err
	}
	return s.dto, nil
}

func inboxRouter(reader analytics.InboxFlowReader) chi.Router {
	r := chi.NewRouter()
	analytics.RegisterRoutes(r, analytics.NewUseCase(nil, nil, analytics.WithInboxFlowReader(reader)))
	return r
}

func sampleInboxDTO() *analytics.InboxFlowDTO {
	return &analytics.InboxFlowDTO{
		PeriodFrom: time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC),
		PeriodTo:   time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC),
		Leads: analytics.LeadsBreakdownDTO{
			Total:     120,
			ByChannel: map[string]int{"telegram": 70, "email": 50},
			ByStatus:  map[string]int{"new": 10, "qualified": 40, "closed": 5},
		},
		Qualifications: analytics.QualificationDistributionDTO{
			ScoreHistogram: []analytics.ScoreBucketDTO{
				{Range: "0-20", Count: 5},
				{Range: "81-100", Count: 25},
			},
			AvgScore: 64.5,
		},
		PendingReplies: analytics.PendingRepliesStatsDTO{
			Approved:               80,
			Rejected:               10,
			CurrentlyPending:       5,
			P50TimeToDecideSeconds: 120,
			P95TimeToDecideSeconds: 600,
		},
	}
}

type inboxWire struct {
	Period struct {
		From string `json:"from"`
		To   string `json:"to"`
	} `json:"period"`
	Leads struct {
		Total     int            `json:"total"`
		ByChannel map[string]int `json:"by_channel"`
		ByStatus  map[string]int `json:"by_status"`
	} `json:"leads"`
	Qualifications struct {
		ScoreHistogram []struct {
			Range string `json:"range"`
			Count int    `json:"count"`
		} `json:"score_histogram"`
		AvgScore float64 `json:"avg_score"`
	} `json:"qualifications"`
	PendingReplies struct {
		Approved               int     `json:"approved"`
		Rejected               int     `json:"rejected"`
		CurrentlyPending       int     `json:"currently_pending"`
		ApproveRate            float64 `json:"approve_rate"`
		P50TimeToDecideSeconds int     `json:"p50_time_to_decide_seconds"`
		P95TimeToDecideSeconds int     `json:"p95_time_to_decide_seconds"`
	} `json:"pending_replies"`
}

func TestHandler_GetInboxFlow_ReturnsPayload(t *testing.T) {
	userID := uuid.New()
	reader := &stubInboxFlowReader{dto: sampleInboxDTO()}

	w := httptest.NewRecorder()
	inboxRouter(reader).ServeHTTP(w, newAuthedRequest(t, "/api/analytics/inbox?period=month", userID))

	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, userID, reader.gotUserID)

	var got inboxWire
	require.NoError(t, json.NewDecoder(w.Body).Decode(&got))

	assert.Equal(t, 120, got.Leads.Total)
	assert.Equal(t, 70, got.Leads.ByChannel["telegram"])
	assert.Equal(t, 40, got.Leads.ByStatus["qualified"])
	require.Len(t, got.Qualifications.ScoreHistogram, 2)
	assert.Equal(t, "0-20", got.Qualifications.ScoreHistogram[0].Range)
	assert.Equal(t, 5, got.Qualifications.ScoreHistogram[0].Count)
	assert.InDelta(t, 64.5, got.Qualifications.AvgScore, 0.001)
	assert.Equal(t, 80, got.PendingReplies.Approved)
	assert.Equal(t, 10, got.PendingReplies.Rejected)
	assert.Equal(t, 5, got.PendingReplies.CurrentlyPending)
	assert.Equal(t, 120, got.PendingReplies.P50TimeToDecideSeconds)
	assert.Equal(t, 600, got.PendingReplies.P95TimeToDecideSeconds)
}

func TestHandler_GetInboxFlow_ApproveRateExcludesPending(t *testing.T) {
	reader := &stubInboxFlowReader{dto: sampleInboxDTO()}
	w := httptest.NewRecorder()
	inboxRouter(reader).ServeHTTP(w, newAuthedRequest(t, "/api/analytics/inbox", uuid.New()))
	require.Equal(t, http.StatusOK, w.Code)

	var got inboxWire
	require.NoError(t, json.NewDecoder(w.Body).Decode(&got))
	// 80 / (80 + 10) — currently_pending (5) is undecided and excluded.
	assert.InDelta(t, 80.0/90.0, got.PendingReplies.ApproveRate, 0.0001)
}

func TestHandler_GetInboxFlow_ApproveRateZeroWhenNoDecisions(t *testing.T) {
	dto := sampleInboxDTO()
	dto.PendingReplies = analytics.PendingRepliesStatsDTO{Approved: 0, Rejected: 0, CurrentlyPending: 3}
	reader := &stubInboxFlowReader{dto: dto}
	w := httptest.NewRecorder()
	inboxRouter(reader).ServeHTTP(w, newAuthedRequest(t, "/api/analytics/inbox", uuid.New()))
	require.Equal(t, http.StatusOK, w.Code)

	var got inboxWire
	require.NoError(t, json.NewDecoder(w.Body).Decode(&got))
	assert.Equal(t, 0.0, got.PendingReplies.ApproveRate, "no decisions → zero rate, never NaN")
	assert.False(t, math.IsNaN(got.PendingReplies.ApproveRate))
}

func TestHandler_GetInboxFlow_PeriodWindows(t *testing.T) {
	cases := []struct {
		query      string
		wantSpan   time.Duration
		wantEpoch0 bool
	}{
		{"?period=week", 7 * 24 * time.Hour, false},
		{"?period=month", 30 * 24 * time.Hour, false},
		{"", 30 * 24 * time.Hour, false}, // default is month
		{"?period=all", 0, true},
	}
	for _, tc := range cases {
		t.Run(tc.query, func(t *testing.T) {
			reader := &stubInboxFlowReader{dto: sampleInboxDTO()}
			w := httptest.NewRecorder()
			inboxRouter(reader).ServeHTTP(w, newAuthedRequest(t, "/api/analytics/inbox"+tc.query, uuid.New()))
			require.Equal(t, http.StatusOK, w.Code)
			require.Equal(t, 1, reader.calls)
			if tc.wantEpoch0 {
				assert.Equal(t, time.Unix(0, 0).UTC(), reader.gotFrom, "period=all starts at epoch")
				return
			}
			span := reader.gotTo.Sub(reader.gotFrom)
			assert.InDelta(t, tc.wantSpan.Seconds(), span.Seconds(), 5, "window span")
		})
	}
}

func TestHandler_GetInboxFlow_InvalidPeriod(t *testing.T) {
	reader := &stubInboxFlowReader{dto: sampleInboxDTO()}
	w := httptest.NewRecorder()
	inboxRouter(reader).ServeHTTP(w, newAuthedRequest(t, "/api/analytics/inbox?period=year", uuid.New()))
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Equal(t, 0, reader.calls, "invalid period must not reach the reader")
}

func TestHandler_GetInboxFlow_Unauthenticated(t *testing.T) {
	reader := &stubInboxFlowReader{dto: sampleInboxDTO()}
	w := httptest.NewRecorder()
	inboxRouter(reader).ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/analytics/inbox", nil))
	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Equal(t, 0, reader.calls)
}
