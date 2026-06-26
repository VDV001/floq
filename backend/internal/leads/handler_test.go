package leads

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/daniil/floq/internal/httputil"
	"github.com/daniil/floq/internal/leads/domain"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockRepoErr is a mock repository that returns errors for specified methods.
type mockRepoErr struct {
	mockRepo
	listLeadsErr       error
	getLeadErr         error
	getLeadForUserErr  error
	listMessagesErr    error
	createMessageErr   error
	getQualErr         error
	getLatestDraftErr  error
	qualifyErr         error
}

func (m *mockRepoErr) GetLeadForUser(_ context.Context, userID, leadID uuid.UUID) (*domain.Lead, error) {
	if m.getLeadForUserErr != nil {
		return nil, m.getLeadForUserErr
	}
	return m.mockRepo.GetLeadForUser(context.Background(), userID, leadID)
}

func (m *mockRepoErr) ListAllLeads(_ context.Context, _ uuid.UUID) ([]domain.LeadWithSource, error) {
	if m.listLeadsErr != nil {
		return nil, m.listLeadsErr
	}
	return m.mockRepo.ListAllLeads(context.Background(), uuid.Nil)
}

func (m *mockRepoErr) ListLeads(_ context.Context, _ uuid.UUID) ([]domain.LeadWithSource, error) {
	if m.listLeadsErr != nil {
		return nil, m.listLeadsErr
	}
	return m.mockRepo.ListLeads(context.Background(), uuid.Nil)
}

func (m *mockRepoErr) ListArchivedLeads(_ context.Context, _ uuid.UUID) ([]domain.LeadWithSource, error) {
	if m.listLeadsErr != nil {
		return nil, m.listLeadsErr
	}
	return m.mockRepo.ListArchivedLeads(context.Background(), uuid.Nil)
}

func (m *mockRepoErr) GetLead(_ context.Context, id uuid.UUID) (*domain.Lead, error) {
	if m.getLeadErr != nil {
		return nil, m.getLeadErr
	}
	return m.mockRepo.GetLead(context.Background(), id)
}

func (m *mockRepoErr) ListMessages(_ context.Context, leadID uuid.UUID) ([]domain.Message, error) {
	if m.listMessagesErr != nil {
		return nil, m.listMessagesErr
	}
	return m.mockRepo.ListMessages(context.Background(), leadID)
}

func (m *mockRepoErr) CreateMessage(_ context.Context, msg *domain.Message) error {
	if m.createMessageErr != nil {
		return m.createMessageErr
	}
	return m.mockRepo.CreateMessage(context.Background(), msg)
}

func (m *mockRepoErr) GetQualification(_ context.Context, leadID uuid.UUID) (*domain.Qualification, error) {
	if m.getQualErr != nil {
		return nil, m.getQualErr
	}
	return m.mockRepo.GetQualification(context.Background(), leadID)
}

func (m *mockRepoErr) GetLatestDraft(_ context.Context, leadID uuid.UUID) (*domain.Draft, error) {
	if m.getLatestDraftErr != nil {
		return nil, m.getLatestDraftErr
	}
	return m.mockRepo.GetLatestDraft(context.Background(), leadID)
}

func newTestRouter(uc *UseCase) *chi.Mux {
	r := chi.NewRouter()
	RegisterRoutes(r, uc)
	return r
}

func reqWithUser(method, url string, body *bytes.Buffer, userID uuid.UUID) *http.Request {
	var req *http.Request
	if body != nil {
		req = httptest.NewRequest(method, url, body)
	} else {
		req = httptest.NewRequest(method, url, nil)
	}
	ctx := httputil.WithUserID(req.Context(), userID)
	return req.WithContext(ctx)
}

func TestHandler_ListLeads(t *testing.T) {
	repo := newMockRepo()
	userID := uuid.New()
	leadID := uuid.New()
	repo.leads[leadID] = &domain.Lead{
		ID:          leadID,
		UserID:      userID,
		Channel:     domain.ChannelEmail,
		ContactName: "Alice",
		Status:      domain.StatusNew,
	}

	r := newTestRouter(NewUseCase(repo, &mockAI{}, nil))
	w := httptest.NewRecorder()
	req := reqWithUser("GET", "/api/leads", nil, userID)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp []LeadResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Len(t, resp, 1)
	assert.Equal(t, "Alice", resp[0].ContactName)
}

