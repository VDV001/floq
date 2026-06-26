package audit_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/daniil/floq/internal/audit"
	"github.com/daniil/floq/internal/audit/domain"
	"github.com/daniil/floq/internal/httputil"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stubAuditRepo is a hand-rolled AuditRepository fake. The cost-summary read
// path is the only method the handler exercises; Save is here to satisfy the
// port.
type stubAuditRepo struct {
	summary   *domain.CostSummary
	err       error
	callCount int
}

func (s *stubAuditRepo) Save(context.Context, []*domain.Entry) error { return nil }

func (s *stubAuditRepo) CostSummary(_ context.Context, _ uuid.UUID, _, _ time.Time) (*domain.CostSummary, error) {
	s.callCount++
	if s.err != nil {
		return nil, s.err
	}
	return s.summary, nil
}

func newAuditRouter(repo domain.AuditRepository) chi.Router {
	r := chi.NewRouter()
	audit.RegisterRoutes(r, audit.NewHandler(audit.NewUseCase(repo)))
	return r
}

func authedGet(t *testing.T, target string, userID uuid.UUID) *http.Request {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, target, nil)
	return req.WithContext(httputil.WithUserID(req.Context(), userID))
}

func TestHandler_GetCostSummary_ReturnsBody(t *testing.T) {
	repo := &stubAuditRepo{summary: &domain.CostSummary{
		TotalUSDMicro: 2_500_000, // → 2.5 USD
		TotalCalls:    5,
		ByRequestType: []domain.RequestTypeBreakdown{
			{RequestType: "qualification", Calls: 3, USDMicro: 1_500_000, InputTokens: 100, OutputTokens: 50},
		},
		ByModel: []domain.ModelBreakdown{
			{Model: "claude-opus-4-8", Calls: 5, USDMicro: 2_500_000, InputTokens: 200, OutputTokens: 80},
		},
		PeriodFrom: time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC),
		PeriodTo:   time.Date(2026, 6, 25, 0, 0, 0, 0, time.UTC),
	}}
	r := newAuditRouter(repo)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, authedGet(t, "/api/audit/cost-summary", uuid.New()))

	require.Equal(t, http.StatusOK, w.Code)
	var got audit.CostSummaryResponse
	require.NoError(t, json.NewDecoder(w.Body).Decode(&got))
	assert.InDelta(t, 2.5, got.TotalUSD, 1e-9, "micro-USD must convert to float USD at the wire")
	assert.Equal(t, 5, got.TotalCalls)
	require.Len(t, got.ByRequestType, 1)
	assert.Equal(t, "qualification", got.ByRequestType[0].RequestType)
	assert.InDelta(t, 1.5, got.ByRequestType[0].USD, 1e-9)
	require.Len(t, got.ByModel, 1)
	assert.Equal(t, "claude-opus-4-8", got.ByModel[0].Model)
	assert.Equal(t, "2026-06-01", got.Period.From)
	assert.Equal(t, "2026-06-25", got.Period.To)
}

func TestHandler_GetCostSummary_ParsesDateParams(t *testing.T) {
	repo := &stubAuditRepo{summary: &domain.CostSummary{}}
	r := newAuditRouter(repo)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, authedGet(t, "/api/audit/cost-summary?from=2026-06-01&to=2026-06-20", uuid.New()))

	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, 1, repo.callCount, "valid params must reach the repo")
}

func TestHandler_GetCostSummary_Unauthorized(t *testing.T) {
	repo := &stubAuditRepo{summary: &domain.CostSummary{}}
	r := newAuditRouter(repo)

	w := httptest.NewRecorder()
	// Plain request — no user_id in context.
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/audit/cost-summary", nil))

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Zero(t, repo.callCount, "missing user_id must NOT reach the repo")
}

func TestHandler_GetCostSummary_BadDateParam(t *testing.T) {
	for _, q := range []string{"?from=nope", "?to=2026-13-99", "?from=2026-06-01&to=notadate"} {
		repo := &stubAuditRepo{summary: &domain.CostSummary{}}
		r := newAuditRouter(repo)

		w := httptest.NewRecorder()
		r.ServeHTTP(w, authedGet(t, "/api/audit/cost-summary"+q, uuid.New()))

		assert.Equal(t, http.StatusBadRequest, w.Code, q)
		assert.Zero(t, repo.callCount, "a malformed date must NOT reach the repo: "+q)
	}
}

func TestHandler_GetCostSummary_InvalidPeriod(t *testing.T) {
	// from after to → the usecase returns ErrInvalidPeriod → 400 (not 500).
	repo := &stubAuditRepo{summary: &domain.CostSummary{}}
	r := newAuditRouter(repo)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, authedGet(t, "/api/audit/cost-summary?from=2026-06-25&to=2026-06-01", uuid.New()))

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandler_GetCostSummary_RepoError(t *testing.T) {
	repo := &stubAuditRepo{err: errors.New("audit_log unavailable")}
	r := newAuditRouter(repo)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, authedGet(t, "/api/audit/cost-summary", uuid.New()))

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}
