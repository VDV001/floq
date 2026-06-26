package onec_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/daniil/floq/internal/httputil"
	"github.com/daniil/floq/internal/integrations/onec"
	"github.com/daniil/floq/internal/integrations/onec/domain"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newConfigRouter(store onec.ConfigStore, mapping onec.MappingConfigStore, tester onec.ConnectionTester, gen onec.SecretGenerator) chi.Router {
	uc := onec.NewConfigUseCase(store, mapping, tester, gen)
	r := chi.NewRouter()
	onec.RegisterConfigRoutes(r, onec.NewConfigHandler(uc))
	return r
}

func authedReq(method, target string, userID uuid.UUID, body string) *http.Request {
	var b io.Reader
	if body != "" {
		b = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, target, b)
	return req.WithContext(httputil.WithUserID(req.Context(), userID))
}

func TestConfigHandler_Unauthorized(t *testing.T) {
	r := newConfigRouter(&fakeConfigStore{}, &fakeMappingStore{}, &fakeTester{}, &fakeSecretGen{secret: validWebhook})
	endpoints := []struct{ method, path string }{
		{http.MethodGet, "/api/onec/config"},
		{http.MethodPut, "/api/onec/config"},
		{http.MethodPost, "/api/onec/config/regenerate-webhook"},
		{http.MethodPost, "/api/onec/test"},
		{http.MethodGet, "/api/onec/mapping"},
		{http.MethodPut, "/api/onec/mapping"},
	}
	for _, e := range endpoints {
		t.Run(e.method+" "+e.path, func(t *testing.T) {
			w := httptest.NewRecorder()
			// No user context → unauthenticated.
			r.ServeHTTP(w, httptest.NewRequest(e.method, e.path, strings.NewReader("{}")))
			assert.Equal(t, http.StatusUnauthorized, w.Code)
		})
	}
}

func TestConfigHandler_GetConfig_MasksSecret(t *testing.T) {
	store := &fakeConfigStore{found: true, cfg: storedCfg(t, "https://1c.example.com", domain.AuthTypeToken, "abcdef123456", validWebhook, true)}
	r := newConfigRouter(store, &fakeMappingStore{}, &fakeTester{}, &fakeSecretGen{})

	w := httptest.NewRecorder()
	r.ServeHTTP(w, authedReq(http.MethodGet, "/api/onec/config", uuid.New(), ""))
	require.Equal(t, http.StatusOK, w.Code)

	var got struct {
		BaseURL       string `json:"base_url"`
		AuthType      string `json:"auth_type"`
		AuthSecret    string `json:"auth_secret"`
		WebhookSecret string `json:"webhook_secret"`
		IsActive      bool   `json:"is_active"`
	}
	require.NoError(t, json.NewDecoder(w.Body).Decode(&got))
	assert.Equal(t, "https://1c.example.com", got.BaseURL)
	assert.Equal(t, "token", got.AuthType)
	assert.Equal(t, "...3456", got.AuthSecret)
	assert.NotContains(t, got.AuthSecret, "abcdef")
	assert.True(t, got.IsActive)
}

func TestConfigHandler_PutConfig_InvalidJSON(t *testing.T) {
	r := newConfigRouter(&fakeConfigStore{}, &fakeMappingStore{}, &fakeTester{}, &fakeSecretGen{})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, authedReq(http.MethodPut, "/api/onec/config", uuid.New(), "{not json"))
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestConfigHandler_PutConfig_ActiveWithoutBaseURL(t *testing.T) {
	store := &fakeConfigStore{found: false}
	r := newConfigRouter(store, &fakeMappingStore{}, &fakeTester{}, &fakeSecretGen{secret: validWebhook})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, authedReq(http.MethodPut, "/api/onec/config", uuid.New(), `{"is_active":true}`))
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Equal(t, 0, store.upsertCalls)
}

func TestConfigHandler_PutConfig_Valid(t *testing.T) {
	store := &fakeConfigStore{found: false}
	r := newConfigRouter(store, &fakeMappingStore{}, &fakeTester{}, &fakeSecretGen{secret: validWebhook})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, authedReq(http.MethodPut, "/api/onec/config", uuid.New(), `{"base_url":"https://1c.example.com","auth_type":"basic","auth_secret":"sek"}`))
	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, 1, store.upsertCalls)
	assert.Equal(t, "sek", store.upserted.AuthSecret)
}

