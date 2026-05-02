package verify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/daniil/floq/internal/httputil"
	"github.com/daniil/floq/internal/prospects/domain"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Mock ProspectRepository ---

type mockProspectRepo struct {
	prospects       []domain.ProspectWithSource
	prospect        *domain.Prospect
	listErr         error
	getErr          error
	updateVerifyErr error
	updatedIDs      []uuid.UUID
}

func (m *mockProspectRepo) ListProspects(_ context.Context, _ uuid.UUID) ([]domain.ProspectWithSource, error) {
	return m.prospects, m.listErr
}

func (m *mockProspectRepo) GetProspect(_ context.Context, id uuid.UUID) (*domain.Prospect, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	return m.prospect, nil
}

func (m *mockProspectRepo) UpdateVerification(_ context.Context, id uuid.UUID, _ domain.VerifyStatus, _ int, _ string, _ time.Time) error {
	m.updatedIDs = append(m.updatedIDs, id)
	return m.updateVerifyErr
}

func setupVerifyRouter() chi.Router {
	r := chi.NewRouter()
	repo := &mockProspectRepo{}
	RegisterRoutes(r, repo, nil, nil)
	return r
}

func setupVerifyRouterWithRepo(repo *mockProspectRepo) chi.Router {
	r := chi.NewRouter()
	RegisterRoutes(r, repo, nil, nil)
	return r
}

// --- verifyEmailSingle ---