func TestHandler_ListLeads_NoAuth(t *testing.T) {
	r := newTestRouter(NewUseCase(newMockRepo(), &mockAI{}, nil))
	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/leads", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestHandler_GetLead(t *testing.T) {
	repo := newMockRepo()
	userID := uuid.New()
	leadID := uuid.New()
	repo.leads[leadID] = &domain.Lead{
		ID:          leadID,
		UserID:      userID,
		Channel:     domain.ChannelEmail,
		ContactName: "Bob",
		Status:      domain.StatusNew,
	}

	r := newTestRouter(NewUseCase(repo, &mockAI{}, nil))
	w := httptest.NewRecorder()
	req := reqWithUser("GET", fmt.Sprintf("/api/leads/%s", leadID), nil, userID)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp LeadResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "Bob", resp.ContactName)
}

func TestHandler_GetLead_NotFound(t *testing.T) {
	r := newTestRouter(NewUseCase(newMockRepo(), &mockAI{}, nil))
	w := httptest.NewRecorder()
	req := reqWithUser("GET", fmt.Sprintf("/api/leads/%s", uuid.New()), nil, uuid.New())
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestHandler_GetLead_BadID(t *testing.T) {
	r := newTestRouter(NewUseCase(newMockRepo(), &mockAI{}, nil))
	w := httptest.NewRecorder()
	req := reqWithUser("GET", "/api/leads/not-a-uuid", nil, uuid.New())
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandler_GetLead_IncludesIdentitySummary(t *testing.T) {
	repo := newMockRepo()
	userID := uuid.New()
	leadID := uuid.New()
	repo.leads[leadID] = &domain.Lead{
		ID:          leadID,
		UserID:      userID,
		Channel:     domain.ChannelEmail,
		ContactName: "Bob",
		Status:      domain.StatusNew,
	}

	identityID := uuid.New()
	otherLead := uuid.New()
	repo.leads[otherLead] = &domain.Lead{ID: otherLead, UserID: userID, Channel: domain.ChannelTelegram, ContactName: "Bob", Status: domain.StatusNew}
	identities := newStubIdentityReader()
	identities.byLead[leadID] = &domain.Identity{
		ID:               identityID,
		UserID:           userID,
		Email:            "bob@acme.com",
		TelegramUsername: "bob",
	}
	identities.linkedByIdentity[identityID] = []uuid.UUID{leadID, otherLead}

	r := newTestRouter(NewUseCase(repo, &mockAI{}, nil, WithIdentityReader(identities)))
	w := httptest.NewRecorder()
	req := reqWithUser("GET", fmt.Sprintf("/api/leads/%s", leadID), nil, userID)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp LeadResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.NotNil(t, resp.Identity, "identity summary must be present when reader is wired and lead has a link")
	assert.Equal(t, identityID, resp.Identity.ID)
	assert.Equal(t, "bob@acme.com", resp.Identity.Email)
	assert.Equal(t, "bob", resp.Identity.TelegramUsername)
	assert.ElementsMatch(t, []uuid.UUID{leadID, otherLead}, resp.Identity.LinkedLeadIDs)
}

func TestHandler_GetLead_ForbidsCrossTenant(t *testing.T) {
	repo := newMockRepo()
	ownerID := uuid.New()
	attackerID := uuid.New()
	leadID := uuid.New()
	repo.leads[leadID] = &domain.Lead{
		ID:          leadID,
		UserID:      ownerID,
		Channel:     domain.ChannelEmail,
		ContactName: "Owner-private",
		Status:      domain.StatusNew,
	}

	r := newTestRouter(NewUseCase(repo, &mockAI{}, nil, WithIdentityReader(newStubIdentityReader())))
	w := httptest.NewRecorder()
	req := reqWithUser("GET", fmt.Sprintf("/api/leads/%s", leadID), nil, attackerID)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code,
		"a user must not be able to read another tenant's lead — indistinguishable from non-existent")
}

func TestHandler_ListMessages_ForbidsCrossTenant(t *testing.T) {
	repo := newMockRepo()
	ownerID := uuid.New()
	attackerID := uuid.New()
	leadID := uuid.New()
	repo.leads[leadID] = &domain.Lead{
		ID:          leadID,
		UserID:      ownerID,
		Channel:     domain.ChannelEmail,
		ContactName: "Owner-private",
		Status:      domain.StatusNew,
	}

	r := newTestRouter(NewUseCase(repo, &mockAI{}, nil))
	w := httptest.NewRecorder()
	req := reqWithUser("GET", fmt.Sprintf("/api/leads/%s/messages", leadID), nil, attackerID)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code,
		"cross-tenant message read must 404 (per-tenant boundary, no info leak)")
}

