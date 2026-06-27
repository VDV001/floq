package domain

import (
	"strings"
	"testing"
)

func TestSignPayload_Deterministic(t *testing.T) {
	payload := []byte(`{"event":"lead.created","id":"abc"}`)
	secret := "supersecretvalue123"

	a := SignPayload(payload, secret)
	b := SignPayload(payload, secret)
	if a != b {
		t.Fatal("signing must be deterministic for the same payload+secret")
	}
	if !strings.HasPrefix(a, "sha256=") {
		t.Errorf("signature header must be prefixed sha256=, got %q", a)
	}
}

func TestSignPayload_SecretAndPayloadSensitive(t *testing.T) {
	payload := []byte(`{"x":1}`)
	if SignPayload(payload, "secretA1234567") == SignPayload(payload, "secretB1234567") {
		t.Error("different secrets must produce different signatures")
	}
	if SignPayload([]byte(`{"x":1}`), "s1234567890") == SignPayload([]byte(`{"x":2}`), "s1234567890") {
		t.Error("different payloads must produce different signatures")
	}
}

// A receiver verifies by recomputing the HMAC over the raw body under the shared
// secret and comparing to X-Floq-Signature. Mirror that here: the signature only
// matches for the exact (payload, secret) pair.
func TestSignPayload_VerifiableByRecompute(t *testing.T) {
	payload := []byte(`{"event":"lead.qualified"}`)
	secret := "supersecretvalue123"
	sig := SignPayload(payload, secret)

	if sig != SignPayload(payload, secret) {
		t.Error("a freshly produced signature must match a recompute")
	}
	if sig == SignPayload(payload, "wrongsecret123") {
		t.Error("a signature must not match under the wrong secret")
	}
	if sig == SignPayload([]byte(`{"event":"tampered"}`), secret) {
		t.Error("a tampered payload must not match")
	}
}
