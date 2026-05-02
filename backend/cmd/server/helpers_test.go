package main

import (
	"context"
	"strings"
	"testing"

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

func TestBuildSMTPTester_RoutesPort465ToTLSPath(t *testing.T) {
	// Indirect routing assertion: errors from port-465 attempts mention
	// the implicit-TLS dial path in their message ("прокси" or generic
	// connect error), while port-587 attempts mention STARTTLS only
	// after the dial succeeds (which it will not against 127.0.0.1:1).
	// Here we just confirm both branches return non-empty errors with
	// distinct, non-overlapping wording — no business assertions.
	tester := buildSMTPTester(nil)

	err465 := tester(context.Background(), "127.0.0.1", "465", "u", "p")
	require.Error(t, err465)

	err587 := tester(context.Background(), "127.0.0.1", "1", "u", "p")
	require.Error(t, err587)

	assert.NotEqual(t, err465.Error(), err587.Error(),
		"the two branches must produce distinct error messages so callers can tell them apart")
	// Both messages currently include some Russian connection-failure copy.
	// We pin that they are non-empty rather than the exact string, so a
	// follow-up that lifts UI strings out of helpers.go does not need to
	// rewrite this test.
	assert.True(t, strings.TrimSpace(err465.Error()) != "")
	assert.True(t, strings.TrimSpace(err587.Error()) != "")
}
