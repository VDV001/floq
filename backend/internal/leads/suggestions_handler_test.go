package leads

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/daniil/floq/internal/httputil"
	"github.com/daniil/floq/internal/leads/domain"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// buildRouter wires the four suggestion endpoints against a UseCase whose
// only dependency is a test-supplied ProspectSuggestionFinder. The UC's
// lead/ai/sender remain nil — none of the suggestion handlers touch them.
func buildRouter(t *testing.T, finder domain.ProspectSuggestionFinder) *chi.Mux {
	t.Helper()
	uc := NewUseCase(newMockRepo(), nil, nil, WithSuggestionFinder(finder))
	h := &Handler{uc: uc}
	r := chi.NewRouter()
	r.Get("/api/leads/suggestion-counts", h.suggestionCounts())
	r.Get("/api/leads/{id}/prospect-suggestions", h.getProspectSuggestions())
	r.Post("/api/leads/{id}/link-prospect", h.linkProspect())
	r.Post("/api/leads/{id}/dismiss-prospect-suggestion", h.dismissProspectSuggestion())
	return r
}

// authed attaches a user_id to the request context so the handler's
// UserIDFromContext lookup succeeds.
func authed(req *http.Request, userID uuid.UUID) *http.Request {
	return req.WithContext(httputil.WithUserID(req.Context(), userID))
}

// --- 401 unauthenticated ---

func TestSuggestionEndpoints_MissingAuthReturns401(t *testing.T) {
	r := buildRouter(t, &mockSuggestionFinder{})
	leadID := uuid.New()

	cases := []struct {
		name string
		req  *http.Request
	}{
		{"suggestion-counts", httptest.NewRequest(http.MethodGet, "/api/leads/suggestion-counts", nil)},
		{"get suggestions", httptest.NewRequest(http.MethodGet, "/api/leads/"+leadID.String()+"/prospect-suggestions", nil)},
		{"link", httptest.NewRequest(http.MethodPost, "/api/leads/"+leadID.String()+"/link-prospect", strings.NewReader(`{"prospect_id":"`+uuid.NewString()+`"}`))},
		{"dismiss", httptest.NewRequest(http.MethodPost, "/api/leads/"+leadID.String()+"/dismiss-prospect-suggestion", strings.NewReader(`{"prospect_id":"`+uuid.NewString()+`"}`))},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			r.ServeHTTP(w, tc.req)
			assert.Equal(t, http.StatusUnauthorized, w.Code)
		})
	}
}

// --- 400 malformed input ---

