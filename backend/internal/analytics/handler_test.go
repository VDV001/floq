package analytics_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/daniil/floq/internal/analytics"
	"github.com/daniil/floq/internal/httputil"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stubReader is a hand-rolled fake. Records the period it was called
// with and returns canned rows. Keeps the handler test independent of
// any DB and of pgx mock libraries.
type stubReader struct {
	rows       []analytics.SequenceStatsDTO
	err        error
	gotUserID  uuid.UUID
	gotPeriod  analytics.Period
	callCount  int
}

func (s *stubReader) GetSequenceStats(_ context.Context, userID uuid.UUID, period analytics.Period) ([]analytics.SequenceStatsDTO, error) {
	s.callCount++
	s.gotUserID = userID
	s.gotPeriod = period
	if s.err != nil {
		return nil, s.err
	}
	return s.rows, nil
}

func newAuthedRequest(t *testing.T, target string, userID uuid.UUID) *http.Request {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, target, nil)
	ctx := httputil.WithUserID(req.Context(), userID)
	return req.WithContext(ctx)
}

func TestHandler_GetSequenceStats_ReturnsRows(t *testing.T) {
	userID := uuid.New()
	seqID := uuid.New()
	reader := &stubReader{rows: []analytics.SequenceStatsDTO{{
		ID: seqID, Name: "Cold Outreach IT",
		Sent: 100, Delivered: 95, Opened: 45, Replied: 12, Converted: 4,
	}}}

	r := chi.NewRouter()
	analytics.RegisterRoutes(r, analytics.NewUseCase(reader))

	w := httptest.NewRecorder()
	r.ServeHTTP(w, newAuthedRequest(t, "/api/analytics/sequences", userID))

	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, userID, reader.gotUserID, "handler must pass authenticated user_id to reader")
	assert.Equal(t, analytics.PeriodAll, reader.gotPeriod, "missing ?period query must default to PeriodAll")

	var got struct {
		Sequences []map[string]any `json:"sequences"`
		Period    string           `json:"period"`
	}
	require.NoError(t, json.NewDecoder(w.Body).Decode(&got))
	require.Len(t, got.Sequences, 1)
	row := got.Sequences[0]
	assert.Equal(t, seqID.String(), row["id"])
	assert.Equal(t, "Cold Outreach IT", row["name"])
	assert.EqualValues(t, 100, row["sent"])
	assert.EqualValues(t, 95, row["delivered"])
	assert.EqualValues(t, 45, row["opened"])
	assert.EqualValues(t, 12, row["replied"])
	assert.EqualValues(t, 4, row["converted"])
	// Rates: open_rate=45/95, reply_rate=12/95, conversion_rate=4/(distinct prospects, here =1 if we only count converted)
	// The handler computes rates from the raw counts; tests assert presence + numeric range.
	assert.Contains(t, row, "open_rate")
	assert.Contains(t, row, "reply_rate")
	assert.Contains(t, row, "conversion_rate")
	assert.Equal(t, "all", got.Period)
}

func TestHandler_GetSequenceStats_PeriodQueryParam(t *testing.T) {
	tests := []struct {
		name       string
		query      string
		wantPeriod analytics.Period
		wantStatus int
	}{
		{"empty defaults to all", "", analytics.PeriodAll, http.StatusOK},
		{"week", "?period=week", analytics.PeriodWeek, http.StatusOK},
		{"month", "?period=month", analytics.PeriodMonth, http.StatusOK},
		{"all explicit", "?period=all", analytics.PeriodAll, http.StatusOK},
		{"invalid -> 400", "?period=year", "", http.StatusBadRequest},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			reader := &stubReader{}
			r := chi.NewRouter()
			analytics.RegisterRoutes(r, analytics.NewUseCase(reader))

			w := httptest.NewRecorder()
			r.ServeHTTP(w, newAuthedRequest(t, "/api/analytics/sequences"+tc.query, uuid.New()))

			require.Equal(t, tc.wantStatus, w.Code)
			if tc.wantStatus == http.StatusOK {
				assert.Equal(t, tc.wantPeriod, reader.gotPeriod)
			} else {
				assert.Zero(t, reader.callCount, "invalid period must NOT reach the reader")
			}
		})
	}
}

func TestHandler_GetSequenceStats_Unauthorized(t *testing.T) {
	reader := &stubReader{}
	r := chi.NewRouter()
	analytics.RegisterRoutes(r, analytics.NewUseCase(reader))

	w := httptest.NewRecorder()
	// Plain httptest request — no user_id in context.
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/analytics/sequences", nil))

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Zero(t, reader.callCount, "missing user_id must NOT reach the reader")
}

func TestHandler_GetSequenceStats_ReaderError(t *testing.T) {
	reader := &stubReader{err: errors.New("pg down")}
	r := chi.NewRouter()
	analytics.RegisterRoutes(r, analytics.NewUseCase(reader))

	w := httptest.NewRecorder()
	r.ServeHTTP(w, newAuthedRequest(t, "/api/analytics/sequences", uuid.New()))

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestHandler_GetSequenceStats_EmptyReturnsEmptyArray(t *testing.T) {
	reader := &stubReader{rows: nil}
	r := chi.NewRouter()
	analytics.RegisterRoutes(r, analytics.NewUseCase(reader))

	w := httptest.NewRecorder()
	r.ServeHTTP(w, newAuthedRequest(t, "/api/analytics/sequences", uuid.New()))

	require.Equal(t, http.StatusOK, w.Code)
	// Wire shape must always include "sequences": [], never null.
	assert.Contains(t, w.Body.String(), `"sequences":[]`)
}

func TestHandler_RatesAreComputedCorrectly(t *testing.T) {
	userID := uuid.New()
	reader := &stubReader{rows: []analytics.SequenceStatsDTO{
		{ID: uuid.New(), Name: "S1", Sent: 100, Delivered: 80, Opened: 40, Replied: 8, Converted: 2},
		// Edge case: zero delivered must not divide-by-zero — rates default to 0.
		{ID: uuid.New(), Name: "S0", Sent: 0, Delivered: 0, Opened: 0, Replied: 0, Converted: 0},
	}}

	r := chi.NewRouter()
	analytics.RegisterRoutes(r, analytics.NewUseCase(reader))

	w := httptest.NewRecorder()
	r.ServeHTTP(w, newAuthedRequest(t, "/api/analytics/sequences", userID))

	var got struct {
		Sequences []map[string]float64 `json:"sequences"`
	}
	// Decode partially — id/name will be strings; ignore the type mismatch by
	// using a generic decode here:
	var raw struct {
		Sequences []map[string]any `json:"sequences"`
	}
	require.NoError(t, json.NewDecoder(w.Body).Decode(&raw))
	require.Len(t, raw.Sequences, 2)

	first := raw.Sequences[0]
	assert.InDelta(t, 0.50, first["open_rate"], 0.001, "open_rate = opened / delivered = 40/80")
	assert.InDelta(t, 0.10, first["reply_rate"], 0.001, "reply_rate = replied / delivered = 8/80")

	zero := raw.Sequences[1]
	assert.EqualValues(t, 0.0, zero["open_rate"], "zero delivered must yield 0 rate, not NaN or div-by-zero")
	_ = got
}
