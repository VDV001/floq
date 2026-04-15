package settings

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/daniil/floq/internal/httputil"
	"github.com/daniil/floq/internal/settings/domain"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- mocks ---

type mockSettingsRepo struct {
	settings *domain.Settings
	err      error
	updated  map[string]any
}

func newMockSettingsRepo() *mockSettingsRepo {
	return &mockSettingsRepo{
		settings: &domain.Settings{
			FullName:     "John Doe",
			Email:        "john@example.com",
			AIProvider:   "openai",
			AIModel:      "gpt-4o",
			AIAPIKey:     "sk-test-key-12345",
			IMAPHost:     "imap.gmail.com",
			IMAPPort:     "993",
			IMAPUser:     "john@gmail.com",
			IMAPPassword: "secret-password",
			ResendAPIKey: "re_api_key_12345",
		},
	}
}

func (m *mockSettingsRepo) GetSettings(_ context.Context, _ uuid.UUID) (*domain.Settings, error) {
	if m.err != nil {
		return nil, m.err
	}
	// Return a copy to avoid mutation issues.
	cp := *m.settings
	return &cp, nil
}

func (m *mockSettingsRepo) UpdateSettings(_ context.Context, _ uuid.UUID, fields map[string]any) error {
	m.updated = fields
	return m.err
}

func (m *mockSettingsRepo) GetStoredIMAPPassword(_ context.Context, _ uuid.UUID) (string, error) {
	return m.settings.IMAPPassword, m.err
}

type mockTelegramValidator struct {
	err error
}

func (v *mockTelegramValidator) Validate(_ string) error {
	return v.err
}

func setupSettingsRouter(repo *mockSettingsRepo, validator *mockTelegramValidator) chi.Router {
	uc := NewUseCase(repo, validator)
	r := chi.NewRouter()
	RegisterRoutes(r, uc, nil, nil)
	return r
}

func withUserCtx(r *http.Request, userID uuid.UUID) *http.Request {
	ctx := httputil.WithUserID(r.Context(), userID)
	return r.WithContext(ctx)
}

// --- GetSettings ---

