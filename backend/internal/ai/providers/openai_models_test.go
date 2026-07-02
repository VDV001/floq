package providers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/openai/openai-go/option"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOpenAIProvider_ListModels_ReturnsIDs(t *testing.T) {
	var lastPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		lastPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"object":"list","data":[
			{"id":"gpt-4o","object":"model","created":1,"owned_by":"openai"},
			{"id":"gpt-4o-mini","object":"model","created":2,"owned_by":"openai"}
		]}`))
	}))
	defer srv.Close()

	p := NewOpenAIProvider("k", "gpt-4o", option.WithBaseURL(srv.URL))
	models, err := p.ListModels(context.Background())
	require.NoError(t, err)
	assert.Contains(t, lastPath, "/models", "must query the /models endpoint")

	ids := make([]string, len(models))
	for i, m := range models {
		ids[i] = m.ID
		assert.Empty(t, m.Meta, "OpenAI-compatible /models exposes no meta")
	}
	assert.Equal(t, []string{"gpt-4o", "gpt-4o-mini"}, ids)
}

func TestOpenAIProvider_ListModels_ErrorPropagates(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	base := srv.URL
	srv.Close()

	p := NewOpenAIProvider("k", "gpt-4o", option.WithBaseURL(base))
	_, err := p.ListModels(context.Background())
	require.Error(t, err, "an unreachable back-end must surface an error, not an empty list")
}
