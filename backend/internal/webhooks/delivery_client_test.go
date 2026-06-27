package webhooks

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/daniil/floq/internal/webhooks/domain"
)

// allowAll is a permissive egress guard so happy-path tests can reach the
// loopback-bound httptest server (the real guard blocks loopback — that's the
// point, exercised by TestDeliver_SSRFGuardBlocksLoopback).
func allowAll(net.IP) bool { return false }

func TestDeliver_PostsSignedPayload(t *testing.T) {
	var gotSig, gotCT, gotBody, gotEventID string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotSig = r.Header.Get(domain.SignatureHeader)
		gotEventID = r.Header.Get(domain.EventIDHeader)
		gotCT = r.Header.Get("Content-Type")
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := newHTTPDeliveryClientWithGuard(allowAll, 5*time.Second)
	status, err := c.Deliver(context.Background(), srv.URL, []byte(`{"event":"lead.created"}`), "sha256=deadbeef", "evt-123")
	if err != nil {
		t.Fatalf("Deliver: %v", err)
	}
	if status != 200 {
		t.Fatalf("status = %d, want 200", status)
	}
	if gotSig != "sha256=deadbeef" {
		t.Errorf("signature header = %q, want sha256=deadbeef", gotSig)
	}
	if gotEventID != "evt-123" {
		t.Errorf("idempotency header = %q, want evt-123", gotEventID)
	}
	if gotCT != "application/json" {
		t.Errorf("content-type = %q, want application/json", gotCT)
	}
	if gotBody != `{"event":"lead.created"}` {
		t.Errorf("body = %q", gotBody)
	}
}

func TestDeliver_Non2xxIsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	c := newHTTPDeliveryClientWithGuard(allowAll, 5*time.Second)
	status, err := c.Deliver(context.Background(), srv.URL, []byte(`{}`), "sig", "evt-x")
	if err == nil {
		t.Fatal("non-2xx must be an error so the worker retries")
	}
	if status != 500 {
		t.Fatalf("status = %d, want 500 reported", status)
	}
}

// SSRF defense layer 2: even if a hostname passed the VO and resolves to an
// internal IP (DNS rebinding), the dial guard refuses the connection.
func TestDeliver_SSRFGuardBlocksLoopback(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	// Real guard (isBlockedIP) — srv.URL is on 127.0.0.1, which must be blocked.
	c := newHTTPDeliveryClientWithGuard(isBlockedIP, 5*time.Second)
	if _, err := c.Deliver(context.Background(), srv.URL, []byte(`{}`), "sig", "evt-x"); err == nil {
		t.Fatal("delivery to a loopback address must be blocked by the dial guard")
	}
}

func TestIsBlockedIP(t *testing.T) {
	blocked := []string{"127.0.0.1", "::1", "10.0.0.1", "192.168.1.1", "169.254.169.254", "0.0.0.0"}
	for _, s := range blocked {
		if !isBlockedIP(net.ParseIP(s)) {
			t.Errorf("isBlockedIP(%s) = false, want true", s)
		}
	}
	allowed := []string{"8.8.8.8", "1.1.1.1"}
	for _, s := range allowed {
		if isBlockedIP(net.ParseIP(s)) {
			t.Errorf("isBlockedIP(%s) = true, want false", s)
		}
	}
}

func TestDeliver_RejectsHugeResponseBody(t *testing.T) {
	// A misbehaving receiver streams a large body; the client must not buffer it
	// unboundedly. We assert delivery still completes (2xx) and returns promptly.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = io.Copy(w, io.LimitReader(neverEnding{}, 5<<20))
	}))
	defer srv.Close()

	c := newHTTPDeliveryClientWithGuard(allowAll, 5*time.Second)
	status, err := c.Deliver(context.Background(), srv.URL, []byte(`{}`), "sig", "evt-x")
	if err != nil || status != 200 {
		t.Fatalf("Deliver: status=%d err=%v", status, err)
	}
}

type neverEnding struct{}

func (neverEnding) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = 'a'
	}
	return len(p), nil
}
