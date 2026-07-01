package providers

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// rtFunc adapts a function to http.RoundTripper so Claude tests can inject
// canned responses without a live api.anthropic.com. The Claude
// constructor takes no base URL, so intercepting the transport is the
// hermetic way to probe CheckHealth.
type rtFunc func(*http.Request) (*http.Response, error)

func (f rtFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func cannedResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     http.Header{"Content-Type": []string{"application/json"}},
	}
}

func newClaudeHealthProvider(rt rtFunc) *ClaudeProvider {
	return NewClaudeProvider("test-key", "", &http.Client{Transport: rt})
}

func TestClaudeProvider_CheckHealth_OK_NoError(t *testing.T) {
	var lastPath string
	p := newClaudeHealthProvider(func(r *http.Request) (*http.Response, error) {
		lastPath = r.URL.Path
		return cannedResponse(http.StatusOK, `{"data":[],"has_more":false}`), nil
	})

	err := p.CheckHealth(context.Background())
	require.NoError(t, err, "a 200 from /v1/models must pass the health check")
	assert.Contains(t, lastPath, "/models",
		"the health check must probe the free /models endpoint, not run a generation")
}

func TestClaudeProvider_CheckHealth_401_WrapsErrProviderAuth(t *testing.T) {
	p := newClaudeHealthProvider(func(_ *http.Request) (*http.Response, error) {
		return cannedResponse(http.StatusUnauthorized, `{"type":"error","error":{"type":"authentication_error","message":"invalid x-api-key"}}`), nil
	})

	err := p.CheckHealth(context.Background())
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrProviderAuth),
		"a 401 must wrap ErrProviderAuth; got: %v", err)
}

func TestClaudeProvider_CheckHealth_429_WrapsErrProviderRateLimit(t *testing.T) {
	p := newClaudeHealthProvider(func(_ *http.Request) (*http.Response, error) {
		return cannedResponse(http.StatusTooManyRequests, `{"type":"error","error":{"type":"rate_limit_error","message":"slow down"}}`), nil
	})

	err := p.CheckHealth(context.Background())
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrProviderRateLimit),
		"a 429 must wrap ErrProviderRateLimit; got: %v", err)
}

func TestClaudeProvider_CheckHealth_Transport_WrapsErrProviderUnreachable(t *testing.T) {
	p := newClaudeHealthProvider(func(_ *http.Request) (*http.Response, error) {
		return nil, errors.New("synthetic transport failure")
	})

	err := p.CheckHealth(context.Background())
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrProviderUnreachable),
		"a transport failure must wrap ErrProviderUnreachable; got: %v", err)
}