func TestHandler_LeadSubresources_ForbidCrossTenant(t *testing.T) {
	// Six lead sub-resources are owned by user A; user B (the
	// attacker) tries to touch each with a valid JWT and the
	// owner's leadID. Every endpoint must answer 404 — the body
	// changes per operation but the contract is uniform: cross-tenant
	// probes can never observe a non-404 response.
	cases := []struct {
		name   string
		method string
		path   func(uuid.UUID) string
		body   func() *bytes.Buffer
	}{
		{name: "PATCH status", method: "PATCH",
			path: func(id uuid.UUID) string { return fmt.Sprintf("/api/leads/%s/status", id) },
			body: func() *bytes.Buffer { return bytes.NewBufferString(`{"status":"qualified"}`) }},
		{name: "POST send", method: "POST",
			path: func(id uuid.UUID) string { return fmt.Sprintf("/api/leads/%s/send", id) },
			body: func() *bytes.Buffer { return bytes.NewBufferString(`{"body":"poke"}`) }},
		{name: "GET qualification", method: "GET",
			path: func(id uuid.UUID) string { return fmt.Sprintf("/api/leads/%s/qualification", id) }},
		{name: "POST qualify", method: "POST",
			path: func(id uuid.UUID) string { return fmt.Sprintf("/api/leads/%s/qualify", id) }},
		{name: "GET draft", method: "GET",
			path: func(id uuid.UUID) string { return fmt.Sprintf("/api/leads/%s/draft", id) }},
		{name: "POST draft regen", method: "POST",
			path: func(id uuid.UUID) string { return fmt.Sprintf("/api/leads/%s/draft/regen", id) }},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := newMockRepo()
			ownerID := uuid.New()
			attackerID := uuid.New()
			leadID := uuid.New()
			repo.leads[leadID] = &domain.Lead{
				ID: leadID, UserID: ownerID,
				Channel: domain.ChannelEmail, ContactName: "Owner", Status: domain.StatusNew,
			}

			r := newTestRouter(NewUseCase(repo, &mockAI{}, nil))
			w := httptest.NewRecorder()
			var body *bytes.Buffer
			if tc.body != nil {
				body = tc.body()
			}
			req := reqWithUser(tc.method, tc.path(leadID), body, attackerID)
			req.Header.Set("Content-Type", "application/json")
			r.ServeHTTP(w, req)

			assert.Equal(t, http.StatusNotFound, w.Code,
				"%s must 404 for foreign tenant — no info leak", tc.name)
		})
	}
}

func TestHandler_ListMessages_Aggregated_ForbidsCrossTenant(t *testing.T) {
	repo := newMockRepo()
	ownerID := uuid.New()
	attackerID := uuid.New()
	leadID := uuid.New()
	repo.leads[leadID] = &domain.Lead{
		ID:          leadID,
		UserID:      ownerID,
		Channel:     domain.ChannelEmail,
		ContactName: "Owner-private",
		Status:      domain.StatusNew,
	}

	r := newTestRouter(NewUseCase(repo, &mockAI{}, nil, WithIdentityReader(newStubIdentityReader())))
	w := httptest.NewRecorder()
	req := reqWithUser("GET", fmt.Sprintf("/api/leads/%s/messages?aggregated=true", leadID), nil, attackerID)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code,
		"aggregated variant must enforce ownership before crossing into the identity-merged timeline")
}

func TestHandler_GetLead_OmitsIdentityWhenNoLink(t *testing.T) {
	repo := newMockRepo()
	userID := uuid.New()
	leadID := uuid.New()
	repo.leads[leadID] = &domain.Lead{
		ID:          leadID,
		UserID:      userID,
		Channel:     domain.ChannelEmail,
		ContactName: "Bob",
		Status:      domain.StatusNew,
	}

	r := newTestRouter(NewUseCase(repo, &mockAI{}, nil, WithIdentityReader(newStubIdentityReader())))
	w := httptest.NewRecorder()
	req := reqWithUser("GET", fmt.Sprintf("/api/leads/%s", leadID), nil, userID)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp LeadResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Nil(t, resp.Identity, "identity must be omitted when no link exists for the lead")
}

