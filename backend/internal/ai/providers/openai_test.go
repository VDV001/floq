package providers

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestOpenAIProvider_AnalyzeImage_Success verifies that AnalyzeImage
// posts a multimodal chat completion with the image inlined as a
// base64 data URI plus the prompt as a text part, and returns the
// assistant content from the response. The httptest server stands in
// for the real OpenAI endpoint so the test stays hermetic.
func TestOpenAIProvider_AnalyzeImage_Success(t *testing.T) {
	var receivedBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		receivedBody = body
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"x","object":"chat.completion","model":"gpt-4o-mini","choices":[{"message":{"role":"assistant","content":"OCR result text"}}]}`))
	}))
	defer srv.Close()

	p := NewOpenAICompatibleProvider("test-key", "gpt-4o-mini", srv.URL+"/", nil)

	got, err := p.AnalyzeImage(context.Background(), []byte("png-bytes"), "image/png", "Transcribe this")
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, "OCR result text", got.Text)
	assert.Equal(t, "gpt-4o-mini", got.Model)

	// Verify request shape: messages[0].content is an array with both an
	// image_url part (data URI) and a text part with the prompt.
	var req map[string]any
	require.NoError(t, json.Unmarshal(receivedBody, &req))
	msgs, _ := req["messages"].([]any)
	require.NotEmpty(t, msgs)
	user, _ := msgs[0].(map[string]any)
	parts, _ := user["content"].([]any)
	require.Len(t, parts, 2, "user content must carry image + text parts")

	var sawImage, sawText bool
	for _, p := range parts {
		m := p.(map[string]any)
		switch m["type"] {
		case "image_url":
			url, _ := m["image_url"].(map[string]any)
			if u, _ := url["url"].(string); strings.HasPrefix(u, "data:image/png;base64,") {
				sawImage = true
			}
		case "text":
			if t, _ := m["text"].(string); strings.Contains(t, "Transcribe this") {
				sawText = true
			}
		}
	}
	assert.True(t, sawImage, "image_url part with data URI not found")
	assert.True(t, sawText, "text part with prompt not found")
}

func TestOpenAIProvider_AnalyzeImage_ProviderError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":{"message":"server boom"}}`))
	}))
	defer srv.Close()

	p := NewOpenAICompatibleProvider("test-key", "gpt-4o-mini", srv.URL+"/", nil)
	_, err := p.AnalyzeImage(context.Background(), []byte("png"), "image/png", "p")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "analyze image")
}
