package main

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

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

