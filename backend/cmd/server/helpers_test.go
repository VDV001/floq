package main

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/daniil/floq/internal/config"
	"github.com/daniil/floq/internal/settings"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// buildSMTPTester returns a closure that performs a real SMTP probe.
// Without a mock SMTP server we can only exercise the dial-error path;
// these tests pin that the closure handles unreachable hosts on both
// branches (port 465 implicit-TLS vs other-port STARTTLS) and surfaces
// an error rather than panicking or returning nil.

func TestBuildSMTPTester_Port465_DialError(t *testing.T) {
	tester := buildSMTPTester(nil)
	// 127.0.0.1:1 is reserved for tcpmux and reliably refuses connections
	// in test environments.
	err := tester(context.Background(), "127.0.0.1", "465", "user@test", "pass")
	require.Error(t, err, "tester must surface dial failures on port 465")
}

func TestBuildSMTPTester_Port587_DialError(t *testing.T) {
	tester := buildSMTPTester(nil)
	err := tester(context.Background(), "127.0.0.1", "1", "user@test", "pass")
	require.Error(t, err, "tester must surface dial failures on STARTTLS path")
}

func TestBuildSMTPTester_BadHost_NotPanic(t *testing.T) {
	// Defensive: the closure must not panic on a bad host even though
	// JoinHostPort accepts it; the dial layer is the safety net.
	tester := buildSMTPTester(nil)
	err := tester(context.Background(), "this.host.does.not.exist.invalid", "587", "u", "p")
	require.Error(t, err)
	assert.NotEmpty(t, err.Error())
}

func TestBuildSMTPTester_Port465_WrapsErrSMTPDial(t *testing.T) {
	tester := buildSMTPTester(nil)
	err := tester(context.Background(), "127.0.0.1", "465", "u", "p")
	require.Error(t, err)
	assert.True(t, errors.Is(err, settings.ErrSMTPDial),
		"port-465 dial failures must wrap settings.ErrSMTPDial so the settings handler can map to a UI message; got: %v", err)
}

func TestBuildSMTPTester_Port587_WrapsErrSMTPDial(t *testing.T) {
	tester := buildSMTPTester(nil)
	err := tester(context.Background(), "127.0.0.1", "1", "u", "p")
	require.Error(t, err)
	assert.True(t, errors.Is(err, settings.ErrSMTPDial),
		"STARTTLS-path dial failures must also wrap settings.ErrSMTPDial; got: %v", err)
}

// roundTripperFunc adapts a function to http.RoundTripper so tests can
// inject canned responses or transport errors without standing up a real
// server.
type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func TestBuildResendTester_HTTP200_NoError(t *testing.T) {
	client := &http.Client{Transport: roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: 200,
			Body:       io.NopCloser(strings.NewReader(`{"data":[]}`)),
			Header:     http.Header{},
		}, nil
	})}
	tester := buildResendTester(client)
	err := tester(context.Background(), "test-key")
	require.NoError(t, err)
}

func TestBuildResendTester_HTTP401_WrapsErrResendAuth(t *testing.T) {
	client := &http.Client{Transport: roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: 401,
			Body:       io.NopCloser(strings.NewReader(`{"message":"invalid key"}`)),
			Header:     http.Header{},
		}, nil
	})}
	tester := buildResendTester(client)
	err := tester(context.Background(), "bad-key")
	require.Error(t, err)
	assert.True(t, errors.Is(err, settings.ErrResendAuth),
		"non-200 status must wrap settings.ErrResendAuth so the settings handler can map to a UI message; got: %v", err)
}

func TestBuildResendTester_TransportError_WrapsErrResendRequest(t *testing.T) {
	client := &http.Client{Transport: roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		return nil, errors.New("synthetic transport error")
	})}
	tester := buildResendTester(client)
	err := tester(context.Background(), "any-key")
	require.Error(t, err)
	assert.True(t, errors.Is(err, settings.ErrResendRequest),
		"transport-level failures must wrap settings.ErrResendRequest; got: %v", err)
}