func TestHandler_GetSettings_Unauthorized(t *testing.T) {
	repo := newMockSettingsRepo()
	r := setupSettingsRouter(repo, &mockTelegramValidator{})
	req := httptest.NewRequest("GET", "/api/settings", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestHandler_GetSettings_Success(t *testing.T) {
	repo := newMockSettingsRepo()
	r := setupSettingsRouter(repo, &mockTelegramValidator{})
	userID := uuid.New()
	req := withUserCtx(httptest.NewRequest("GET", "/api/settings", nil), userID)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var resp Settings
	err := json.NewDecoder(rec.Body).Decode(&resp)
	require.NoError(t, err)
	assert.Equal(t, "John Doe", resp.FullName)
	// Secrets should be masked.
	assert.Equal(t, "...2345", resp.AIAPIKey)
	assert.Equal(t, "...word", resp.IMAPPassword)
	assert.Equal(t, "...2345", resp.ResendAPIKey)
}

func TestHandler_GetSettings_RepoError(t *testing.T) {
	repo := newMockSettingsRepo()
	repo.err = fmt.Errorf("db down")
	r := setupSettingsRouter(repo, &mockTelegramValidator{})
	userID := uuid.New()
	req := withUserCtx(httptest.NewRequest("GET", "/api/settings", nil), userID)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

// --- UpdateSettings ---

func TestHandler_UpdateSettings_Success(t *testing.T) {
	repo := newMockSettingsRepo()
	r := setupSettingsRouter(repo, &mockTelegramValidator{})
	userID := uuid.New()

	body := `{"auto_qualify":true}`
	req := withUserCtx(httptest.NewRequest("PUT", "/api/settings", bytes.NewBufferString(body)), userID)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.True(t, repo.updated["auto_qualify"].(bool))
}

func TestHandler_UpdateSettings_Unauthorized(t *testing.T) {
	repo := newMockSettingsRepo()
	r := setupSettingsRouter(repo, &mockTelegramValidator{})
	body := `{"auto_qualify":true}`
	req := httptest.NewRequest("PUT", "/api/settings", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestHandler_UpdateSettings_InvalidJSON(t *testing.T) {
	repo := newMockSettingsRepo()
	r := setupSettingsRouter(repo, &mockTelegramValidator{})
	userID := uuid.New()

	req := withUserCtx(httptest.NewRequest("PUT", "/api/settings", bytes.NewBufferString("not json")), userID)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestHandler_UpdateSettings_EmptyBody(t *testing.T) {
	repo := newMockSettingsRepo()
	r := setupSettingsRouter(repo, &mockTelegramValidator{})
	userID := uuid.New()

	req := withUserCtx(httptest.NewRequest("PUT", "/api/settings", bytes.NewBufferString("{}")), userID)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestHandler_UpdateSettings_InvalidTelegramToken(t *testing.T) {
	repo := newMockSettingsRepo()
	validator := &mockTelegramValidator{err: fmt.Errorf("bad token")}
	r := setupSettingsRouter(repo, validator)
	userID := uuid.New()

	body := `{"telegram_bot_token":"bad-token"}`
	req := withUserCtx(httptest.NewRequest("PUT", "/api/settings", bytes.NewBufferString(body)), userID)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

// --- Usage ---

func TestHandler_GetUsage_Unauthorized(t *testing.T) {
	repo := newMockSettingsRepo()
	r := setupSettingsRouter(repo, &mockTelegramValidator{})
	req := httptest.NewRequest("GET", "/api/usage", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestHandler_GetUsage_Success_NoCounter(t *testing.T) {
	repo := newMockSettingsRepo()
	r := setupSettingsRouter(repo, &mockTelegramValidator{})
	userID := uuid.New()
	req := withUserCtx(httptest.NewRequest("GET", "/api/usage", nil), userID)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	var resp map[string]any
	err := json.NewDecoder(rec.Body).Decode(&resp)
	require.NoError(t, err)
	assert.Equal(t, "growth", resp["plan"])
	assert.Equal(t, float64(0), resp["month_leads"])
	assert.Equal(t, float64(0), resp["total_leads"])
}

// --- Usecase tests ---

func TestUseCase_GetSettings_MasksSecrets(t *testing.T) {
	repo := newMockSettingsRepo()
	uc := NewUseCase(repo, &mockTelegramValidator{})

	s, err := uc.GetSettings(context.Background(), uuid.New())
	require.NoError(t, err)
	assert.Equal(t, "...2345", s.AIAPIKey)
	assert.Equal(t, "...word", s.IMAPPassword)
	assert.Equal(t, "...2345", s.ResendAPIKey)
	// Non-secret fields should not be masked.
	assert.Equal(t, "John Doe", s.FullName)
}

func TestUseCase_UpdateSettings_SingleField(t *testing.T) {
	repo := newMockSettingsRepo()
	uc := NewUseCase(repo, &mockTelegramValidator{})

	body := []byte(`{"auto_followup_days":7}`)
	_, err := uc.UpdateSettings(context.Background(), uuid.New(), body)
	require.NoError(t, err)
	assert.Equal(t, 7, repo.updated["auto_followup_days"])
}

func TestUseCase_UpdateSettings_MultipleFields(t *testing.T) {
	repo := newMockSettingsRepo()
	uc := NewUseCase(repo, &mockTelegramValidator{})

	body := []byte(`{"auto_qualify":true,"auto_draft":true,"auto_send_delay_min":15}`)
	_, err := uc.UpdateSettings(context.Background(), uuid.New(), body)
	require.NoError(t, err)
	assert.True(t, repo.updated["auto_qualify"].(bool))
	assert.True(t, repo.updated["auto_draft"].(bool))
	assert.Equal(t, 15, repo.updated["auto_send_delay_min"])
}

func TestUseCase_UpdateSettings_TelegramToken_SetsActive(t *testing.T) {
	repo := newMockSettingsRepo()
	uc := NewUseCase(repo, &mockTelegramValidator{})

	body := []byte(`{"telegram_bot_token":"valid-token"}`)
	_, err := uc.UpdateSettings(context.Background(), uuid.New(), body)
	require.NoError(t, err)
	assert.Equal(t, "valid-token", repo.updated["telegram_bot_token"])
	assert.True(t, repo.updated["telegram_bot_active"].(bool))
}

func TestUseCase_UpdateSettings_TelegramToken_EmptyClearsActive(t *testing.T) {
	repo := newMockSettingsRepo()
	uc := NewUseCase(repo, &mockTelegramValidator{})

	body := []byte(`{"telegram_bot_token":""}`)
	_, err := uc.UpdateSettings(context.Background(), uuid.New(), body)
	require.NoError(t, err)
	assert.False(t, repo.updated["telegram_bot_active"].(bool))
}

func TestUseCase_UpdateSettings_NoFields(t *testing.T) {
	repo := newMockSettingsRepo()
	uc := NewUseCase(repo, &mockTelegramValidator{})

	body := []byte(`{}`)
	_, err := uc.UpdateSettings(context.Background(), uuid.New(), body)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no fields")
}

func TestUseCase_UpdateSettings_InvalidJSON(t *testing.T) {
	repo := newMockSettingsRepo()
	uc := NewUseCase(repo, &mockTelegramValidator{})

	_, err := uc.UpdateSettings(context.Background(), uuid.New(), []byte("bad"))
	assert.Error(t, err)
}

func TestUseCase_GetStoredIMAPPassword(t *testing.T) {
	repo := newMockSettingsRepo()
	uc := NewUseCase(repo, &mockTelegramValidator{})

	pwd, err := uc.GetStoredIMAPPassword(context.Background(), uuid.New())
	require.NoError(t, err)
	assert.Equal(t, "secret-password", pwd)
}

func TestUseCase_GetStoredResendKey(t *testing.T) {
	repo := newMockSettingsRepo()
	uc := NewUseCase(repo, &mockTelegramValidator{})

	key, err := uc.GetStoredResendKey(context.Background(), uuid.New())
	require.NoError(t, err)
	assert.Equal(t, "re_api_key_12345", key)
}

func TestUseCase_GetStoredAISettings(t *testing.T) {
	repo := newMockSettingsRepo()
	uc := NewUseCase(repo, &mockTelegramValidator{})

	ai, err := uc.GetStoredAISettings(context.Background(), uuid.New())
	require.NoError(t, err)
	assert.Equal(t, "openai", ai.Provider)
	assert.Equal(t, "gpt-4o", ai.Model)
	assert.Equal(t, "sk-test-key-12345", ai.APIKey)
}

// --- testIMAP ---

func TestHandler_TestIMAP_Unauthorized(t *testing.T) {
	repo := newMockSettingsRepo()
	r := setupSettingsRouter(repo, &mockTelegramValidator{})
	req := httptest.NewRequest("POST", "/api/settings/test-imap", bytes.NewBufferString(`{}`))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestHandler_TestIMAP_InvalidJSON(t *testing.T) {
	repo := newMockSettingsRepo()
	r := setupSettingsRouter(repo, &mockTelegramValidator{})
	userID := uuid.New()
	req := withUserCtx(httptest.NewRequest("POST", "/api/settings/test-imap", bytes.NewBufferString("bad")), userID)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestHandler_TestIMAP_MissingFields(t *testing.T) {
	repo := newMockSettingsRepo()
	r := setupSettingsRouter(repo, &mockTelegramValidator{})
	userID := uuid.New()
	body := `{"host":"","port":"","user":"","password":""}`
	req := withUserCtx(httptest.NewRequest("POST", "/api/settings/test-imap", bytes.NewBufferString(body)), userID)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	var resp map[string]any
	err := json.NewDecoder(rec.Body).Decode(&resp)
	require.NoError(t, err)
	assert.False(t, resp["success"].(bool))
}

func TestHandler_TestIMAP_UseStored(t *testing.T) {
	repo := newMockSettingsRepo()
	r := setupSettingsRouter(repo, &mockTelegramValidator{})
	userID := uuid.New()
	// use_stored=true but missing host/port/user -> fail with missing fields message.
	body := `{"use_stored":true}`
	req := withUserCtx(httptest.NewRequest("POST", "/api/settings/test-imap", bytes.NewBufferString(body)), userID)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

// --- testAI ---

func TestHandler_TestAI_Unauthorized(t *testing.T) {
	repo := newMockSettingsRepo()
	r := setupSettingsRouter(repo, &mockTelegramValidator{})
	req := httptest.NewRequest("POST", "/api/settings/test-ai", bytes.NewBufferString(`{}`))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestHandler_TestAI_InvalidJSON(t *testing.T) {
	repo := newMockSettingsRepo()
	r := setupSettingsRouter(repo, &mockTelegramValidator{})
	userID := uuid.New()
	req := withUserCtx(httptest.NewRequest("POST", "/api/settings/test-ai", bytes.NewBufferString("bad")), userID)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestHandler_TestAI_NoProvider(t *testing.T) {
	repo := newMockSettingsRepo()
	repo.settings.AIProvider = ""
	repo.settings.AIAPIKey = ""
	r := setupSettingsRouter(repo, &mockTelegramValidator{})
	userID := uuid.New()
	body := `{"provider":"","use_stored":true}`
	req := withUserCtx(httptest.NewRequest("POST", "/api/settings/test-ai", bytes.NewBufferString(body)), userID)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	var resp map[string]any
	err := json.NewDecoder(rec.Body).Decode(&resp)
	require.NoError(t, err)
	assert.False(t, resp["success"].(bool))
}

func TestHandler_TestAI_NoTester(t *testing.T) {
	repo := newMockSettingsRepo()
	// aiTester is nil in setupSettingsRouter.
	r := setupSettingsRouter(repo, &mockTelegramValidator{})
	userID := uuid.New()
	body := `{"provider":"openai","api_key":"sk-test"}`
	req := withUserCtx(httptest.NewRequest("POST", "/api/settings/test-ai", bytes.NewBufferString(body)), userID)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	var resp map[string]any
	err := json.NewDecoder(rec.Body).Decode(&resp)
	require.NoError(t, err)
	assert.False(t, resp["success"].(bool))
}

func TestHandler_TestAI_WithTester_Success(t *testing.T) {
	repo := newMockSettingsRepo()
	uc := NewUseCase(repo, &mockTelegramValidator{})
	r := chi.NewRouter()
	tester := func(_ context.Context, provider, model, apiKey string) (string, error) {
		return "OpenAI", nil
	}
	RegisterRoutes(r, uc, tester, nil)

	userID := uuid.New()
	body := `{"provider":"openai","model":"gpt-4o","api_key":"sk-test"}`
	req := withUserCtx(httptest.NewRequest("POST", "/api/settings/test-ai", bytes.NewBufferString(body)), userID)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	var resp map[string]any
	err := json.NewDecoder(rec.Body).Decode(&resp)
	require.NoError(t, err)
	assert.True(t, resp["success"].(bool))
	assert.Contains(t, resp["message"], "OpenAI")
}

func TestHandler_TestAI_WithTester_Error(t *testing.T) {
	repo := newMockSettingsRepo()
	uc := NewUseCase(repo, &mockTelegramValidator{})
	r := chi.NewRouter()
	tester := func(_ context.Context, provider, model, apiKey string) (string, error) {
		return "", fmt.Errorf("connection refused")
	}
	RegisterRoutes(r, uc, tester, nil)

	userID := uuid.New()
	body := `{"provider":"openai","api_key":"sk-test"}`
	req := withUserCtx(httptest.NewRequest("POST", "/api/settings/test-ai", bytes.NewBufferString(body)), userID)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	var resp map[string]any
	err := json.NewDecoder(rec.Body).Decode(&resp)
	require.NoError(t, err)
	assert.False(t, resp["success"].(bool))
}

func TestHandler_TestAI_UseStored(t *testing.T) {
	repo := newMockSettingsRepo()
	r := setupSettingsRouter(repo, &mockTelegramValidator{})
	userID := uuid.New()
	body := `{"use_stored":true}`
	req := withUserCtx(httptest.NewRequest("POST", "/api/settings/test-ai", bytes.NewBufferString(body)), userID)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	// aiTester is nil, so will return "AI test unavailable"
	assert.Equal(t, http.StatusOK, rec.Code)
}

// --- testResend ---

func TestHandler_TestResend_Unauthorized(t *testing.T) {
	repo := newMockSettingsRepo()
	r := setupSettingsRouter(repo, &mockTelegramValidator{})
	req := httptest.NewRequest("POST", "/api/settings/test-resend", bytes.NewBufferString(`{}`))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestHandler_TestResend_InvalidJSON(t *testing.T) {
	repo := newMockSettingsRepo()
	r := setupSettingsRouter(repo, &mockTelegramValidator{})
	userID := uuid.New()
	req := withUserCtx(httptest.NewRequest("POST", "/api/settings/test-resend", bytes.NewBufferString("bad")), userID)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestHandler_TestResend_EmptyKey(t *testing.T) {
	repo := newMockSettingsRepo()
	repo.settings.ResendAPIKey = ""
	r := setupSettingsRouter(repo, &mockTelegramValidator{})
	userID := uuid.New()
	body := `{"api_key":"","use_stored":true}`
	req := withUserCtx(httptest.NewRequest("POST", "/api/settings/test-resend", bytes.NewBufferString(body)), userID)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	var resp map[string]any
	err := json.NewDecoder(rec.Body).Decode(&resp)
	require.NoError(t, err)
	assert.False(t, resp["success"].(bool))
}

// --- testSMTP ---

func TestHandler_TestSMTP_InvalidJSON(t *testing.T) {
	repo := newMockSettingsRepo()
	r := setupSettingsRouter(repo, &mockTelegramValidator{})
	req := httptest.NewRequest("POST", "/api/settings/test-smtp", bytes.NewBufferString("bad"))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestHandler_TestSMTP_MissingHost(t *testing.T) {
	repo := newMockSettingsRepo()
	r := setupSettingsRouter(repo, &mockTelegramValidator{})
	body := `{"host":"","user":"u","password":"p"}`
	req := httptest.NewRequest("POST", "/api/settings/test-smtp", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	var resp map[string]any
	err := json.NewDecoder(rec.Body).Decode(&resp)
	require.NoError(t, err)
	assert.False(t, resp["success"].(bool))
}

func TestHandler_TestSMTP_MissingPassword(t *testing.T) {
	repo := newMockSettingsRepo()
	r := setupSettingsRouter(repo, &mockTelegramValidator{})
	body := `{"host":"h","user":"u","password":""}`
	req := httptest.NewRequest("POST", "/api/settings/test-smtp", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	var resp map[string]any
	err := json.NewDecoder(rec.Body).Decode(&resp)
	require.NoError(t, err)
	assert.False(t, resp["success"].(bool))
}

func TestHandler_TestSMTP_MissingFields(t *testing.T) {
	repo := newMockSettingsRepo()
	r := setupSettingsRouter(repo, &mockTelegramValidator{})
	body := `{"host":"","user":"","password":""}`
	req := httptest.NewRequest("POST", "/api/settings/test-smtp", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	var resp map[string]any
	err := json.NewDecoder(rec.Body).Decode(&resp)
	require.NoError(t, err)
	assert.False(t, resp["success"].(bool))
}

// --- getUsage with counter ---

func TestHandler_GetUsage_WithCounter(t *testing.T) {
	repo := newMockSettingsRepo()
	uc := NewUseCase(repo, &mockTelegramValidator{})
	r := chi.NewRouter()
	counter := func(_ context.Context, _ uuid.UUID) (int, int, error) {
		return 42, 100, nil
	}
	RegisterRoutes(r, uc, nil, counter)

	userID := uuid.New()
	req := withUserCtx(httptest.NewRequest("GET", "/api/usage", nil), userID)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	var resp map[string]any
	err := json.NewDecoder(rec.Body).Decode(&resp)
	require.NoError(t, err)
	assert.Equal(t, float64(42), resp["month_leads"])
	assert.Equal(t, float64(100), resp["total_leads"])
}

func TestHandler_GetUsage_CounterError(t *testing.T) {
	repo := newMockSettingsRepo()
	uc := NewUseCase(repo, &mockTelegramValidator{})
	r := chi.NewRouter()
	counter := func(_ context.Context, _ uuid.UUID) (int, int, error) {
		return 0, 0, fmt.Errorf("db error")
	}
	RegisterRoutes(r, uc, nil, counter)

	userID := uuid.New()
	req := withUserCtx(httptest.NewRequest("GET", "/api/usage", nil), userID)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestHandler_TestIMAP_AllFields_ConnectionFails(t *testing.T) {
	repo := newMockSettingsRepo()
	r := setupSettingsRouter(repo, &mockTelegramValidator{})
	userID := uuid.New()
	body := `{"host":"imap.invalid.test","port":"993","user":"u@test.com","password":"pass"}`
	req := withUserCtx(httptest.NewRequest("POST", "/api/settings/test-imap", bytes.NewBufferString(body)), userID)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	var resp map[string]any
	err := json.NewDecoder(rec.Body).Decode(&resp)
	require.NoError(t, err)
	assert.False(t, resp["success"].(bool))
}

func TestHandler_TestSMTP_DefaultPort(t *testing.T) {
	repo := newMockSettingsRepo()
	r := setupSettingsRouter(repo, &mockTelegramValidator{})
	// port empty -> defaults to 465, connection will fail (no real SMTP)
	body := `{"host":"smtp.invalid.test","port":"","user":"u","password":"p"}`
	req := httptest.NewRequest("POST", "/api/settings/test-smtp", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	var resp map[string]any
	err := json.NewDecoder(rec.Body).Decode(&resp)
	require.NoError(t, err)
	assert.False(t, resp["success"].(bool))
}

func TestHandler_TestSMTP_ConnectionFails(t *testing.T) {
	repo := newMockSettingsRepo()
	r := setupSettingsRouter(repo, &mockTelegramValidator{})
	body := `{"host":"localhost","port":"19999","user":"u","password":"p"}`
	req := httptest.NewRequest("POST", "/api/settings/test-smtp", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	var resp map[string]any
	err := json.NewDecoder(rec.Body).Decode(&resp)
	require.NoError(t, err)
	assert.False(t, resp["success"].(bool))
}

func TestHandler_TestResend_WithKey(t *testing.T) {
	repo := newMockSettingsRepo()
	r := setupSettingsRouter(repo, &mockTelegramValidator{})
	userID := uuid.New()
	// Bad key will return 401 from Resend API.
	body := `{"api_key":"re_bad_key_12345"}`
	req := withUserCtx(httptest.NewRequest("POST", "/api/settings/test-resend", bytes.NewBufferString(body)), userID)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	var resp map[string]any
	err := json.NewDecoder(rec.Body).Decode(&resp)
	require.NoError(t, err)
	// Either success=false (bad key) or an error message
	assert.NotNil(t, resp["success"])
}

func TestHandler_TestResend_UseStored(t *testing.T) {
	repo := newMockSettingsRepo()
	r := setupSettingsRouter(repo, &mockTelegramValidator{})
	userID := uuid.New()
	body := `{"use_stored":true}`
	req := withUserCtx(httptest.NewRequest("POST", "/api/settings/test-resend", bytes.NewBufferString(body)), userID)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

// --- UpdateSettings more paths ---

func TestUseCase_UpdateSettings_AllFields(t *testing.T) {
	repo := newMockSettingsRepo()
	uc := NewUseCase(repo, &mockTelegramValidator{})

	body := []byte(`{
		"telegram_bot_token": "valid-token",
		"imap_host": "imap.test.com",
		"imap_port": "993",
		"imap_user": "user@test.com",
		"imap_password": "secret",
		"resend_api_key": "re_key",
		"smtp_host": "smtp.test.com",
		"smtp_port": "465",
		"smtp_user": "smtp_user",
		"smtp_password": "smtp_pass",
		"ai_provider": "openai",
		"ai_model": "gpt-4o",
		"ai_api_key": "sk-key",
		"notify_telegram": true,
		"notify_email_digest": false,
		"auto_qualify": true,
		"auto_draft": true,
		"auto_send": true,
		"auto_send_delay_min": 15,
		"auto_followup": true,
		"auto_followup_days": 3,
		"auto_prospect_to_lead": true,
		"auto_verify_import": false
	}`)
	_, err := uc.UpdateSettings(context.Background(), uuid.New(), body)
	require.NoError(t, err)

	assert.Equal(t, "valid-token", repo.updated["telegram_bot_token"])
	assert.True(t, repo.updated["telegram_bot_active"].(bool))
	assert.Equal(t, "imap.test.com", repo.updated["imap_host"])
	assert.Equal(t, "993", repo.updated["imap_port"])
	assert.Equal(t, "user@test.com", repo.updated["imap_user"])
	assert.Equal(t, "secret", repo.updated["imap_password"])
	assert.Equal(t, "re_key", repo.updated["resend_api_key"])
	assert.Equal(t, "smtp.test.com", repo.updated["smtp_host"])
	assert.Equal(t, "465", repo.updated["smtp_port"])
	assert.Equal(t, "smtp_user", repo.updated["smtp_user"])
	assert.Equal(t, "smtp_pass", repo.updated["smtp_password"])
	assert.Equal(t, "openai", repo.updated["ai_provider"])
	assert.Equal(t, "gpt-4o", repo.updated["ai_model"])
	assert.Equal(t, "sk-key", repo.updated["ai_api_key"])
	assert.True(t, repo.updated["notify_telegram"].(bool))
	assert.False(t, repo.updated["notify_email_digest"].(bool))
	assert.True(t, repo.updated["auto_qualify"].(bool))
	assert.True(t, repo.updated["auto_draft"].(bool))
	assert.True(t, repo.updated["auto_send"].(bool))
	assert.Equal(t, 15, repo.updated["auto_send_delay_min"])
	assert.True(t, repo.updated["auto_followup"].(bool))
	assert.Equal(t, 3, repo.updated["auto_followup_days"])
	assert.True(t, repo.updated["auto_prospect_to_lead"].(bool))
	assert.False(t, repo.updated["auto_verify_import"].(bool))
}

func TestUseCase_UpdateSettings_TelegramBotActiveOnly(t *testing.T) {
	repo := newMockSettingsRepo()
	uc := NewUseCase(repo, &mockTelegramValidator{})

	// Sending telegram_bot_active without token
	body := []byte(`{"telegram_bot_active": false}`)
	_, err := uc.UpdateSettings(context.Background(), uuid.New(), body)
	require.NoError(t, err)
	assert.False(t, repo.updated["telegram_bot_active"].(bool))
}

func TestUseCase_GetStoredAISettings_Error(t *testing.T) {
	repo := newMockSettingsRepo()
	repo.err = fmt.Errorf("db error")
	uc := NewUseCase(repo, &mockTelegramValidator{})

	_, err := uc.GetStoredAISettings(context.Background(), uuid.New())
	assert.Error(t, err)
}

func TestUseCase_GetStoredResendKey_Error(t *testing.T) {
	repo := newMockSettingsRepo()
	repo.err = fmt.Errorf("db error")
	uc := NewUseCase(repo, &mockTelegramValidator{})

	_, err := uc.GetStoredResendKey(context.Background(), uuid.New())
	assert.Error(t, err)
}

func TestHandler_UpdateSettings_RepoUpdateError(t *testing.T) {
	repo := &errorSettingsRepo{
		mockSettingsRepo: *newMockSettingsRepo(),
		updateErr:        fmt.Errorf("database connection lost"),
	}
	uc := NewUseCase(repo, &mockTelegramValidator{})
	r := chi.NewRouter()
	RegisterRoutes(r, uc, nil, nil)

	userID := uuid.New()
	body := `{"ai_provider":"openai"}`
	req := withUserCtx(httptest.NewRequest("PUT", "/api/settings", bytes.NewBufferString(body)), userID)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

// errorSettingsRepo fails on UpdateSettings.
type errorSettingsRepo struct {
	mockSettingsRepo
	updateErr error
}

func (m *errorSettingsRepo) UpdateSettings(_ context.Context, _ uuid.UUID, _ map[string]any) error {
	return m.updateErr
}

func TestHandler_UpdateSettings_ReadBodyError(t *testing.T) {
	repo := newMockSettingsRepo()
	r := setupSettingsRouter(repo, &mockTelegramValidator{})
	userID := uuid.New()

	// Test with nil body (empty reader).
	req := withUserCtx(httptest.NewRequest("PUT", "/api/settings", nil), userID)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

// --- domainToDTO computed: SMTPActive ---

func TestDomainToDTO_SMTPActive(t *testing.T) {
	tests := []struct {
		name     string
		ds       domain.Settings
		expected bool
	}{
		{"all set", domain.Settings{SMTPHost: "smtp.gmail.com", SMTPUser: "u", SMTPPassword: "p"}, true},
		{"missing host", domain.Settings{SMTPUser: "u", SMTPPassword: "p"}, false},
		{"missing user", domain.Settings{SMTPHost: "h", SMTPPassword: "p"}, false},
		{"missing password", domain.Settings{SMTPHost: "h", SMTPUser: "u"}, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dto := domainToDTO(&tc.ds)
			assert.Equal(t, tc.expected, dto.SMTPActive)
		})
	}
}

func TestHandler_TestSMTP_ConnectionRefused(t *testing.T) {
	repo := newMockSettingsRepo()
	r := setupSettingsRouter(repo, &mockTelegramValidator{})
	body := `{"host":"127.0.0.1","port":"19999","user":"u","password":"p"}`
	req := httptest.NewRequest("POST", "/api/settings/test-smtp", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	var resp map[string]any
	err := json.NewDecoder(rec.Body).Decode(&resp)
	require.NoError(t, err)
	assert.False(t, resp["success"].(bool))
}

func TestHandler_UpdateSettings_RepoError(t *testing.T) {
	repo := newMockSettingsRepo()
	// First UpdateSettings call will succeed, but GetSettings after will fail
	r := setupSettingsRouter(repo, &mockTelegramValidator{})
	userID := uuid.New()

	// Set repo error so UpdateSettings in usecase returns non-validation error
	repo.err = fmt.Errorf("db down")
	body := `{"auto_qualify":true}`
	req := withUserCtx(httptest.NewRequest("PUT", "/api/settings", bytes.NewBufferString(body)), userID)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

