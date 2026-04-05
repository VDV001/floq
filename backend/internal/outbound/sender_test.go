package outbound

import (
	"testing"

	"github.com/google/uuid"
)

func TestNewSender_Fields(t *testing.T) {
	ownerID := uuid.New()
	s := NewSender(nil, ownerID, "key123", "from@test.com", "https://app.test", nil, nil)

	if s.ownerID != ownerID {
		t.Errorf("expected ownerID %s, got %s", ownerID, s.ownerID)
	}
	if s.fallbackKey != "key123" {
		t.Errorf("expected fallbackKey %q, got %q", "key123", s.fallbackKey)
	}
	if s.fromAddress != "from@test.com" {
		t.Errorf("expected fromAddress %q, got %q", "from@test.com", s.fromAddress)
	}
	if s.appBaseURL != "https://app.test" {
		t.Errorf("expected appBaseURL %q, got %q", "https://app.test", s.appBaseURL)
	}
}

func TestNewSender_NilDeps(t *testing.T) {
	s := NewSender(nil, uuid.Nil, "", "", "", nil, nil)
	if s == nil {
		t.Fatal("expected non-nil Sender")
	}
	if s.fallbackKey != "" {
		t.Errorf("expected empty fallbackKey, got %q", s.fallbackKey)
	}
}