func TestBuildResendTester_NoUIStringsLeak(t *testing.T) {
	client := &http.Client{Transport: roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: 401,
			Body:       io.NopCloser(strings.NewReader(``)),
			Header:     http.Header{},
		}, nil
	})}
	tester := buildResendTester(client)
	err := tester(context.Background(), "bad-key")
	require.Error(t, err)
	assert.NotContains(t, err.Error(), "Неверный",
		"helpers.go must not return Russian UI strings — settings/handler.go owns the user copy")
	assert.NotContains(t, err.Error(), "Ошибка",
		"helpers.go must not return Russian UI strings")
}

func TestBuildAITester_UnknownProvider_WrapsErrAIUnknownProvider(t *testing.T) {
	tester := buildAITester(&config.Config{}, nil)
	_, err := tester(context.Background(), "definitely-not-a-real-provider", "model", "key")
	require.Error(t, err)
	assert.True(t, errors.Is(err, settings.ErrAIUnknownProvider),
		"unknown-provider switch default must wrap settings.ErrAIUnknownProvider; got: %v", err)
	assert.NotContains(t, err.Error(), "неизвестный провайдер",
		"helpers.go must not embed Russian copy in the error — handler maps via errors.Is")
}

// TestBuildAITester_Ollama_* verify the Ollama connection test routes
// through the fast HealthChecker probe (GET /api/tags) instead of a full
// generation, and that the composition root maps the provider's typed
// errors into the settings vocabulary so the handler can produce Russian
// copy via errors.Is. An httptest server stands in for a local Ollama.

func TestBuildAITester_Ollama_ModelPresent_NoError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/tags", r.URL.Path,
			"the Ollama connection test must probe /api/tags, not run a generation")
		_, _ = w.Write([]byte(`{"models":[{"name":"gemma3:4b"}]}`))
	}))
	defer srv.Close()

	tester := buildAITester(&config.Config{OllamaBaseURL: srv.URL}, srv.Client())
	name, err := tester(context.Background(), "ollama", "gemma3:4b", "")
	require.NoError(t, err)
	assert.Equal(t, "ollama", name)
}

