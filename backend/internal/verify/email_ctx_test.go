package verify

import (
	"context"
	"errors"
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type ctxKey struct{ name string }

// capturingDialer records the context.Context passed to DialContext
// and returns a synthetic dial error so smtpProbe short-circuits before
// any real SMTP work.
type capturingDialer struct {
	capturedCtx context.Context
}

func (d *capturingDialer) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	d.capturedCtx = ctx
	return nil, errors.New("synthetic dial error")
}

func TestSmtpProbe_PropagatesContextToDialer(t *testing.T) {
	dialer := &capturingDialer{}
	key := ctxKey{name: "marker"}
	ctx := context.WithValue(context.Background(), key, "propagated-value")

	_, _, _ = smtpProbe(ctx, "mx.example.invalid", "user@example.invalid", "example.invalid", dialer)

	require.NotNil(t, dialer.capturedCtx, "dialer.DialContext must be invoked")
	assert.Equal(t, "propagated-value", dialer.capturedCtx.Value(key), "ctx must be propagated, not replaced with context.Background()")
}

func TestSmtpProbe_RespectsCanceledContext(t *testing.T) {
	dialer := &capturingDialer{}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, _, _ = smtpProbe(ctx, "mx.example.invalid", "user@example.invalid", "example.invalid", dialer)

	require.NotNil(t, dialer.capturedCtx)
	assert.Error(t, dialer.capturedCtx.Err(), "captured ctx must be the canceled one")
}

func TestVerifyEmail_AcceptsContextAndExplicitDialer(t *testing.T) {
	// Disposable domain short-circuits before any network call,
	// so this test only checks that the new signature compiles
	// and that callers can pass nil dialer explicitly (no variadic).
	result := VerifyEmail(context.Background(), "user@mailinator.com", nil)
	assert.True(t, result.IsValidSyntax)
	assert.True(t, result.IsDisposable)
	assert.Equal(t, "invalid", result.Status)
}