func TestHandler_UpdateStatus(t *testing.T) {
	repo := newMockRepo()
	userID := uuid.New()
	leadID := uuid.New()
	repo.leads[leadID] = &domain.Lead{
		ID:     leadID,
		UserID: userID,
		Status: domain.StatusNew,
	}

	r := newTestRouter(NewUseCase(repo, &mockAI{}, nil))
	body := bytes.NewBufferString(`{"status":"qualified"}`)
	w := httptest.NewRecorder()
	req := reqWithUser("PATCH", fmt.Sprintf("/api/leads/%s/status", leadID), body, userID)
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]string
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "qualified", resp["status"])
}

func TestHandler_UpdateStatus_EmptyStatus(t *testing.T) {
	repo := newMockRepo()
	userID := uuid.New()
	leadID := uuid.New()
	repo.leads[leadID] = &domain.Lead{ID: leadID, UserID: userID, Status: domain.StatusNew}
	r := newTestRouter(NewUseCase(repo, &mockAI{}, nil))
	body := bytes.NewBufferString(`{"status":""}`)
	w := httptest.NewRecorder()
	req := reqWithUser("PATCH", fmt.Sprintf("/api/leads/%s/status", leadID), body, userID)
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandler_UpdateStatus_InvalidJSON(t *testing.T) {
	repo := newMockRepo()
	userID := uuid.New()
	leadID := uuid.New()
	repo.leads[leadID] = &domain.Lead{ID: leadID, UserID: userID, Status: domain.StatusNew}
	r := newTestRouter(NewUseCase(repo, &mockAI{}, nil))
	body := bytes.NewBufferString(`not json`)
	w := httptest.NewRecorder()
	req := reqWithUser("PATCH", fmt.Sprintf("/api/leads/%s/status", leadID), body, userID)
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandler_ExportCSV(t *testing.T) {
	repo := newMockRepo()
	userID := uuid.New()
	leadID := uuid.New()
	repo.leads[leadID] = &domain.Lead{
		ID:          leadID,
		UserID:      userID,
		Channel:     domain.ChannelEmail,
		ContactName: "Alice",
		Company:     "Acme",
		Status:      domain.StatusNew,
	}

	r := newTestRouter(NewUseCase(repo, &mockAI{}, nil))
	w := httptest.NewRecorder()
	req := reqWithUser("GET", "/api/leads/export", nil, userID)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Header().Get("Content-Type"), "text/csv")
	assert.Contains(t, w.Header().Get("Content-Disposition"), "floq-leads-")
	assert.Contains(t, w.Body.String(), "Alice,Acme,email")
}

func TestHandler_ExportCSV_NoAuth(t *testing.T) {
	r := newTestRouter(NewUseCase(newMockRepo(), &mockAI{}, nil))
	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/leads/export", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestHandler_ImportCSV(t *testing.T) {
	repo := newMockRepo()
	userID := uuid.New()

	csvData := "contact_name,channel,company\nAlice,email,Acme\n"
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, err := mw.CreateFormFile("file", "leads.csv")
	require.NoError(t, err)
	_, err = fw.Write([]byte(csvData))
	require.NoError(t, err)
	mw.Close()

	r := newTestRouter(NewUseCase(repo, &mockAI{}, nil))
	w := httptest.NewRecorder()
	req := reqWithUser("POST", "/api/leads/import", &buf, userID)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]int
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, 1, resp["imported"])
}

func TestHandler_ImportCSV_TooLarge(t *testing.T) {
	repo := newMockRepo()
	// Tiny caps exercise the 413 path without a real 50 MiB body: upload=50 bytes.
	r := chi.NewRouter()
	r.Use(httputil.MaxBodyBytesWithUploads(10, 50))
	RegisterRoutes(r, NewUseCase(repo, &mockAI{}, nil))

	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, err := mw.CreateFormFile("file", "big.csv")
	require.NoError(t, err)
	_, err = fw.Write(bytes.Repeat([]byte("x"), 200)) // exceeds the 50-byte upload cap
	require.NoError(t, err)
	mw.Close()

	w := httptest.NewRecorder()
	req := reqWithUser("POST", "/api/leads/import", &buf, uuid.New())
	req.Header.Set("Content-Type", mw.FormDataContentType())
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusRequestEntityTooLarge, w.Code, "oversized upload must be 413, not 400")
}

