package onec

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/daniil/floq/internal/integrations/onec/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testCreds(t *testing.T, baseURL string, at domain.AuthType, secret string) *domain.OutboundCredentials {
	t.Helper()
	c, err := domain.NewOutboundCredentials(baseURL, at, secret)
	require.NoError(t, err)
	return c
}

func testDraft(t *testing.T) *domain.CounterpartyDraft {
	t.Helper()
	d, err := domain.NewCounterpartyDraft("Иван", "iv@ex.ru", "ООО Ромашка")
	require.NoError(t, err)
	return d
}

func fastClient() *HTTPClient {
	return NewHTTPClient(http.DefaultClient, WithClientBackoff(time.Millisecond))
}

func TestHTTPClient_CreateCounterparty_Success(t *testing.T) {
	var gotMethod, gotAuth, gotPath string
	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotAuth, gotPath = r.Method, r.Header.Get("Authorization"), r.URL.Path
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &body)
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"Ref_Key":"ctr-123"}`))
	}))
	defer srv.Close()

	ref, err := fastClient().CreateCounterparty(context.Background(),
		testCreds(t, srv.URL, domain.AuthTypeBasic, "dXNlcjpwYXNz"), testDraft(t))

	require.NoError(t, err)
	assert.Equal(t, "ctr-123", ref)
	assert.Equal(t, http.MethodPost, gotMethod)
	assert.Equal(t, "Basic dXNlcjpwYXNz", gotAuth)
	assert.Contains(t, gotPath, "Контрагенты")
	assert.Equal(t, "Иван", body["Description"])
	assert.Equal(t, "iv@ex.ru", body["Email"])
}

func TestHTTPClient_CreateCounterparty_TokenAuth(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"Ref_Key":"x"}`))
	}))
	defer srv.Close()

	_, err := fastClient().CreateCounterparty(context.Background(),
		testCreds(t, srv.URL, domain.AuthTypeToken, "tok-abc"), testDraft(t))

	require.NoError(t, err)
	assert.Equal(t, "Bearer tok-abc", gotAuth)
}

func TestHTTPClient_CreateCounterparty_RetriesThenSucceeds(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt32(&calls, 1) == 1 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"Ref_Key":"ok"}`))
	}))
	defer srv.Close()

	ref, err := fastClient().CreateCounterparty(context.Background(),
		testCreds(t, srv.URL, domain.AuthTypeBasic, "s"), testDraft(t))

	require.NoError(t, err)
	assert.Equal(t, "ok", ref)
	assert.Equal(t, int32(2), atomic.LoadInt32(&calls), "should retry the transient 503 exactly once")
}

func TestHTTPClient_CreateCounterparty_4xxIsTerminal(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer srv.Close()

	_, err := fastClient().CreateCounterparty(context.Background(),
		testCreds(t, srv.URL, domain.AuthTypeBasic, "s"), testDraft(t))

	require.Error(t, err)
	assert.Equal(t, int32(1), atomic.LoadInt32(&calls), "4xx must not be retried")
}

func TestHTTPClient_CreateCounterparty_ExhaustsRetries(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer srv.Close()

	_, err := fastClient().CreateCounterparty(context.Background(),
		testCreds(t, srv.URL, domain.AuthTypeBasic, "s"), testDraft(t))

	require.Error(t, err)
	assert.Equal(t, int32(clientMaxAttempts), atomic.LoadInt32(&calls), "5xx must surface after max attempts")
}

// Compile-time check that the HTTP client satisfies the port.
var _ OneCClient = (*HTTPClient)(nil)