func TestConfigHandler_Test(t *testing.T) {
	t.Run("success → 200 success:true", func(t *testing.T) {
		store := &fakeConfigStore{found: true, cfg: storedCfg(t, "https://1c.example.com", domain.AuthTypeBasic, "s", "", true)}
		r := newConfigRouter(store, &fakeMappingStore{}, &fakeTester{}, &fakeSecretGen{})
		w := httptest.NewRecorder()
		r.ServeHTTP(w, authedReq(http.MethodPost, "/api/onec/test", uuid.New(), "{}"))
		require.Equal(t, http.StatusOK, w.Code)
		var got struct {
			Success bool   `json:"success"`
			Error   string `json:"error"`
		}
		require.NoError(t, json.NewDecoder(w.Body).Decode(&got))
		assert.True(t, got.Success)
	})

	t.Run("auth failure → 200 success:false with russian message", func(t *testing.T) {
		store := &fakeConfigStore{found: true, cfg: storedCfg(t, "https://1c.example.com", domain.AuthTypeBasic, "s", "", true)}
		tester := &fakeTester{err: onec.ErrOnecAuth}
		r := newConfigRouter(store, &fakeMappingStore{}, tester, &fakeSecretGen{})
		w := httptest.NewRecorder()
		r.ServeHTTP(w, authedReq(http.MethodPost, "/api/onec/test", uuid.New(), "{}"))
		require.Equal(t, http.StatusOK, w.Code)
		var got struct {
			Success bool   `json:"success"`
			Error   string `json:"error"`
		}
		require.NoError(t, json.NewDecoder(w.Body).Decode(&got))
		assert.False(t, got.Success)
		assert.NotEmpty(t, got.Error)
	})

	t.Run("empty base url → 200 success:false", func(t *testing.T) {
		store := &fakeConfigStore{found: false}
		tester := &fakeTester{}
		r := newConfigRouter(store, &fakeMappingStore{}, tester, &fakeSecretGen{})
		w := httptest.NewRecorder()
		r.ServeHTTP(w, authedReq(http.MethodPost, "/api/onec/test", uuid.New(), "{}"))
		require.Equal(t, http.StatusOK, w.Code)
		var got struct {
			Success bool `json:"success"`
		}
		require.NoError(t, json.NewDecoder(w.Body).Decode(&got))
		assert.False(t, got.Success)
		assert.Equal(t, 0, tester.calls)
	})
}

func TestConfigHandler_RegenerateWebhook(t *testing.T) {
	store := &fakeConfigStore{found: true, cfg: storedCfg(t, "https://1c.example.com", domain.AuthTypeBasic, "s", "", false)}
	r := newConfigRouter(store, &fakeMappingStore{}, &fakeTester{}, &fakeSecretGen{secret: validWebhook})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, authedReq(http.MethodPost, "/api/onec/config/regenerate-webhook", uuid.New(), ""))
	require.Equal(t, http.StatusOK, w.Code)
	var got struct {
		WebhookSecret string `json:"webhook_secret"`
	}
	require.NoError(t, json.NewDecoder(w.Body).Decode(&got))
	assert.Equal(t, validWebhook, got.WebhookSecret, "full secret returned once")
}

func TestConfigHandler_PutMapping(t *testing.T) {
	t.Run("valid → 200", func(t *testing.T) {
		mapping := &fakeMappingStore{}
		r := newConfigRouter(&fakeConfigStore{}, mapping, &fakeTester{}, &fakeSecretGen{})
		body := `{"rules":[{"external_type":"Документ.ОплатаПокупателя","kind":"payment","email_field":"email"}]}`
		w := httptest.NewRecorder()
		r.ServeHTTP(w, authedReq(http.MethodPut, "/api/onec/mapping", uuid.New(), body))
		require.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, 1, mapping.saveCalls)
	})

	t.Run("invalid kind → 400", func(t *testing.T) {
		mapping := &fakeMappingStore{}
		r := newConfigRouter(&fakeConfigStore{}, mapping, &fakeTester{}, &fakeSecretGen{})
		body := `{"rules":[{"external_type":"X","kind":"bogus","email_field":"email"}]}`
		w := httptest.NewRecorder()
		r.ServeHTTP(w, authedReq(http.MethodPut, "/api/onec/mapping", uuid.New(), body))
		assert.Equal(t, http.StatusBadRequest, w.Code)
		assert.Equal(t, 0, mapping.saveCalls)
	})

	t.Run("empty rules → 400", func(t *testing.T) {
		mapping := &fakeMappingStore{}
		r := newConfigRouter(&fakeConfigStore{}, mapping, &fakeTester{}, &fakeSecretGen{})
		w := httptest.NewRecorder()
		r.ServeHTTP(w, authedReq(http.MethodPut, "/api/onec/mapping", uuid.New(), `{"rules":[]}`))
		assert.Equal(t, http.StatusBadRequest, w.Code)
		assert.Equal(t, 0, mapping.saveCalls)
	})
}

func TestConfigHandler_GetMapping(t *testing.T) {
	rule := domain.MappingRule{ExternalType: "Документ.ОплатаПокупателя", Kind: domain.EventKindPayment, EmailField: "email"}
	cfg, err := domain.NewMappingConfig(uuid.New(), []domain.MappingRule{rule})
	require.NoError(t, err)
	mapping := &fakeMappingStore{cfg: cfg}
	r := newConfigRouter(&fakeConfigStore{}, mapping, &fakeTester{}, &fakeSecretGen{})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, authedReq(http.MethodGet, "/api/onec/mapping", uuid.New(), ""))
	require.Equal(t, http.StatusOK, w.Code)
	var got struct {
		Rules []struct {
			ExternalType string `json:"external_type"`
			Kind         string `json:"kind"`
		} `json:"rules"`
	}
	require.NoError(t, json.NewDecoder(w.Body).Decode(&got))
	require.Len(t, got.Rules, 1)
	assert.Equal(t, "payment", got.Rules[0].Kind)
}
