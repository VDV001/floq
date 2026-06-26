package domain

import (
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"
)

const testSecret = "test-signing-secret"

func TestUnsubscribeToken_RoundTrip(t *testing.T) {
	id := uuid.New()
	token := SignUnsubscribeToken(id, testSecret)
	if token == "" {
		t.Fatal("expected non-empty token")
	}

	got, err := ParseUnsubscribeToken(token, testSecret)
	if err != nil {
		t.Fatalf("parse valid token: %v", err)
	}
	if got != id {
		t.Errorf("round-trip id = %s, want %s", got, id)
	}
}

func TestUnsubscribeToken_Deterministic(t *testing.T) {
	// A stable token per (prospect, secret) lets the same List-Unsubscribe
	// link survive across resends without storing it.
	id := uuid.New()
	first := SignUnsubscribeToken(id, testSecret)
	second := SignUnsubscribeToken(id, testSecret)
	if first != second {
		t.Error("token should be deterministic for the same prospect and secret")
	}
}

func TestUnsubscribeToken_DistinctPerProspect(t *testing.T) {
	a := SignUnsubscribeToken(uuid.New(), testSecret)
	b := SignUnsubscribeToken(uuid.New(), testSecret)
	if a == b {
		t.Error("different prospects must produce different tokens")
	}
}

func TestUnsubscribeToken_URLSafe(t *testing.T) {
	token := SignUnsubscribeToken(uuid.New(), testSecret)
	// No characters that would need percent-encoding in a path segment.
	if strings.ContainsAny(token, "+/= ") {
		t.Errorf("token %q contains non-URL-safe characters", token)
	}
}

func TestUnsubscribeToken_WrongSecret(t *testing.T) {
	id := uuid.New()
	token := SignUnsubscribeToken(id, testSecret)
	if _, err := ParseUnsubscribeToken(token, "other-secret"); !errors.Is(err, ErrInvalidUnsubscribeToken) {
		t.Errorf("err = %v, want ErrInvalidUnsubscribeToken", err)
	}
}

func TestUnsubscribeToken_Tampered(t *testing.T) {
	id := uuid.New()
	token := SignUnsubscribeToken(id, testSecret)
	// Flip the FIRST character of the signature part. The last base64 char of
	// a 32-byte RawURLEncoding only carries padding bits, so flipping it can
	// decode to the same bytes (the source of an earlier flaky failure); the
	// first signature char always encodes significant bits, so changing it is
	// a guaranteed real tamper.
	dot := strings.IndexByte(token, '.')
	sigStart := dot + 1
	flipped := byte('A')
	if token[sigStart] == 'A' {
		flipped = 'B'
	}
	tampered := token[:sigStart] + string(flipped) + token[sigStart+1:]
	if _, err := ParseUnsubscribeToken(tampered, testSecret); !errors.Is(err, ErrInvalidUnsubscribeToken) {
		t.Errorf("err = %v, want ErrInvalidUnsubscribeToken for tampered token", err)
	}
}

func TestUnsubscribeToken_Malformed(t *testing.T) {
	tests := []struct {
		name  string
		token string
	}{
		{"empty", ""},
		{"no separator", "abcdef"},
		{"bad base64 payload", "!!!.QUJD"},
		{"bad base64 signature", "QUJD.!!!"},
		{"too many parts", "a.b.c"},
		{"payload not a uuid", "QUJD.QUJD"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := ParseUnsubscribeToken(tt.token, testSecret); !errors.Is(err, ErrInvalidUnsubscribeToken) {
				t.Errorf("err = %v, want ErrInvalidUnsubscribeToken", err)
			}
		})
	}
}
