package analytics_test

import (
	"context"
	"encoding/json"
	"errors"
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

// stubCostReader records the [from, to) window the handler computes
// from the period query param so the test can assert the window logic
// without DB.
type stubCostReader struct {
	dto       *analytics.CostRatiosDTO
	err       error
	gotUserID uuid.UUID
	gotFrom   time.Time
	gotTo     time.Time
	callCount int
}

func (s *stubCostReader) GetCostRatios(_ context.Context, userID uuid.UUID, from, to time.Time) (*analytics.CostRatiosDTO, error) {
	s.callCount++
	s.gotUserID = userID
	s.gotFrom = from
	s.gotTo = to
	if s.err != nil {
		return nil, s.err
	}
	return s.dto, nil
}

// newCostUseCase builds a UseCase wired to a stub reader for both
// ports. The seq port is reused from handler_test.go's stubReader (or
// nil — the cost-ratios handler doesn't call it).
func newCostUseCase(cost analytics.CostRatiosReader) *analytics.UseCase {
	return analytics.NewUseCase(nil, cost)
}

func dummyCostDTO(totalMicro int64, leads, qualified, converted, drafts int) *analytics.CostRatiosDTO {
	return &analytics.CostRatiosDTO{
		PeriodFrom:               time.Now().UTC().Add(-7 * 24 * time.Hour),
		PeriodTo:                 time.Now().UTC(),
		TotalCostUSDMicro:        totalMicro,
		TotalCalls:               42,
		LeadsCount:               leads,
		QualifiedLeadsCount:      qualified,
		ConvertedCount:           converted,
		DraftsSentCount:          drafts,
		CostPerLeadUSDMicro:      safeRatioForTest(totalMicro, leads),
		CostPerQualifiedUSDMicro: safeRatioForTest(totalMicro, qualified),
		CostPerConvertedUSDMicro: safeRatioForTest(totalMicro, converted),
		CostPerDraftSentUSDMicro: safeRatioForTest(totalMicro, drafts),
	}
}

func safeRatioForTest(total int64, count int) int64 {
	if count <= 0 {
		return 0
	}
	return total / int64(count)
}

func TestHandler_GetCostRatios_ReturnsFloatUSD(t *testing.T) {
	userID := uuid.New()
	reader := &stubCostReader{dto: dummyCostDTO(6_000_000, 4, 2, 1, 5)}

	r := chi.NewRouter()
	analytics.RegisterRoutes(r, newCostUseCase(reader))

	w := httptest.NewRecorder()
	r.ServeHTTP(w, newAuthedRequest(t, "/api/analytics/cost-ratios", userID))

	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, userID, reader.gotUserID)

	var got map[string]any
	require.NoError(t, json.NewDecoder(w.Body).Decode(&got))
	assert.InDelta(t, 6.0, got["total_cost_usd"], 0.0001, "USD float = micro / 1_000_000")
	assert.EqualValues(t, 42, got["total_calls"])
	assert.EqualValues(t, 4, got["leads_count"])
	assert.EqualValues(t, 2, got["qualified_leads_count"])
	assert.EqualValues(t, 1, got["converted_count"])
	assert.EqualValues(t, 5, got["drafts_sent_count"])
	assert.InDelta(t, 1.5, got["cost_per_lead_usd"], 0.0001, "6 / 4")
	assert.InDelta(t, 3.0, got["cost_per_qualified_lead_usd"], 0.0001, "6 / 2")
	assert.InDelta(t, 6.0, got["cost_per_converted_usd"], 0.0001, "6 / 1")
	assert.InDelta(t, 1.2, got["cost_per_draft_sent_usd"], 0.0001, "6 / 5")

	period, _ := got["period"].(map[string]any)
	require.NotNil(t, period, "response must include period.from/to")
	assert.Contains(t, period, "from")
	assert.Contains(t, period, "to")
}

func TestHandler_GetCostRatios_PeriodQueryWindow(t *testing.T) {
	tests := []struct {
		name         string
		query        string
		wantHasLower bool          // expects [from] not at epoch
		wantSpanMax  time.Duration // tolerable upper bound (test stability)
		wantStatus   int
	}{
		{"empty defaults to month", "", true, 31 * 24 * time.Hour, http.StatusOK},
		{"week", "?period=week", true, 8 * 24 * time.Hour, http.StatusOK},
		{"month", "?period=month", true, 31 * 24 * time.Hour, http.StatusOK},
		{"all", "?period=all", false, 0, http.StatusOK},
		{"invalid -> 400", "?period=quarter", false, 0, http.StatusBadRequest},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			reader := &stubCostReader{dto: dummyCostDTO(0, 0, 0, 0, 0)}
			r := chi.NewRouter()
			analytics.RegisterRoutes(r, newCostUseCase(reader))

			w := httptest.NewRecorder()
			r.ServeHTTP(w, newAuthedRequest(t, "/api/analytics/cost-ratios"+tc.query, uuid.New()))

			require.Equal(t, tc.wantStatus, w.Code)
			if tc.wantStatus != http.StatusOK {
				assert.Zero(t, reader.callCount, "invalid period must NOT reach reader")
				return
			}
			require.Equal(t, 1, reader.callCount)
			if tc.wantHasLower {
				// The lower bound must be within the expected span from `to`.
				span := reader.gotTo.Sub(reader.gotFrom)
				assert.True(t, span > 0 && span <= tc.wantSpanMax+time.Minute,
					"span %v exceeds tolerated upper bound %v", span, tc.wantSpanMax)
			} else {
				// PeriodAll → from is the epoch sentinel.
				assert.True(t, reader.gotFrom.Year() <= 1970, "all-time period must use epoch as from")
			}
		})
	}
}

func TestHandler_GetCostRatios_Unauthorized(t *testing.T) {
	reader := &stubCostReader{}
	r := chi.NewRouter()
	analytics.RegisterRoutes(r, newCostUseCase(reader))

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/analytics/cost-ratios", nil))

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Zero(t, reader.callCount)
}

func TestHandler_GetCostRatios_ReaderError(t *testing.T) {
	reader := &stubCostReader{err: errors.New("pg down")}
	r := chi.NewRouter()
	analytics.RegisterRoutes(r, newCostUseCase(reader))

	w := httptest.NewRecorder()
	r.ServeHTTP(w, newAuthedRequest(t, "/api/analytics/cost-ratios", uuid.New()))

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestHandler_GetCostRatios_ZeroDenominatorYieldsZeroRatio(t *testing.T) {
	reader := &stubCostReader{dto: dummyCostDTO(0, 0, 0, 0, 0)}
	r := chi.NewRouter()
	analytics.RegisterRoutes(r, newCostUseCase(reader))

	w := httptest.NewRecorder()
	r.ServeHTTP(w, newAuthedRequest(t, "/api/analytics/cost-ratios", uuid.New()))

	require.Equal(t, http.StatusOK, w.Code)
	var got map[string]any
	require.NoError(t, json.NewDecoder(w.Body).Decode(&got))
	assert.EqualValues(t, 0.0, got["cost_per_lead_usd"], "zero leads → zero ratio, never NaN/Inf")
}

// Compile-time guard: existing stubReader still satisfies the sequence
// port so the original handler tests keep working.
var _ analytics.SequenceStatsReader = (*stubReader)(nil)
