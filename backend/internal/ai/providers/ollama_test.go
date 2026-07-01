package providers

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// tagsServer stands up an httptest server that answers GET /api/tags with
// the given JSON body and status. It records the last path it was asked
// for so tests can pin the native (non-/v1) endpoint.
func tagsServer(t *testing.T, status int, body string) (*httptest.Server, *string) {
	t.Helper()
	var lastPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		lastPath = r.URL.Path
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return srv, &lastPath
}

func TestOllamaProvider_CheckHealth_ModelPresent_NoError(t *testing.T) {
	srv, lastPath := tagsServer(t, http.StatusOK, `{"models":[{"name":"gemma3:4b"},{"name":"llama3:8b"}]}`)
	p := NewOllamaProvider(srv.URL, "gemma3:4b", srv.Client())

	err := p.CheckHealth(context.Background())
	require.NoError(t, err, "configured model present in /api/tags must pass the health check")
	assert.Equal(t, "/api/tags", *lastPath, "health check must probe the native /api/tags endpoint, not /v1")
}

func TestOllamaProvider_CheckHealth_ModelAbsent_WrapsErrModelNotFound(t *testing.T) {
	srv, _ := tagsServer(t, http.StatusOK, `{"models":[{"name":"llama3:8b"}]}`)
	p := NewOllamaProvider(srv.URL, "gemma3:4b", srv.Client())

	err := p.CheckHealth(context.Background())
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrOllamaModelNotFound),
		"a reachable server missing the model must wrap ErrOllamaModelNotFound; got: %v", err)
}

func TestOllamaProvider_CheckHealth_ImplicitLatestTag_NoError(t *testing.T) {
	// User configures a bare "gemma3"; Ollama stores it as "gemma3:latest".
	srv, _ := tagsServer(t, http.StatusOK, `{"models":[{"name":"gemma3:latest"}]}`)
	p := NewOllamaProvider(srv.URL, "gemma3", srv.Client())

	err := p.CheckHealth(context.Background())
	require.NoError(t, err, "a bare model name must match its implicit :latest tag")
}

func TestOllamaProvider_CheckHealth_ServerError_WrapsErrUnreachable(t *testing.T) {
	srv, _ := tagsServer(t, http.StatusInternalServerError, `boom`)
	p := NewOllamaProvider(srv.URL, "gemma3:4b", srv.Client())

	err := p.CheckHealth(context.Background())
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrOllamaUnreachable),
		"a non-200 /api/tags response must wrap ErrOllamaUnreachable; got: %v", err)
}

func TestOllamaProvider_CheckHealth_Unreachable_WrapsErrUnreachable(t *testing.T) {
	srv, _ := tagsServer(t, http.StatusOK, `{}`)
	base := srv.URL
	srv.Close() // now the address refuses connections
	p := NewOllamaProvider(base, "gemma3:4b", http.DefaultClient)

	err := p.CheckHealth(context.Background())
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrOllamaUnreachable),
		"a transport failure must wrap ErrOllamaUnreachable; got: %v", err)
}

func TestOllamaModelMatches(t *testing.T) {
	cases := []struct {
		name      string
		installed string
		want      string
		match     bool
	}{
		{"exact tagged", "gemma3:4b", "gemma3:4b", true},
		{"exact bare", "gemma3", "gemma3", true},
		{"implicit latest", "gemma3:latest", "gemma3", true},
		{"different tag", "gemma3:2b", "gemma3:4b", false},
		{"different model", "llama3:8b", "gemma3:4b", false},
		{"bare want vs different tag", "gemma3:4b", "gemma3", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.match, ollamaModelMatches(tc.installed, tc.want))
		})
	}
}