func TestHandler_ImportCSV_NoFile(t *testing.T) {
	r := newTestRouter(NewUseCase(newMockRepo(), &mockAI{}, nil))
	w := httptest.NewRecorder()
	req := reqWithUser("POST", "/api/leads/import", bytes.NewBuffer(nil), uuid.New())
	req.Header.Set("Content-Type", "multipart/form-data; boundary=xxx")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandler_ListMessages(t *testing.T) {
	repo := newMockRepo()
	userID := uuid.New()
	leadID := uuid.New()
	repo.leads[leadID] = &domain.Lead{ID: leadID, UserID: userID}
	repo.messages[leadID] = []domain.Message{
		{ID: uuid.New(), LeadID: leadID, Direction: domain.DirectionInbound, Body: "hi"},
	}

	r := newTestRouter(NewUseCase(repo, &mockAI{}, nil))
	w := httptest.NewRecorder()
	req := reqWithUser("GET", fmt.Sprintf("/api/leads/%s/messages", leadID), nil, userID)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp []MessageResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Len(t, resp, 1)
	assert.Equal(t, "hi", resp[0].Body)
}

func TestHandler_SendMessage(t *testing.T) {
	repo := newMockRepo()
	userID := uuid.New()
	leadID := uuid.New()
	repo.leads[leadID] = &domain.Lead{
		ID:      leadID,
		UserID:  userID,
		Channel: domain.ChannelEmail,
		Status:  domain.StatusInConversation,
	}

	r := newTestRouter(NewUseCase(repo, &mockAI{}, nil))
	body := bytes.NewBufferString(`{"body":"Hello!"}`)
	w := httptest.NewRecorder()
	req := reqWithUser("POST", fmt.Sprintf("/api/leads/%s/send", leadID), body, userID)
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
	var resp MessageResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "Hello!", resp.Body)
}

func TestHandler_SendMessage_EmptyBody(t *testing.T) {
	repo := newMockRepo()
	userID := uuid.New()
	leadID := uuid.New()
	repo.leads[leadID] = &domain.Lead{ID: leadID, UserID: userID, Status: domain.StatusInConversation}
	r := newTestRouter(NewUseCase(repo, &mockAI{}, nil))
	body := bytes.NewBufferString(`{"body":""}`)
	w := httptest.NewRecorder()
	req := reqWithUser("POST", fmt.Sprintf("/api/leads/%s/send", leadID), body, userID)
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandler_GetQualification(t *testing.T) {
	repo := newMockRepo()
	userID := uuid.New()
	leadID := uuid.New()
	repo.leads[leadID] = &domain.Lead{ID: leadID, UserID: userID, Status: domain.StatusNew}
	repo.qualifications[leadID] = &domain.Qualification{
		ID:             uuid.New(),
		LeadID:         leadID,
		IdentifiedNeed: "CRM",
		Score:          80,
	}

	r := newTestRouter(NewUseCase(repo, &mockAI{}, nil))
	w := httptest.NewRecorder()
	req := reqWithUser("GET", fmt.Sprintf("/api/leads/%s/qualification", leadID), nil, userID)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp QualificationResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "CRM", resp.IdentifiedNeed)
}

func TestHandler_GetQualification_NotFound(t *testing.T) {
	repo := newMockRepo()
	userID := uuid.New()
	leadID := uuid.New()
	repo.leads[leadID] = &domain.Lead{ID: leadID, UserID: userID, Status: domain.StatusNew}
	r := newTestRouter(NewUseCase(repo, &mockAI{}, nil))
	w := httptest.NewRecorder()
	req := reqWithUser("GET", fmt.Sprintf("/api/leads/%s/qualification", leadID), nil, userID)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestHandler_GetDraft(t *testing.T) {
	repo := newMockRepo()
	userID := uuid.New()
	leadID := uuid.New()
	repo.leads[leadID] = &domain.Lead{ID: leadID, UserID: userID, Status: domain.StatusInConversation}
	repo.drafts[leadID] = &domain.Draft{
		ID:     uuid.New(),
		LeadID: leadID,
		Body:   "draft text",
	}

	r := newTestRouter(NewUseCase(repo, &mockAI{}, nil))
	w := httptest.NewRecorder()
	req := reqWithUser("GET", fmt.Sprintf("/api/leads/%s/draft", leadID), nil, userID)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp DraftResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "draft text", resp.Body)
}