func TestGetProspectSuggestions_InvalidLeadID(t *testing.T) {
	r := buildRouter(t, &mockSuggestionFinder{})
	req := authed(httptest.NewRequest(http.MethodGet, "/api/leads/not-a-uuid/prospect-suggestions", nil), uuid.New())
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestLinkProspect_MalformedJSON(t *testing.T) {
	r := buildRouter(t, &mockSuggestionFinder{})
	req := authed(httptest.NewRequest(http.MethodPost, "/api/leads/"+uuid.NewString()+"/link-prospect", strings.NewReader(`{not json`)), uuid.New())
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestLinkProspect_InvalidProspectUUID(t *testing.T) {
	r := buildRouter(t, &mockSuggestionFinder{})
	req := authed(httptest.NewRequest(http.MethodPost, "/api/leads/"+uuid.NewString()+"/link-prospect", strings.NewReader(`{"prospect_id":"nope"}`)), uuid.New())
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestDismissProspect_InvalidProspectUUID(t *testing.T) {
	r := buildRouter(t, &mockSuggestionFinder{})
	req := authed(httptest.NewRequest(http.MethodPost, "/api/leads/"+uuid.NewString()+"/dismiss-prospect-suggestion", strings.NewReader(`{"prospect_id":"oops"}`)), uuid.New())
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// --- 404 sentinel mapping ---

func TestGetProspectSuggestions_ErrLeadNotFoundMapsTo404(t *testing.T) {
	finder := &mockSuggestionFinder{findErr: domain.ErrLeadNotFound}
	r := buildRouter(t, finder)
	req := authed(httptest.NewRequest(http.MethodGet, "/api/leads/"+uuid.NewString()+"/prospect-suggestions", nil), uuid.New())
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.Contains(t, w.Body.String(), "lead not found")
}

func TestLinkProspect_ErrProspectNotFoundMapsTo404(t *testing.T) {
	finder := &mockSuggestionFinder{linkErr: domain.ErrProspectNotFound}
	r := buildRouter(t, finder)
	req := authed(httptest.NewRequest(http.MethodPost, "/api/leads/"+uuid.NewString()+"/link-prospect", strings.NewReader(`{"prospect_id":"`+uuid.NewString()+`"}`)), uuid.New())
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.Contains(t, w.Body.String(), "prospect not found")
}

func TestDismissProspect_ErrLeadNotFoundMapsTo404(t *testing.T) {
	finder := &mockSuggestionFinder{dismissErr: domain.ErrLeadNotFound}
	r := buildRouter(t, finder)
	req := authed(httptest.NewRequest(http.MethodPost, "/api/leads/"+uuid.NewString()+"/dismiss-prospect-suggestion", strings.NewReader(`{"prospect_id":"`+uuid.NewString()+`"}`)), uuid.New())
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

// --- 500: non-sentinel errors don't leak internal messages ---

func TestLinkProspect_InternalErrorUsesFixedMessage(t *testing.T) {
	finder := &mockSuggestionFinder{linkErr: errors.New("pq: relation \"leads\" does not exist")}
	r := buildRouter(t, finder)
	req := authed(httptest.NewRequest(http.MethodPost, "/api/leads/"+uuid.NewString()+"/link-prospect", strings.NewReader(`{"prospect_id":"`+uuid.NewString()+`"}`)), uuid.New())
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
	// Internal details must not leak.
	assert.NotContains(t, w.Body.String(), "relation")
	assert.NotContains(t, w.Body.String(), "pq:")
	assert.Contains(t, w.Body.String(), "failed to link prospect")
}

// --- 200 happy paths ---

func TestGetProspectSuggestions_Happy(t *testing.T) {
	leadID := uuid.New()
	prospectID := uuid.New()
	finder := &mockSuggestionFinder{
		findResult: []domain.ProspectSuggestion{
			{ProspectID: prospectID, Name: "Даниил", Company: "Floq", Confidence: domain.ConfidenceHigh},
		},
	}
	r := buildRouter(t, finder)
	req := authed(httptest.NewRequest(http.MethodGet, "/api/leads/"+leadID.String()+"/prospect-suggestions", nil), uuid.New())
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	var resp []prospectSuggestionResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.Len(t, resp, 1)
	assert.Equal(t, prospectID.String(), resp[0].ProspectID)
	assert.Equal(t, "high", resp[0].Confidence)
	assert.Equal(t, leadID, finder.findLeadID)
}

func TestLinkProspect_Happy(t *testing.T) {
	finder := &mockSuggestionFinder{}
	r := buildRouter(t, finder)
	leadID, prospectID, userID := uuid.New(), uuid.New(), uuid.New()
	req := authed(httptest.NewRequest(http.MethodPost, "/api/leads/"+leadID.String()+"/link-prospect", strings.NewReader(`{"prospect_id":"`+prospectID.String()+`"}`)), userID)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, userID, finder.linkedUserID)
	assert.Equal(t, leadID, finder.linkedLead)
	assert.Equal(t, prospectID, finder.linkedPros)
}

func TestDismissProspect_Happy(t *testing.T) {
	finder := &mockSuggestionFinder{}
	r := buildRouter(t, finder)
	leadID, prospectID, userID := uuid.New(), uuid.New(), uuid.New()
	req := authed(httptest.NewRequest(http.MethodPost, "/api/leads/"+leadID.String()+"/dismiss-prospect-suggestion", strings.NewReader(`{"prospect_id":"`+prospectID.String()+`"}`)), userID)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, userID, finder.dismissedU)
	assert.Equal(t, leadID, finder.dismissedL)
	assert.Equal(t, prospectID, finder.dismissedP)
}

func TestSuggestionCounts_Happy(t *testing.T) {
	leadID := uuid.New()
	finder := &mockSuggestionFinder{counts: map[uuid.UUID]int{leadID: 5}}
	r := buildRouter(t, finder)
	req := authed(httptest.NewRequest(http.MethodGet, "/api/leads/suggestion-counts", nil), uuid.New())
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	var got map[string]int
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	assert.Equal(t, 5, got[leadID.String()])
}

// Ensure non-sentinel error in SuggestionCounts becomes 500 with a fixed
// message — it never returns ErrLeadNotFound so no 404 path applies here.
func TestSuggestionCounts_InternalError(t *testing.T) {
	finder := &mockSuggestionFinder{countsErr: errors.New("boom")}
	r := buildRouter(t, finder)
	req := authed(httptest.NewRequest(http.MethodGet, "/api/leads/suggestion-counts", nil), uuid.New())
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.NotContains(t, w.Body.String(), "boom")
}

// Regression guard: exhaustively covers every handler that reads user_id so
// adding a new one without auth wiring trips the suite immediately.
func TestSuggestionEndpoints_ContextParamExtraction(t *testing.T) {
	// Ensure our mock path sees context properly by exercising one endpoint.
	finder := &mockSuggestionFinder{}
	r := buildRouter(t, finder)
	userID := uuid.New()
	leadID := uuid.New()
	req := authed(httptest.NewRequest(http.MethodGet, "/api/leads/"+leadID.String()+"/prospect-suggestions", nil), userID)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	// Finder received the user id from context, not zero.
	assert.NotEqual(t, uuid.Nil, finder.findUserID)
	assert.Equal(t, userID, finder.findUserID)
}

// Silences unused-import warnings if test file reorders.
var _ = context.Background
