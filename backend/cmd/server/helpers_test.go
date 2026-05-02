package main

import (
	"context"
	"errors"
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