func TestHandler_GetDraft_NotFound(t *testing.T) {
	repo := newMockRepo()
	userID := uuid.New()
	leadID := uuid.New()
	repo.leads[leadID] = &domain.Lead{ID: leadID, UserID: userID, Status: domain.StatusInConversation}
	r := newTestRouter(NewUseCase(repo, &mockAI{}, nil))
	w := httptest.NewRecorder()
	req := reqWithUser("GET", fmt.Sprintf("/api/leads/%s/draft", leadID), nil, userID)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestHandler_QualifyLead(t *testing.T) {
	repo := newMockRepo()
	userID := uuid.New()
	leadID := uuid.New()
	repo.leads[leadID] = &domain.Lead{
		ID:           leadID,
		UserID:       userID,
		ContactName:  "Ivan",
		Channel:      domain.ChannelTelegram,
		FirstMessage: "Need CRM",
		Status:       domain.StatusNew,
	}

	aiSvc := &mockAI{
		qualifyResult: &domain.Qualification{
			IdentifiedNeed:    "CRM",
			EstimatedBudget:   "100k",
			Score:             90,
			ScoreReason:       "High",
			RecommendedAction: "Call",
			ProviderUsed:      "test",
		},
	}

	r := newTestRouter(NewUseCase(repo, aiSvc, nil))
	w := httptest.NewRecorder()
	req := reqWithUser("POST", fmt.Sprintf("/api/leads/%s/qualify", leadID), nil, userID)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp QualificationResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "CRM", resp.IdentifiedNeed)
}

func TestHandler_RegenerateDraft(t *testing.T) {
	repo := newMockRepo()
	userID := uuid.New()
	leadID := uuid.New()
	repo.leads[leadID] = &domain.Lead{
		ID:           leadID,
		UserID:       userID,
		ContactName:  "Anna",
		FirstMessage: "Looking for help",
	}

	aiSvc := &mockAI{draftBody: "Dear Anna, ..."}
	r := newTestRouter(NewUseCase(repo, aiSvc, nil))
	w := httptest.NewRecorder()
	req := reqWithUser("POST", fmt.Sprintf("/api/leads/%s/draft/regen", leadID), nil, userID)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp DraftResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "Dear Anna, ...", resp.Body)
}

// --- Error path handler tests ---

