package sequences

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/daniil/floq/internal/httputil"
	"github.com/daniil/floq/internal/sequences/domain"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// helper: create a router with all routes and an authenticated context middleware
func setupRouter(uc *UseCase, userID uuid.UUID) chi.Router {
	r := chi.NewRouter()
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			ctx := httputil.WithUserID(req.Context(), userID)
			next.ServeHTTP(w, req.WithContext(ctx))
		})
	})
	RegisterRoutes(r, uc)
	RegisterPublicRoutes(r, uc)
	return r
}

func setupRouterNoAuth(uc *UseCase) chi.Router {
	r := chi.NewRouter()
	RegisterRoutes(r, uc)
	RegisterPublicRoutes(r, uc)
	return r
}

func doRequest(router chi.Router, method, path string, body interface{}) *httptest.ResponseRecorder {
	var buf bytes.Buffer
	if body != nil {
		json.NewEncoder(&buf).Encode(body)
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	return rr
}

// --- ListSequences Handler ---

func TestHandler_ListSequences(t *testing.T) {
	userID := uuid.New()
	seqID := uuid.New()
	repo := &mockRepo{sequences: []domain.Sequence{{ID: seqID, UserID: userID, Name: "Seq1"}}}
	uc := NewUseCase(repo, &mockAI{}, newMockProspectReader(), &mockLeadCreator{})
	router := setupRouter(uc, userID)

	rr := doRequest(router, http.MethodGet, "/api/sequences", nil)
	assert.Equal(t, http.StatusOK, rr.Code)

	var resp []SequenceResponse
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	require.Len(t, resp, 1)
	assert.Equal(t, "Seq1", resp[0].Name)
}

func TestHandler_ListSequences_Unauthorized(t *testing.T) {
	uc := NewUseCase(&mockRepo{}, &mockAI{}, newMockProspectReader(), &mockLeadCreator{})
	router := setupRouterNoAuth(uc)

	rr := doRequest(router, http.MethodGet, "/api/sequences", nil)
	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestHandler_ListSequences_Error(t *testing.T) {
	userID := uuid.New()
	repo := &mockRepo{seqErr: errors.New("db error")}
	uc := NewUseCase(repo, &mockAI{}, newMockProspectReader(), &mockLeadCreator{})
	router := setupRouter(uc, userID)

	rr := doRequest(router, http.MethodGet, "/api/sequences", nil)
	assert.Equal(t, http.StatusInternalServerError, rr.Code)
}

// --- CreateSequence Handler ---

func TestHandler_CreateSequence(t *testing.T) {
	userID := uuid.New()
	uc := NewUseCase(&mockRepo{}, &mockAI{}, newMockProspectReader(), &mockLeadCreator{})
	router := setupRouter(uc, userID)

	rr := doRequest(router, http.MethodPost, "/api/sequences", map[string]string{"name": "New Seq"})
	assert.Equal(t, http.StatusCreated, rr.Code)

	var resp SequenceResponse
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	assert.Equal(t, "New Seq", resp.Name)
}

func TestHandler_CreateSequence_EmptyName(t *testing.T) {
	userID := uuid.New()
	uc := NewUseCase(&mockRepo{}, &mockAI{}, newMockProspectReader(), &mockLeadCreator{})
	router := setupRouter(uc, userID)

	rr := doRequest(router, http.MethodPost, "/api/sequences", map[string]string{"name": ""})
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestHandler_CreateSequence_Unauthorized(t *testing.T) {
	uc := NewUseCase(&mockRepo{}, &mockAI{}, newMockProspectReader(), &mockLeadCreator{})
	router := setupRouterNoAuth(uc)

	rr := doRequest(router, http.MethodPost, "/api/sequences", map[string]string{"name": "Test"})
	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestHandler_CreateSequence_InvalidBody(t *testing.T) {
	userID := uuid.New()
	uc := NewUseCase(&mockRepo{}, &mockAI{}, newMockProspectReader(), &mockLeadCreator{})
	router := setupRouter(uc, userID)

	req := httptest.NewRequest(http.MethodPost, "/api/sequences", bytes.NewBufferString("not json"))
	req.Header.Set("Content-Type", "application/json")
	ctx := httputil.WithUserID(req.Context(), userID)
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

// --- GetSequence Handler ---

func TestHandler_GetSequence(t *testing.T) {
	seqID := uuid.New()
	repo := &mockRepo{
		sequences: []domain.Sequence{{ID: seqID, Name: "Test"}},
		steps:     []domain.SequenceStep{{ID: uuid.New(), SequenceID: seqID, StepOrder: 1, Channel: domain.StepChannelEmail}},
	}
	uc := NewUseCase(repo, &mockAI{}, newMockProspectReader(), &mockLeadCreator{})
	router := setupRouter(uc, uuid.New())

	rr := doRequest(router, http.MethodGet, "/api/sequences/"+seqID.String(), nil)
	assert.Equal(t, http.StatusOK, rr.Code)

	var resp SequenceDetailResponse
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	assert.Equal(t, "Test", resp.Sequence.Name)
	assert.Len(t, resp.Steps, 1)
}

func TestHandler_GetSequence_NotFound(t *testing.T) {
	repo := &mockRepo{}
	uc := NewUseCase(repo, &mockAI{}, newMockProspectReader(), &mockLeadCreator{})
	router := setupRouter(uc, uuid.New())

	rr := doRequest(router, http.MethodGet, "/api/sequences/"+uuid.New().String(), nil)
	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestHandler_GetSequence_InvalidID(t *testing.T) {
	uc := NewUseCase(&mockRepo{}, &mockAI{}, newMockProspectReader(), &mockLeadCreator{})
	router := setupRouter(uc, uuid.New())

	rr := doRequest(router, http.MethodGet, "/api/sequences/not-a-uuid", nil)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

// --- UpdateSequence Handler ---

func TestHandler_UpdateSequence(t *testing.T) {
	seqID := uuid.New()
	repo := &mockRepo{sequences: []domain.Sequence{{ID: seqID, Name: "Old"}}}
	uc := NewUseCase(repo, &mockAI{}, newMockProspectReader(), &mockLeadCreator{})
	router := setupRouter(uc, uuid.New())

	rr := doRequest(router, http.MethodPut, "/api/sequences/"+seqID.String(), map[string]string{"name": "Updated"})
	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestHandler_UpdateSequence_EmptyName(t *testing.T) {
	seqID := uuid.New()
	uc := NewUseCase(&mockRepo{}, &mockAI{}, newMockProspectReader(), &mockLeadCreator{})
	router := setupRouter(uc, uuid.New())

	rr := doRequest(router, http.MethodPut, "/api/sequences/"+seqID.String(), map[string]string{"name": ""})
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestHandler_UpdateSequence_InvalidID(t *testing.T) {
	uc := NewUseCase(&mockRepo{}, &mockAI{}, newMockProspectReader(), &mockLeadCreator{})
	router := setupRouter(uc, uuid.New())

	rr := doRequest(router, http.MethodPut, "/api/sequences/bad-id", map[string]string{"name": "Test"})
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

// --- DeleteSequence Handler ---

func TestHandler_DeleteSequence(t *testing.T) {
	seqID := uuid.New()
	uc := NewUseCase(&mockRepo{}, &mockAI{}, newMockProspectReader(), &mockLeadCreator{})
	router := setupRouter(uc, uuid.New())

	rr := doRequest(router, http.MethodDelete, "/api/sequences/"+seqID.String(), nil)
	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestHandler_DeleteSequence_InvalidID(t *testing.T) {
	uc := NewUseCase(&mockRepo{}, &mockAI{}, newMockProspectReader(), &mockLeadCreator{})
	router := setupRouter(uc, uuid.New())

	rr := doRequest(router, http.MethodDelete, "/api/sequences/bad-id", nil)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

// --- AddStep Handler ---

func TestHandler_AddStep(t *testing.T) {
	seqID := uuid.New()
	uc := NewUseCase(&mockRepo{}, &mockAI{}, newMockProspectReader(), &mockLeadCreator{})
	router := setupRouter(uc, uuid.New())

	body := map[string]interface{}{
		"step_order":  1,
		"delay_days":  0,
		"channel":     "email",
		"prompt_hint": "intro",
	}
	rr := doRequest(router, http.MethodPost, "/api/sequences/"+seqID.String()+"/steps", body)
	assert.Equal(t, http.StatusCreated, rr.Code)
}

func TestHandler_AddStep_DefaultChannel(t *testing.T) {
	seqID := uuid.New()
	uc := NewUseCase(&mockRepo{}, &mockAI{}, newMockProspectReader(), &mockLeadCreator{})
	router := setupRouter(uc, uuid.New())

	body := map[string]interface{}{
		"step_order":  1,
		"delay_days":  0,
		"prompt_hint": "intro",
	}
	rr := doRequest(router, http.MethodPost, "/api/sequences/"+seqID.String()+"/steps", body)
	assert.Equal(t, http.StatusCreated, rr.Code)

	var resp SequenceStepResponse
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	assert.Equal(t, "email", resp.Channel)
}

func TestHandler_AddStep_InvalidID(t *testing.T) {
	uc := NewUseCase(&mockRepo{}, &mockAI{}, newMockProspectReader(), &mockLeadCreator{})
	router := setupRouter(uc, uuid.New())

	rr := doRequest(router, http.MethodPost, "/api/sequences/bad-id/steps", map[string]interface{}{"step_order": 1})
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

// --- DeleteStep Handler ---

func TestHandler_DeleteStep(t *testing.T) {
	seqID := uuid.New()
	stepID := uuid.New()
	uc := NewUseCase(&mockRepo{}, &mockAI{}, newMockProspectReader(), &mockLeadCreator{})
	router := setupRouter(uc, uuid.New())

	rr := doRequest(router, http.MethodDelete, "/api/sequences/"+seqID.String()+"/steps/"+stepID.String(), nil)
	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestHandler_DeleteStep_InvalidStepID(t *testing.T) {
	seqID := uuid.New()
	uc := NewUseCase(&mockRepo{}, &mockAI{}, newMockProspectReader(), &mockLeadCreator{})
	router := setupRouter(uc, uuid.New())

	rr := doRequest(router, http.MethodDelete, "/api/sequences/"+seqID.String()+"/steps/bad-id", nil)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

// --- LaunchSequence Handler ---

func TestHandler_LaunchSequence(t *testing.T) {
	seqID := uuid.New()
	pid := uuid.New()

	repo := &mockRepo{
		steps: []domain.SequenceStep{
			{ID: uuid.New(), SequenceID: seqID, StepOrder: 1, DelayDays: 0, Channel: domain.StepChannelEmail},
		},
	}
	pr := newMockProspectReader()
	pr.prospects[pid] = &domain.ProspectView{
		ID: pid, UserID: uuid.New(), Name: "Alice", Status: "new", VerifyStatus: "valid",
	}

	uc := NewUseCase(repo, &mockAI{coldBody: "hi"}, pr, &mockLeadCreator{})
	router := setupRouter(uc, uuid.New())

	body := map[string]interface{}{
		"prospect_ids": []string{pid.String()},
		"send_now":     false,
	}
	rr := doRequest(router, http.MethodPost, "/api/sequences/"+seqID.String()+"/launch", body)
	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestHandler_LaunchSequence_EmptyProspects(t *testing.T) {
	seqID := uuid.New()
	uc := NewUseCase(&mockRepo{}, &mockAI{}, newMockProspectReader(), &mockLeadCreator{})
	router := setupRouter(uc, uuid.New())

	body := map[string]interface{}{
		"prospect_ids": []string{},
	}
	rr := doRequest(router, http.MethodPost, "/api/sequences/"+seqID.String()+"/launch", body)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestHandler_LaunchSequence_InvalidID(t *testing.T) {
	uc := NewUseCase(&mockRepo{}, &mockAI{}, newMockProspectReader(), &mockLeadCreator{})
	router := setupRouter(uc, uuid.New())

	rr := doRequest(router, http.MethodPost, "/api/sequences/bad-id/launch", map[string]interface{}{"prospect_ids": []string{uuid.New().String()}})
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

// --- ToggleActive Handler ---

func TestHandler_ToggleActive(t *testing.T) {
	seqID := uuid.New()
	uc := NewUseCase(&mockRepo{}, &mockAI{}, newMockProspectReader(), &mockLeadCreator{})
	router := setupRouter(uc, uuid.New())

	rr := doRequest(router, http.MethodPatch, "/api/sequences/"+seqID.String()+"/toggle", map[string]bool{"is_active": true})
	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestHandler_ToggleActive_InvalidID(t *testing.T) {
	uc := NewUseCase(&mockRepo{}, &mockAI{}, newMockProspectReader(), &mockLeadCreator{})
	router := setupRouter(uc, uuid.New())

	rr := doRequest(router, http.MethodPatch, "/api/sequences/bad-id/toggle", map[string]bool{"is_active": true})
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

// --- GetQueue Handler ---

func TestHandler_GetQueue(t *testing.T) {
	userID := uuid.New()
	msgs := []domain.OutboundMessage{{ID: uuid.New(), Body: "msg1", Channel: domain.StepChannelEmail, Status: domain.OutboundStatusDraft}}
	repo := &mockRepo{queue: msgs}
	uc := NewUseCase(repo, &mockAI{}, newMockProspectReader(), &mockLeadCreator{})
	router := setupRouter(uc, userID)

	rr := doRequest(router, http.MethodGet, "/api/outbound/queue", nil)
	assert.Equal(t, http.StatusOK, rr.Code)

	var resp []OutboundMessageResponse
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	assert.Len(t, resp, 1)
}

func TestHandler_GetQueue_Unauthorized(t *testing.T) {
	uc := NewUseCase(&mockRepo{}, &mockAI{}, newMockProspectReader(), &mockLeadCreator{})
	router := setupRouterNoAuth(uc)

	rr := doRequest(router, http.MethodGet, "/api/outbound/queue", nil)
	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

// --- GetSent Handler ---

func TestHandler_GetSent(t *testing.T) {
	userID := uuid.New()
	msgs := []domain.OutboundMessage{{ID: uuid.New(), Body: "sent1", Channel: domain.StepChannelEmail, Status: domain.OutboundStatusSent}}
	repo := &mockRepo{sent: msgs}
	uc := NewUseCase(repo, &mockAI{}, newMockProspectReader(), &mockLeadCreator{})
	router := setupRouter(uc, userID)

	rr := doRequest(router, http.MethodGet, "/api/outbound/sent", nil)
	assert.Equal(t, http.StatusOK, rr.Code)

	var resp []OutboundMessageResponse
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	assert.Len(t, resp, 1)
}

func TestHandler_GetSent_Unauthorized(t *testing.T) {
	uc := NewUseCase(&mockRepo{}, &mockAI{}, newMockProspectReader(), &mockLeadCreator{})
	router := setupRouterNoAuth(uc)

	rr := doRequest(router, http.MethodGet, "/api/outbound/sent", nil)
	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

// --- ApproveMessage Handler ---

func TestHandler_ApproveMessage(t *testing.T) {
	msgID := uuid.New()
	uc := NewUseCase(&mockRepo{}, &mockAI{}, newMockProspectReader(), &mockLeadCreator{})
	router := setupRouter(uc, uuid.New())

	rr := doRequest(router, http.MethodPost, "/api/outbound/"+msgID.String()+"/approve", nil)
	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestHandler_ApproveMessage_InvalidID(t *testing.T) {
	uc := NewUseCase(&mockRepo{}, &mockAI{}, newMockProspectReader(), &mockLeadCreator{})
	router := setupRouter(uc, uuid.New())

	rr := doRequest(router, http.MethodPost, "/api/outbound/bad-id/approve", nil)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

// --- RejectMessage Handler ---

func TestHandler_RejectMessage(t *testing.T) {
	msgID := uuid.New()
	uc := NewUseCase(&mockRepo{}, &mockAI{}, newMockProspectReader(), &mockLeadCreator{})
	router := setupRouter(uc, uuid.New())

	rr := doRequest(router, http.MethodPost, "/api/outbound/"+msgID.String()+"/reject", nil)
	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestHandler_RejectMessage_InvalidID(t *testing.T) {
	uc := NewUseCase(&mockRepo{}, &mockAI{}, newMockProspectReader(), &mockLeadCreator{})
	router := setupRouter(uc, uuid.New())

	rr := doRequest(router, http.MethodPost, "/api/outbound/bad-id/reject", nil)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

// --- EditMessage Handler ---

func TestHandler_EditMessage(t *testing.T) {
	msgID := uuid.New()
	repo := &mockRepo{
		messages: []*domain.OutboundMessage{
			{ID: msgID, Body: "original", Channel: domain.StepChannelEmail},
		},
	}
	uc := NewUseCase(repo, &mockAI{}, newMockProspectReader(), &mockLeadCreator{})
	router := setupRouter(uc, uuid.New())

	rr := doRequest(router, http.MethodPost, "/api/outbound/"+msgID.String()+"/edit", map[string]string{"body": "new body"})
	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestHandler_EditMessage_EmptyBody(t *testing.T) {
	msgID := uuid.New()
	uc := NewUseCase(&mockRepo{}, &mockAI{}, newMockProspectReader(), &mockLeadCreator{})
	router := setupRouter(uc, uuid.New())

	rr := doRequest(router, http.MethodPost, "/api/outbound/"+msgID.String()+"/edit", map[string]string{"body": ""})
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestHandler_EditMessage_InvalidID(t *testing.T) {
	uc := NewUseCase(&mockRepo{}, &mockAI{}, newMockProspectReader(), &mockLeadCreator{})
	router := setupRouter(uc, uuid.New())

	rr := doRequest(router, http.MethodPost, "/api/outbound/bad-id/edit", map[string]string{"body": "test"})
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

// --- GetStats Handler ---

func TestHandler_GetStats(t *testing.T) {
	userID := uuid.New()
	stats := &domain.Stats{Draft: 5, Approved: 3, Sent: 10, Opened: 2, Replied: 1, Bounced: 0}
	repo := &mockRepo{statsVal: stats}
	uc := NewUseCase(repo, &mockAI{}, newMockProspectReader(), &mockLeadCreator{})
	router := setupRouter(uc, userID)

	rr := doRequest(router, http.MethodGet, "/api/outbound/stats", nil)
	assert.Equal(t, http.StatusOK, rr.Code)

	var resp StatsResponse
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	assert.Equal(t, 5, resp.Draft)
	assert.Equal(t, 10, resp.Sent)
}

func TestHandler_GetStats_Unauthorized(t *testing.T) {
	uc := NewUseCase(&mockRepo{}, &mockAI{}, newMockProspectReader(), &mockLeadCreator{})
	router := setupRouterNoAuth(uc)

	rr := doRequest(router, http.MethodGet, "/api/outbound/stats", nil)
	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestHandler_GetStats_Error(t *testing.T) {
	userID := uuid.New()
	repo := &mockRepo{statsErr: errors.New("db error")}
	uc := NewUseCase(repo, &mockAI{}, newMockProspectReader(), &mockLeadCreator{})
	router := setupRouter(uc, userID)

	rr := doRequest(router, http.MethodGet, "/api/outbound/stats", nil)
	assert.Equal(t, http.StatusInternalServerError, rr.Code)
}

// --- TrackOpen Handler ---

func TestHandler_TrackOpen(t *testing.T) {
	msgID := uuid.New()
	uc := NewUseCase(&mockRepo{}, &mockAI{}, newMockProspectReader(), &mockLeadCreator{})
	router := setupRouter(uc, uuid.New())

	rr := doRequest(router, http.MethodGet, "/api/track/open/"+msgID.String(), nil)
	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "image/gif", rr.Header().Get("Content-Type"))
	assert.Equal(t, "no-cache, no-store, must-revalidate", rr.Header().Get("Cache-Control"))
}

func TestHandler_TrackOpen_InvalidID(t *testing.T) {
	uc := NewUseCase(&mockRepo{}, &mockAI{}, newMockProspectReader(), &mockLeadCreator{})
	router := setupRouter(uc, uuid.New())

	rr := doRequest(router, http.MethodGet, "/api/track/open/bad-id", nil)
	assert.Equal(t, http.StatusNotFound, rr.Code)
}

// --- PreviewMessage Handler ---

func TestHandler_PreviewMessage_Email(t *testing.T) {
	ai := &mockAI{coldBody: "Preview email text"}
	uc := NewUseCase(&mockRepo{}, ai, newMockProspectReader(), &mockLeadCreator{})
	router := setupRouter(uc, uuid.New())

	body := map[string]string{
		"name":    "Alice",
		"company": "Acme",
		"context": "CEO",
		"channel": "email",
		"hint":    "intro",
	}
	rr := doRequest(router, http.MethodPost, "/api/sequences/preview", body)
	assert.Equal(t, http.StatusOK, rr.Code)

	var resp map[string]string
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	assert.Equal(t, "Preview email text", resp["text"])
}

func TestHandler_PreviewMessage_Telegram(t *testing.T) {
	ai := &mockAI{telegramBody: "Preview tg text"}
	uc := NewUseCase(&mockRepo{}, ai, newMockProspectReader(), &mockLeadCreator{})
	router := setupRouter(uc, uuid.New())

	body := map[string]string{
		"name":    "Alice",
		"channel": "telegram",
	}
	rr := doRequest(router, http.MethodPost, "/api/sequences/preview", body)
	assert.Equal(t, http.StatusOK, rr.Code)

	var resp map[string]string
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	assert.Equal(t, "Preview tg text", resp["text"])
}

func TestHandler_PreviewMessage_MissingName(t *testing.T) {
	uc := NewUseCase(&mockRepo{}, &mockAI{}, newMockProspectReader(), &mockLeadCreator{})
	router := setupRouter(uc, uuid.New())

	rr := doRequest(router, http.MethodPost, "/api/sequences/preview", map[string]string{"company": "Acme"})
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestHandler_PreviewMessage_Defaults(t *testing.T) {
	ai := &mockAI{coldBody: "default preview"}
	uc := NewUseCase(&mockRepo{}, ai, newMockProspectReader(), &mockLeadCreator{})
	router := setupRouter(uc, uuid.New())

	// No channel, no hint — should default to email and "первое касание"
	body := map[string]string{"name": "Alice"}
	rr := doRequest(router, http.MethodPost, "/api/sequences/preview", body)
	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestHandler_PreviewMessage_AIError(t *testing.T) {
	ai := &mockAI{err: errors.New("api error")}
	uc := NewUseCase(&mockRepo{}, ai, newMockProspectReader(), &mockLeadCreator{})
	router := setupRouter(uc, uuid.New())

	body := map[string]string{"name": "Alice"}
	rr := doRequest(router, http.MethodPost, "/api/sequences/preview", body)
	assert.Equal(t, http.StatusInternalServerError, rr.Code)
}

// --- ListSequences returns empty array, not null ---

func TestHandler_ListSequences_EmptyReturnsArray(t *testing.T) {
	userID := uuid.New()
	repo := &mockRepo{} // sequences is nil
	uc := NewUseCase(repo, &mockAI{}, newMockProspectReader(), &mockLeadCreator{})
	router := setupRouter(uc, userID)

	rr := doRequest(router, http.MethodGet, "/api/sequences", nil)
	assert.Equal(t, http.StatusOK, rr.Code)

	var resp []SequenceResponse
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	assert.NotNil(t, resp)
	assert.Len(t, resp, 0)
}

// --- context helper test ---

func TestWithUserID_RoundTrip(t *testing.T) {
	userID := uuid.New()
	ctx := httputil.WithUserID(context.Background(), userID)
	got, ok := httputil.UserIDFromContext(ctx)
	require.True(t, ok)
	assert.Equal(t, userID, got)
}

// --- Additional error path tests for handler coverage ---

func TestHandler_CreateSequence_RepoError(t *testing.T) {
	userID := uuid.New()
	repo := &mockRepo{createSeqErr: errors.New("db error")}
	uc := NewUseCase(repo, &mockAI{}, newMockProspectReader(), &mockLeadCreator{})
	router := setupRouter(uc, userID)

	rr := doRequest(router, http.MethodPost, "/api/sequences", map[string]string{"name": "Test"})
	assert.Equal(t, http.StatusInternalServerError, rr.Code)
}

func TestHandler_GetSequence_StepsError(t *testing.T) {
	seqID := uuid.New()
	repo := &mockRepo{
		sequences: []domain.Sequence{{ID: seqID, Name: "Test"}},
		stepErr:   errors.New("db error"),
	}
	uc := NewUseCase(repo, &mockAI{}, newMockProspectReader(), &mockLeadCreator{})
	router := setupRouter(uc, uuid.New())

	rr := doRequest(router, http.MethodGet, "/api/sequences/"+seqID.String(), nil)
	assert.Equal(t, http.StatusInternalServerError, rr.Code)
}

func TestHandler_GetSequence_RepoError(t *testing.T) {
	repo := &mockRepo{seqErr: errors.New("db error")}
	uc := NewUseCase(repo, &mockAI{}, newMockProspectReader(), &mockLeadCreator{})
	router := setupRouter(uc, uuid.New())

	rr := doRequest(router, http.MethodGet, "/api/sequences/"+uuid.New().String(), nil)
	assert.Equal(t, http.StatusInternalServerError, rr.Code)
}

func TestHandler_UpdateSequence_RepoError(t *testing.T) {
	seqID := uuid.New()
	repo := &mockRepo{sequences: []domain.Sequence{{ID: seqID, Name: "Old"}}, updateSeqErr: errors.New("db error")}
	uc := NewUseCase(repo, &mockAI{}, newMockProspectReader(), &mockLeadCreator{})
	router := setupRouter(uc, uuid.New())

	rr := doRequest(router, http.MethodPut, "/api/sequences/"+seqID.String(), map[string]string{"name": "Updated"})
	assert.Equal(t, http.StatusInternalServerError, rr.Code)
}

func TestHandler_UpdateSequence_InvalidBody(t *testing.T) {
	seqID := uuid.New()
	uc := NewUseCase(&mockRepo{}, &mockAI{}, newMockProspectReader(), &mockLeadCreator{})
	r := setupRouter(uc, uuid.New())

	req := httptest.NewRequest(http.MethodPut, "/api/sequences/"+seqID.String(), bytes.NewBufferString("not json"))
	ctx := httputil.WithUserID(req.Context(), uuid.New())
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestHandler_DeleteSequence_RepoError(t *testing.T) {
	seqID := uuid.New()
	repo := &mockRepo{deleteSeqErr: errors.New("db error")}
	uc := NewUseCase(repo, &mockAI{}, newMockProspectReader(), &mockLeadCreator{})
	router := setupRouter(uc, uuid.New())

	rr := doRequest(router, http.MethodDelete, "/api/sequences/"+seqID.String(), nil)
	assert.Equal(t, http.StatusInternalServerError, rr.Code)
}

func TestHandler_AddStep_RepoError(t *testing.T) {
	seqID := uuid.New()
	repo := &mockRepo{createStepErr: errors.New("db error")}
	uc := NewUseCase(repo, &mockAI{}, newMockProspectReader(), &mockLeadCreator{})
	router := setupRouter(uc, uuid.New())

	body := map[string]interface{}{"step_order": 1, "delay_days": 0, "channel": "email"}
	rr := doRequest(router, http.MethodPost, "/api/sequences/"+seqID.String()+"/steps", body)
	assert.Equal(t, http.StatusInternalServerError, rr.Code)
}

func TestHandler_AddStep_InvalidBody(t *testing.T) {
	seqID := uuid.New()
	uc := NewUseCase(&mockRepo{}, &mockAI{}, newMockProspectReader(), &mockLeadCreator{})
	r := setupRouter(uc, uuid.New())

	req := httptest.NewRequest(http.MethodPost, "/api/sequences/"+seqID.String()+"/steps", bytes.NewBufferString("not json"))
	ctx := httputil.WithUserID(req.Context(), uuid.New())
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestHandler_DeleteStep_RepoError(t *testing.T) {
	seqID := uuid.New()
	stepID := uuid.New()
	repo := &mockRepo{deleteStepErr: errors.New("db error")}
	uc := NewUseCase(repo, &mockAI{}, newMockProspectReader(), &mockLeadCreator{})
	router := setupRouter(uc, uuid.New())

	rr := doRequest(router, http.MethodDelete, "/api/sequences/"+seqID.String()+"/steps/"+stepID.String(), nil)
	assert.Equal(t, http.StatusInternalServerError, rr.Code)
}

func TestHandler_LaunchSequence_RepoError(t *testing.T) {
	seqID := uuid.New()
	repo := &mockRepo{stepErr: errors.New("db error")}
	uc := NewUseCase(repo, &mockAI{}, newMockProspectReader(), &mockLeadCreator{})
	router := setupRouter(uc, uuid.New())

	body := map[string]interface{}{"prospect_ids": []string{uuid.New().String()}}
	rr := doRequest(router, http.MethodPost, "/api/sequences/"+seqID.String()+"/launch", body)
	assert.Equal(t, http.StatusInternalServerError, rr.Code)
}

func TestHandler_LaunchSequence_InvalidBody(t *testing.T) {
	seqID := uuid.New()
	uc := NewUseCase(&mockRepo{}, &mockAI{}, newMockProspectReader(), &mockLeadCreator{})
	r := setupRouter(uc, uuid.New())

	req := httptest.NewRequest(http.MethodPost, "/api/sequences/"+seqID.String()+"/launch", bytes.NewBufferString("not json"))
	ctx := httputil.WithUserID(req.Context(), uuid.New())
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestHandler_ToggleActive_RepoError(t *testing.T) {
	seqID := uuid.New()
	repo := &mockRepo{toggleErr: errors.New("db error")}
	uc := NewUseCase(repo, &mockAI{}, newMockProspectReader(), &mockLeadCreator{})
	router := setupRouter(uc, uuid.New())

	rr := doRequest(router, http.MethodPatch, "/api/sequences/"+seqID.String()+"/toggle", map[string]bool{"is_active": true})
	assert.Equal(t, http.StatusInternalServerError, rr.Code)
}

func TestHandler_ToggleActive_InvalidBody(t *testing.T) {
	seqID := uuid.New()
	uc := NewUseCase(&mockRepo{}, &mockAI{}, newMockProspectReader(), &mockLeadCreator{})
	r := setupRouter(uc, uuid.New())

	req := httptest.NewRequest(http.MethodPatch, "/api/sequences/"+seqID.String()+"/toggle", bytes.NewBufferString("not json"))
	ctx := httputil.WithUserID(req.Context(), uuid.New())
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestHandler_GetQueue_RepoError(t *testing.T) {
	userID := uuid.New()
	repo := &mockRepo{queueErr: errors.New("db error")}
	uc := NewUseCase(repo, &mockAI{}, newMockProspectReader(), &mockLeadCreator{})
	router := setupRouter(uc, userID)

	rr := doRequest(router, http.MethodGet, "/api/outbound/queue", nil)
	assert.Equal(t, http.StatusInternalServerError, rr.Code)
}

func TestHandler_GetSent_RepoError(t *testing.T) {
	userID := uuid.New()
	repo := &mockRepo{sentErr: errors.New("db error")}
	uc := NewUseCase(repo, &mockAI{}, newMockProspectReader(), &mockLeadCreator{})
	router := setupRouter(uc, userID)

	rr := doRequest(router, http.MethodGet, "/api/outbound/sent", nil)
	assert.Equal(t, http.StatusInternalServerError, rr.Code)
}

func TestHandler_ApproveMessage_RepoError(t *testing.T) {
	msgID := uuid.New()
	repo := &mockRepo{statusUpdateErr: errors.New("db error")}
	uc := NewUseCase(repo, &mockAI{}, newMockProspectReader(), &mockLeadCreator{})
	router := setupRouter(uc, uuid.New())

	rr := doRequest(router, http.MethodPost, "/api/outbound/"+msgID.String()+"/approve", nil)
	assert.Equal(t, http.StatusInternalServerError, rr.Code)
}

func TestHandler_RejectMessage_RepoError(t *testing.T) {
	msgID := uuid.New()
	repo := &mockRepo{statusUpdateErr: errors.New("db error")}
	uc := NewUseCase(repo, &mockAI{}, newMockProspectReader(), &mockLeadCreator{})
	router := setupRouter(uc, uuid.New())

	rr := doRequest(router, http.MethodPost, "/api/outbound/"+msgID.String()+"/reject", nil)
	assert.Equal(t, http.StatusInternalServerError, rr.Code)
}

func TestHandler_EditMessage_RepoError(t *testing.T) {
	msgID := uuid.New()
	// Message not found = error
	repo := &mockRepo{}
	uc := NewUseCase(repo, &mockAI{}, newMockProspectReader(), &mockLeadCreator{})
	router := setupRouter(uc, uuid.New())

	rr := doRequest(router, http.MethodPost, "/api/outbound/"+msgID.String()+"/edit", map[string]string{"body": "new"})
	assert.Equal(t, http.StatusInternalServerError, rr.Code)
}

func TestHandler_EditMessage_InvalidBody(t *testing.T) {
	msgID := uuid.New()
	uc := NewUseCase(&mockRepo{}, &mockAI{}, newMockProspectReader(), &mockLeadCreator{})
	r := setupRouter(uc, uuid.New())

	req := httptest.NewRequest(http.MethodPost, "/api/outbound/"+msgID.String()+"/edit", bytes.NewBufferString("not json"))
	ctx := httputil.WithUserID(req.Context(), uuid.New())
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestHandler_PreviewMessage_InvalidBody(t *testing.T) {
	uc := NewUseCase(&mockRepo{}, &mockAI{}, newMockProspectReader(), &mockLeadCreator{})
	r := setupRouter(uc, uuid.New())

	req := httptest.NewRequest(http.MethodPost, "/api/sequences/preview", bytes.NewBufferString("not json"))
	ctx := httputil.WithUserID(req.Context(), uuid.New())
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

// --- Nil queue/sent returns empty array ---

func TestHandler_GetQueue_NilReturnsArray(t *testing.T) {
	userID := uuid.New()
	repo := &mockRepo{} // queue is nil
	uc := NewUseCase(repo, &mockAI{}, newMockProspectReader(), &mockLeadCreator{})
	router := setupRouter(uc, userID)

	rr := doRequest(router, http.MethodGet, "/api/outbound/queue", nil)
	assert.Equal(t, http.StatusOK, rr.Code)

	var resp []OutboundMessageResponse
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	assert.NotNil(t, resp)
	assert.Len(t, resp, 0)
}

func TestHandler_GetSent_NilReturnsArray(t *testing.T) {
	userID := uuid.New()
	repo := &mockRepo{} // sent is nil
	uc := NewUseCase(repo, &mockAI{}, newMockProspectReader(), &mockLeadCreator{})
	router := setupRouter(uc, userID)

	rr := doRequest(router, http.MethodGet, "/api/outbound/sent", nil)
	assert.Equal(t, http.StatusOK, rr.Code)

	var resp []OutboundMessageResponse
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	assert.NotNil(t, resp)
	assert.Len(t, resp, 0)
}

// --- GetSequence nil steps returns empty array ---

func TestHandler_GetSequence_NilStepsReturnsArray(t *testing.T) {
	seqID := uuid.New()
	repo := &mockRepo{
		sequences: []domain.Sequence{{ID: seqID, Name: "Test"}},
		// steps is nil
	}
	uc := NewUseCase(repo, &mockAI{}, newMockProspectReader(), &mockLeadCreator{})
	router := setupRouter(uc, uuid.New())

	rr := doRequest(router, http.MethodGet, "/api/sequences/"+seqID.String(), nil)
	assert.Equal(t, http.StatusOK, rr.Code)

	var resp SequenceDetailResponse
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	assert.NotNil(t, resp.Steps)
	assert.Len(t, resp.Steps, 0)
}
