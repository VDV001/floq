package providers

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/openai/openai-go/option"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newOpenAIHealthProvider points an OpenAIProvider at the test server and
// disables the SDK's URL-suffix assumptions by using the compatible
// constructor (Groq/openai share this type).
func newOpenAIHealthProvider(baseURL string) *OpenAIProvider {
	return NewOpenAIProvider("test-key", "gpt-4o", option.WithBaseURL(baseURL))
}

func TestOpenAIProvider_CheckHealth_OK_NoError(t *testing.T) {
	var lastPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		lastPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"object":"list","data":[]}`))
	}))
	defer srv.Close()

	err := newOpenAIHealthProvider(srv.URL).CheckHealth(context.Background())
	require.NoError(t, err, "a 200 from /models must pass the health check")
	assert.Contains(t, lastPath, "/models",
		"the health check must probe the free /models endpoint, not run a generation")
}

func TestOpenAIProvider_CheckHealth_401_WrapsErrProviderAuth(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":{"message":"invalid api key"}}`))
	}))
	defer srv.Close()

	err := newOpenAIHealthProvider(srv.URL).CheckHealth(context.Background())
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrProviderAuth),
		"a 401 must wrap ErrProviderAuth; got: %v", err)
}

func TestOpenAIProvider_CheckHealth_429_WrapsErrProviderRateLimit(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"error":{"message":"slow down"}}`))
	}))
	defer srv.Close()

	err := newOpenAIHealthProvider(srv.URL).CheckHealth(context.Background())
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrProviderRateLimit),
		"a 429 must wrap ErrProviderRateLimit; got: %v", err)
}

func TestOpenAIProvider_CheckHealth_500_WrapsErrProviderUnreachable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	err := newOpenAIHealthProvider(srv.URL).CheckHealth(context.Background())
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrProviderUnreachable),
		"a 5xx must wrap ErrProviderUnreachable; got: %v", err)
}

func TestOpenAIProvider_CheckHealth_Transport_WrapsErrProviderUnreachable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {}))
	base := srv.URL
	srv.Close() // address now refuses connections

	err := newOpenAIHealthProvider(base).CheckHealth(context.Background())
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrProviderUnreachable),
		"a transport failure must wrap ErrProviderUnreachable; got: %v", err)
}