func TestHandler_ListLeads_Error(t *testing.T) {
	repo := &mockRepoErr{mockRepo: *newMockRepo(), listLeadsErr: fmt.Errorf("db down")}
	r := newTestRouter(NewUseCase(repo, &mockAI{}, nil))
	w := httptest.NewRecorder()
	req := reqWithUser("GET", "/api/leads", nil, uuid.New())
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestHandler_GetLead_Error(t *testing.T) {
	repo := &mockRepoErr{mockRepo: *newMockRepo(), getLeadForUserErr: fmt.Errorf("db down")}
	r := newTestRouter(NewUseCase(repo, &mockAI{}, nil))
	w := httptest.NewRecorder()
	req := reqWithUser("GET", fmt.Sprintf("/api/leads/%s", uuid.New()), nil, uuid.New())
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestHandler_ListMessages_Error(t *testing.T) {
	userID := uuid.New()
	leadID := uuid.New()
	base := newMockRepo()
	base.leads[leadID] = &domain.Lead{ID: leadID, UserID: userID}
	repo := &mockRepoErr{mockRepo: *base, listMessagesErr: fmt.Errorf("db down")}
	r := newTestRouter(NewUseCase(repo, &mockAI{}, nil))
	w := httptest.NewRecorder()
	req := reqWithUser("GET", fmt.Sprintf("/api/leads/%s/messages", leadID), nil, userID)
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestHandler_ListMessages_BadID(t *testing.T) {
	r := newTestRouter(NewUseCase(newMockRepo(), &mockAI{}, nil))
	w := httptest.NewRecorder()
	req := reqWithUser("GET", "/api/leads/bad-id/messages", nil, uuid.New())
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandler_SendMessage_BadID(t *testing.T) {
	r := newTestRouter(NewUseCase(newMockRepo(), &mockAI{}, nil))
	body := bytes.NewBufferString(`{"body":"hi"}`)
	w := httptest.NewRecorder()
	req := reqWithUser("POST", "/api/leads/bad-id/send", body, uuid.New())
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandler_SendMessage_InvalidJSON(t *testing.T) {
	repo := newMockRepo()
	userID := uuid.New()
	leadID := uuid.New()
	repo.leads[leadID] = &domain.Lead{ID: leadID, UserID: userID, Status: domain.StatusInConversation}
	r := newTestRouter(NewUseCase(repo, &mockAI{}, nil))
	body := bytes.NewBufferString(`not json`)
	w := httptest.NewRecorder()
	req := reqWithUser("POST", fmt.Sprintf("/api/leads/%s/send", leadID), body, userID)
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandler_GetQualification_BadID(t *testing.T) {
	r := newTestRouter(NewUseCase(newMockRepo(), &mockAI{}, nil))
	w := httptest.NewRecorder()
	req := reqWithUser("GET", "/api/leads/bad-id/qualification", nil, uuid.New())
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandler_GetQualification_Error(t *testing.T) {
	userID := uuid.New()
	leadID := uuid.New()
	base := newMockRepo()
	base.leads[leadID] = &domain.Lead{ID: leadID, UserID: userID, Status: domain.StatusNew}
	repo := &mockRepoErr{mockRepo: *base, getQualErr: fmt.Errorf("db down")}
	r := newTestRouter(NewUseCase(repo, &mockAI{}, nil))
	w := httptest.NewRecorder()
	req := reqWithUser("GET", fmt.Sprintf("/api/leads/%s/qualification", leadID), nil, userID)
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestHandler_QualifyLead_BadID(t *testing.T) {
	r := newTestRouter(NewUseCase(newMockRepo(), &mockAI{}, nil))
	w := httptest.NewRecorder()
	req := reqWithUser("POST", "/api/leads/bad-id/qualify", nil, uuid.New())
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandler_QualifyLead_Error(t *testing.T) {
	// authorizeLead passes, then QualifyLead fails because the AI
	// stub has no result configured — usecase derefs nil aiResult.
	userID := uuid.New()
	leadID := uuid.New()
	repo := newMockRepo()
	repo.leads[leadID] = &domain.Lead{
		ID: leadID, UserID: userID, Channel: domain.ChannelEmail,
		ContactName: "X", FirstMessage: "hi", Status: domain.StatusNew,
	}
	ai := &mockAI{qualifyErr: fmt.Errorf("ai down")}
	r := newTestRouter(NewUseCase(repo, ai, nil))
	w := httptest.NewRecorder()
	req := reqWithUser("POST", fmt.Sprintf("/api/leads/%s/qualify", leadID), nil, userID)
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestHandler_GetDraft_BadID(t *testing.T) {
	r := newTestRouter(NewUseCase(newMockRepo(), &mockAI{}, nil))
	w := httptest.NewRecorder()
	req := reqWithUser("GET", "/api/leads/bad-id/draft", nil, uuid.New())
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandler_GetDraft_Error(t *testing.T) {
	userID := uuid.New()
	leadID := uuid.New()
	base := newMockRepo()
	base.leads[leadID] = &domain.Lead{ID: leadID, UserID: userID, Status: domain.StatusInConversation}
	repo := &mockRepoErr{mockRepo: *base, getLatestDraftErr: fmt.Errorf("db down")}
	r := newTestRouter(NewUseCase(repo, &mockAI{}, nil))
	w := httptest.NewRecorder()
	req := reqWithUser("GET", fmt.Sprintf("/api/leads/%s/draft", leadID), nil, userID)
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestHandler_RegenerateDraft_BadID(t *testing.T) {
	r := newTestRouter(NewUseCase(newMockRepo(), &mockAI{}, nil))
	w := httptest.NewRecorder()
	req := reqWithUser("POST", "/api/leads/bad-id/draft/regen", nil, uuid.New())
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandler_RegenerateDraft_Error(t *testing.T) {
	// authorizeLead passes; the AI stub errors so regenerate fails downstream.
	userID := uuid.New()
	leadID := uuid.New()
	repo := newMockRepo()
	repo.leads[leadID] = &domain.Lead{
		ID: leadID, UserID: userID, Channel: domain.ChannelEmail,
		ContactName: "X", FirstMessage: "hi", Status: domain.StatusNew,
	}
	ai := &mockAI{draftErr: fmt.Errorf("ai down")}
	r := newTestRouter(NewUseCase(repo, ai, nil))
	w := httptest.NewRecorder()
	req := reqWithUser("POST", fmt.Sprintf("/api/leads/%s/draft/regen", leadID), nil, userID)
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestHandler_UpdateStatus_BadID(t *testing.T) {
	r := newTestRouter(NewUseCase(newMockRepo(), &mockAI{}, nil))
	body := bytes.NewBufferString(`{"status":"qualified"}`)
	w := httptest.NewRecorder()
	req := reqWithUser("PATCH", "/api/leads/bad-id/status", body, uuid.New())
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandler_SendMessage_UCError(t *testing.T) {
	// authorizeLead passes; the use case-level GetLead inside SendMessage
	// returns an error (db blip) — the handler maps to 500.
	userID := uuid.New()
	leadID := uuid.New()
	base := newMockRepo()
	base.leads[leadID] = &domain.Lead{ID: leadID, UserID: userID, Status: domain.StatusInConversation}
	repo := &mockRepoErr{mockRepo: *base, getLeadErr: fmt.Errorf("db error")}
	r := newTestRouter(NewUseCase(repo, &mockAI{}, nil))
	body := bytes.NewBufferString(`{"body":"hi"}`)
	w := httptest.NewRecorder()
	req := reqWithUser("POST", fmt.Sprintf("/api/leads/%s/send", leadID), body, userID)
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestHandler_ImportCSV_NoAuth(t *testing.T) {
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, _ := mw.CreateFormFile("file", "test.csv")
	fw.Write([]byte("contact_name,channel\nAlice,email\n"))
	mw.Close()

	r := newTestRouter(NewUseCase(newMockRepo(), &mockAI{}, nil))
	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/leads/import", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestHandler_ImportCSV_BadCSV(t *testing.T) {
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, _ := mw.CreateFormFile("file", "test.csv")
	fw.Write([]byte("name,email\nAlice,a@b.com\n")) // missing required columns
	mw.Close()

	r := newTestRouter(NewUseCase(newMockRepo(), &mockAI{}, nil))
	w := httptest.NewRecorder()
	req := reqWithUser("POST", "/api/leads/import", &buf, uuid.New())
	req.Header.Set("Content-Type", mw.FormDataContentType())
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandler_ExportCSV_Error(t *testing.T) {
	repo := &mockRepoErr{mockRepo: *newMockRepo(), listLeadsErr: fmt.Errorf("db down")}
	r := newTestRouter(NewUseCase(repo, &mockAI{}, nil))
	w := httptest.NewRecorder()
	req := reqWithUser("GET", "/api/leads/export", nil, uuid.New())
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestHandler_UpdateStatus_UCError(t *testing.T) {
	// Invalid status in the UC
	repo := newMockRepo()
	userID := uuid.New()
	leadID := uuid.New()
	repo.leads[leadID] = &domain.Lead{ID: leadID, UserID: userID, Status: domain.StatusNew}
	r := newTestRouter(NewUseCase(repo, &mockAI{}, nil))
	body := bytes.NewBufferString(`{"status":"won"}`) // invalid transition new->won
	w := httptest.NewRecorder()
	req := reqWithUser("PATCH", fmt.Sprintf("/api/leads/%s/status", leadID), body, userID)
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandler_ListLeads_EmptyList(t *testing.T) {
	r := newTestRouter(NewUseCase(newMockRepo(), &mockAI{}, nil))
	w := httptest.NewRecorder()
	req := reqWithUser("GET", "/api/leads", nil, uuid.New())
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp []LeadResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Len(t, resp, 0)
}

func TestHandler_ListMessages_EmptyList(t *testing.T) {
	repo := newMockRepo()
	userID := uuid.New()
	leadID := uuid.New()
	repo.leads[leadID] = &domain.Lead{ID: leadID, UserID: userID}
	r := newTestRouter(NewUseCase(repo, &mockAI{}, nil))
	w := httptest.NewRecorder()
	req := reqWithUser("GET", fmt.Sprintf("/api/leads/%s/messages", leadID), nil, userID)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp []MessageResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Len(t, resp, 0)
}
