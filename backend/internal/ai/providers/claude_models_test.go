package providers

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClaudeProvider_ListModels_ReturnsIDsWithDisplayName(t *testing.T) {
	p := newClaudeHealthProvider(func(_ *http.Request) (*http.Response, error) {
		return cannedResponse(http.StatusOK, `{"data":[
			{"id":"claude-sonnet-4-6","type":"model","display_name":"Claude Sonnet 4.6","created_at":"2025-01-01T00:00:00Z"},
			{"id":"claude-opus-4-7","type":"model","display_name":"Claude Opus 4.7","created_at":"2025-01-01T00:00:00Z"}
		],"has_more":false}`), nil
	})

	models, err := p.ListModels(context.Background())
	require.NoError(t, err)
	require.Len(t, models, 2)
	assert.Equal(t, "claude-sonnet-4-6", models[0].ID)
	assert.Equal(t, "Claude Sonnet 4.6", models[0].Meta, "display name surfaced as Meta")
}

func TestClaudeProvider_ListModels_Transport_Errors(t *testing.T) {
	p := newClaudeHealthProvider(func(_ *http.Request) (*http.Response, error) {
		return nil, assert.AnError
	})
	_, err := p.ListModels(context.Background())
	require.Error(t, err)
}