func TestHandler_VerifyEmail_Success(t *testing.T) {
	r := setupVerifyRouter()
	body := `{"email":"test@gmail.com"}`
	req := httptest.NewRequest("POST", "/api/verify/email", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	var result EmailResult
	err := json.NewDecoder(rec.Body).Decode(&result)
	require.NoError(t, err)
	assert.Equal(t, "test@gmail.com", result.Email)
	assert.True(t, result.IsValidSyntax)
}

func TestHandler_VerifyEmail_InvalidSyntax(t *testing.T) {
	r := setupVerifyRouter()
	body := `{"email":"not-valid"}`
	req := httptest.NewRequest("POST", "/api/verify/email", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	var result EmailResult
	err := json.NewDecoder(rec.Body).Decode(&result)
	require.NoError(t, err)
	assert.False(t, result.IsValidSyntax)
	assert.Equal(t, "invalid", result.Status)
	assert.Equal(t, 0, result.Score)
}

func TestHandler_VerifyEmail_Empty(t *testing.T) {
	r := setupVerifyRouter()
	body := `{"email":""}`
	req := httptest.NewRequest("POST", "/api/verify/email", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestHandler_VerifyEmail_InvalidJSON(t *testing.T) {
	r := setupVerifyRouter()
	req := httptest.NewRequest("POST", "/api/verify/email", bytes.NewBufferString("bad"))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestHandler_VerifyEmail_MissingField(t *testing.T) {
	r := setupVerifyRouter()
	body := `{}`
	req := httptest.NewRequest("POST", "/api/verify/email", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

// --- getVerifyStatus ---

func TestHandler_GetVerifyStatus_InvalidID(t *testing.T) {
	r := setupVerifyRouter()
	req := httptest.NewRequest("GET", "/api/prospects/not-uuid/verify", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestHandler_GetVerifyStatus_NotFound(t *testing.T) {
	repo := &mockProspectRepo{prospect: nil}
	r := setupVerifyRouterWithRepo(repo)
	id := uuid.New()
	req := httptest.NewRequest("GET", "/api/prospects/"+id.String()+"/verify", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestHandler_GetVerifyStatus_Found(t *testing.T) {
	repo := &mockProspectRepo{
		prospect: &domain.Prospect{
			VerifyStatus:  domain.VerifyStatusValid,
			VerifyScore:   90,
			VerifyDetails: `{"email":{"status":"valid"}}`,
		},
	}
	r := setupVerifyRouterWithRepo(repo)
	id := uuid.New()
	req := httptest.NewRequest("GET", "/api/prospects/"+id.String()+"/verify", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	var resp map[string]any
	err := json.NewDecoder(rec.Body).Decode(&resp)
	require.NoError(t, err)
	assert.Equal(t, string(domain.VerifyStatusValid), resp["verify_status"])
	assert.Equal(t, float64(90), resp["verify_score"])
}

func TestHandler_GetVerifyStatus_RepoError(t *testing.T) {
	repo := &mockProspectRepo{getErr: fmt.Errorf("db error")}
	r := setupVerifyRouterWithRepo(repo)
	id := uuid.New()
	req := httptest.NewRequest("GET", "/api/prospects/"+id.String()+"/verify", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

// --- verifyBatch ---

func TestHandler_VerifyBatch_Unauthorized(t *testing.T) {
	r := setupVerifyRouter()
	req := httptest.NewRequest("POST", "/api/verify/batch", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestHandler_VerifyBatch_ListError(t *testing.T) {
	repo := &mockProspectRepo{listErr: fmt.Errorf("db error")}
	r := setupVerifyRouterWithRepo(repo)
	userID := uuid.New()
	ctx := httputil.WithUserID(context.Background(), userID)
	req := httptest.NewRequest("POST", "/api/verify/batch", nil).WithContext(ctx)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestHandler_VerifyBatch_NoProspects(t *testing.T) {
	repo := &mockProspectRepo{prospects: nil}
	r := setupVerifyRouterWithRepo(repo)
	userID := uuid.New()
	ctx := httputil.WithUserID(context.Background(), userID)
	req := httptest.NewRequest("POST", "/api/verify/batch", nil).WithContext(ctx)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	var resp map[string]int
	err := json.NewDecoder(rec.Body).Decode(&resp)
	require.NoError(t, err)
	assert.Equal(t, 0, resp["verified"])
}

func TestHandler_VerifyBatch_AlreadyChecked(t *testing.T) {
	repo := &mockProspectRepo{
		prospects: []domain.ProspectWithSource{
			{Prospect: domain.Prospect{ID: uuid.New(), Email: "a@b.com", VerifyStatus: domain.VerifyStatusValid}},
		},
	}
	r := setupVerifyRouterWithRepo(repo)
	userID := uuid.New()
	ctx := httputil.WithUserID(context.Background(), userID)
	req := httptest.NewRequest("POST", "/api/verify/batch", nil).WithContext(ctx)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	var resp map[string]int
	err := json.NewDecoder(rec.Body).Decode(&resp)
	require.NoError(t, err)
	assert.Equal(t, 0, resp["verified"])
}

func TestHandler_VerifyBatch_VerifiesNotChecked(t *testing.T) {
	pID := uuid.New()
	repo := &mockProspectRepo{
		prospects: []domain.ProspectWithSource{
			{Prospect: domain.Prospect{ID: pID, Email: "test@gmail.com", VerifyStatus: domain.VerifyStatusNotChecked}},
		},
	}
	r := setupVerifyRouterWithRepo(repo)
	userID := uuid.New()
	ctx := httputil.WithUserID(context.Background(), userID)
	req := httptest.NewRequest("POST", "/api/verify/batch", nil).WithContext(ctx)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	var resp map[string]int
	err := json.NewDecoder(rec.Body).Decode(&resp)
	require.NoError(t, err)
	assert.Equal(t, 1, resp["verified"])
	assert.Contains(t, repo.updatedIDs, pID)
}

func TestHandler_VerifyBatch_UpdateVerifyError(t *testing.T) {
	pID := uuid.New()
	repo := &mockProspectRepo{
		prospects: []domain.ProspectWithSource{
			{Prospect: domain.Prospect{ID: pID, Email: "test@gmail.com", VerifyStatus: domain.VerifyStatusNotChecked}},
		},
		updateVerifyErr: fmt.Errorf("update error"),
	}
	r := setupVerifyRouterWithRepo(repo)
	userID := uuid.New()
	ctx := httputil.WithUserID(context.Background(), userID)
	req := httptest.NewRequest("POST", "/api/verify/batch", nil).WithContext(ctx)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	var resp map[string]int
	err := json.NewDecoder(rec.Body).Decode(&resp)
	require.NoError(t, err)
	assert.Equal(t, 0, resp["verified"]) // count not incremented on error
}

// --- VerifyEmail function ---

func TestVerifyEmail_DisposableDomain(t *testing.T) {
	result := VerifyEmail(context.Background(), "user@mailinator.com", nil)
	assert.True(t, result.IsValidSyntax)
	assert.True(t, result.IsDisposable)
	assert.Equal(t, 5, result.Score)
	assert.Equal(t, "invalid", result.Status)
}

func TestVerifyEmail_FreeProvider(t *testing.T) {
	result := VerifyEmail(context.Background(), "user@gmail.com", nil)
	assert.True(t, result.IsValidSyntax)
	assert.True(t, result.IsFreeProvider)
}

func TestVerifyEmail_ValidSyntaxFormat(t *testing.T) {
	tests := []struct {
		email string
		valid bool
	}{
		{"user@domain.com", true},
		{"user+tag@domain.co.uk", true},
		{"@domain.com", false},
		{"user@", false},
		{"user@domain", false},
		{"", false},
	}
	for _, tc := range tests {
		result := VerifyEmail(context.Background(), tc.email, nil)
		assert.Equal(t, tc.valid, result.IsValidSyntax, "email: %s", tc.email)
	}
}

// --- TelegramResult ---

func TestTelegramResult_Fields(t *testing.T) {
	r := TelegramResult{
		Username: "testuser",
		Exists:   true,
	}

	data, err := json.Marshal(r)
	require.NoError(t, err)

	var decoded TelegramResult
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)
	assert.Equal(t, "testuser", decoded.Username)
	assert.True(t, decoded.Exists)
	assert.Empty(t, decoded.Error)
}

func TestTelegramResult_WithError(t *testing.T) {
	r := TelegramResult{
		Username: "user",
		Exists:   false,
		Error:    "not found",
	}

	data, err := json.Marshal(r)
	require.NoError(t, err)
	assert.Contains(t, string(data), "not found")
}