func TestBuildAITester_Ollama_ModelAbsent_WrapsErrAIModelNotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"models":[{"name":"llama3:8b"}]}`))
	}))
	defer srv.Close()

	tester := buildAITester(&config.Config{OllamaBaseURL: srv.URL}, srv.Client())
	_, err := tester(context.Background(), "ollama", "gemma3:4b", "")
	require.Error(t, err)
	assert.True(t, errors.Is(err, settings.ErrAIModelNotFound),
		"an un-pulled model must map to settings.ErrAIModelNotFound; got: %v", err)
	assert.NotContains(t, err.Error(), "Модель",
		"helpers.go must not embed Russian copy — handler maps via errors.Is")
}

func TestBuildAITester_Ollama_Unreachable_WrapsErrAIUnreachable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {}))
	base := srv.URL
	srv.Close() // address now refuses connections

	tester := buildAITester(&config.Config{OllamaBaseURL: base}, http.DefaultClient)
	_, err := tester(context.Background(), "ollama", "gemma3:4b", "")
	require.Error(t, err)
	assert.True(t, errors.Is(err, settings.ErrAIUnreachable),
		"an unreachable Ollama must map to settings.ErrAIUnreachable; got: %v", err)
}

// TestBuildAITester_Cloud_* verify that cloud providers route their
// connection test through the free /models health probe (no billed
// generation, #235 M3) and that the SDK's typed errors map into the
// settings vocabulary so the handler renders friendly Russian copy.

func TestBuildAITester_OpenAI_Auth401_WrapsErrAIAuth(t *testing.T) {
	var paths []string
	client := &http.Client{Transport: roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		paths = append(paths, r.URL.Path)
		return &http.Response{
			StatusCode: 401,
			Body:       io.NopCloser(strings.NewReader(`{"error":{"message":"invalid key"}}`)),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
		}, nil
	})}
	tester := buildAITester(&config.Config{OpenAIModel: "gpt-4o"}, client)
	_, err := tester(context.Background(), "openai", "gpt-4o", "bad-key")
	require.Error(t, err)
	assert.True(t, errors.Is(err, settings.ErrAIAuth),
		"a 401 from the cloud health probe must map to settings.ErrAIAuth; got: %v", err)
	// Prove the test probed /models and never ran a generation (#235 M3).
	for _, p := range paths {
		assert.NotContains(t, p, "chat/completions",
			"the connection test must not run a generation; hit %q", p)
	}
}

func TestBuildAITester_Claude_RateLimit429_WrapsErrAIRateLimit(t *testing.T) {
	client := &http.Client{Transport: roundTripperFunc(func(_ *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: 429,
			Body:       io.NopCloser(strings.NewReader(`{"type":"error","error":{"type":"rate_limit_error","message":"slow down"}}`)),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
		}, nil
	})}
	tester := buildAITester(&config.Config{}, client)
	_, err := tester(context.Background(), "claude", "claude-sonnet-4-6", "some-key")
	require.Error(t, err)
	assert.True(t, errors.Is(err, settings.ErrAIRateLimit),
		"a 429 from the cloud health probe must map to settings.ErrAIRateLimit; got: %v", err)
	assert.NotContains(t, err.Error(), "Слишком",
		"helpers.go must not embed Russian copy — handler maps via errors.Is")
}

// TestBuildAITester_Gemini/OpenRouter verify the two OpenAI-compatible
// providers (#228) are constructed, route their connection test through
// the free /models health probe, and report their own name (not "openai").

func TestBuildAITester_Gemini_OK_ReportsName(t *testing.T) {
	client := &http.Client{Transport: roundTripperFunc(func(_ *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: 200,
			Body:       io.NopCloser(strings.NewReader(`{"object":"list","data":[]}`)),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
		}, nil
	})}
	tester := buildAITester(&config.Config{}, client)
	name, err := tester(context.Background(), "gemini", "gemini-2.0-flash", "AIza-key")
	require.NoError(t, err)
	assert.Equal(t, "gemini", name, "the Gemini provider must report its own name, not «openai»")
}

func TestBuildAITester_OpenRouter_OK_ReportsName(t *testing.T) {
	client := &http.Client{Transport: roundTripperFunc(func(_ *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: 200,
			Body:       io.NopCloser(strings.NewReader(`{"object":"list","data":[]}`)),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
		}, nil
	})}
	tester := buildAITester(&config.Config{}, client)
	name, err := tester(context.Background(), "openrouter", "openai/gpt-4o-mini", "sk-or-key")
	require.NoError(t, err)
	assert.Equal(t, "openrouter", name, "the OpenRouter provider must report its own name")
}

func TestBuildAIModelLister_OpenAI_MapsModels(t *testing.T) {
	client := &http.Client{Transport: roundTripperFunc(func(_ *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: 200,
			Body:       io.NopCloser(strings.NewReader(`{"object":"list","data":[{"id":"gpt-4o","object":"model","created":1,"owned_by":"openai"}]}`)),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
		}, nil
	})}
	lister := buildAIModelLister(&config.Config{}, client)
	models, err := lister(context.Background(), "openai", "gpt-4o", "sk-key")
	require.NoError(t, err)
	require.Len(t, models, 1)
	assert.Equal(t, "gpt-4o", models[0].ID)
}

func TestBuildAIModelLister_UnknownProvider_Errors(t *testing.T) {
	lister := buildAIModelLister(&config.Config{}, nil)
	_, err := lister(context.Background(), "definitely-not-real", "", "")
	require.Error(t, err)
	assert.True(t, errors.Is(err, settings.ErrAIUnknownProvider),
		"unknown provider must wrap settings.ErrAIUnknownProvider; got: %v", err)
}

func TestBuildSMTPTester_NoUIStringsLeak(t *testing.T) {
	// Composition-root helpers must NOT carry user-facing copy.
	// settings/handler.go owns the Russian translation via errors.Is.
	tester := buildSMTPTester(nil)
	err := tester(context.Background(), "127.0.0.1", "1", "u", "p")
	require.Error(t, err)
	assert.NotContains(t, err.Error(), "Не удалось",
		"helpers.go must not return Russian UI strings; this is a Clean Architecture violation")
	assert.NotContains(t, err.Error(), "Ошибка",
		"helpers.go must not return Russian UI strings")
	assert.NotContains(t, err.Error(), "Неверный",
		"helpers.go must not return Russian UI strings")
}

