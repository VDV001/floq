package domain

import (
	"errors"
	"testing"

	"github.com/google/uuid"
)

func TestSuppressionChannel_IsValid(t *testing.T) {
	valid := []SuppressionChannel{SuppressionChannelEmail, SuppressionChannelTelegram}
	for _, c := range valid {
		if !c.IsValid() {
			t.Errorf("%q should be valid", c)
		}
	}
	if SuppressionChannel("sms").IsValid() {
		t.Error("unknown channel should be invalid")
	}
}

func TestNewSuppression_NormalizesAddress(t *testing.T) {
	userID := uuid.New()

	tests := []struct {
		name    string
		channel SuppressionChannel
		address string
		want    string
	}{
		{"email lowercased and trimmed", SuppressionChannelEmail, "  Bob@Example.COM ", "bob@example.com"},
		{"telegram strips @ and lowercases", SuppressionChannelTelegram, "@BobTG", "bobtg"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, err := NewSuppression(userID, tt.channel, tt.address, "unsubscribe")
			if err != nil {
				t.Fatalf("NewSuppression: %v", err)
			}
			if s.Address != tt.want {
				t.Errorf("address = %q, want %q", s.Address, tt.want)
			}
			if s.Channel != tt.channel {
				t.Errorf("channel = %q, want %q", s.Channel, tt.channel)
			}
			if s.Reason != "unsubscribe" {
				t.Errorf("reason = %q, want %q", s.Reason, "unsubscribe")
			}
			if s.ID == uuid.Nil {
				t.Error("ID should be generated")
			}
			if s.CreatedAt.IsZero() {
				t.Error("CreatedAt should be set")
			}
		})
	}
}

func TestNewSuppression_Invalid(t *testing.T) {
	userID := uuid.New()

	tests := []struct {
		name    string
		userID  uuid.UUID
		channel SuppressionChannel
		address string
		reason  string
		wantErr error
	}{
		{"nil userID", uuid.Nil, SuppressionChannelEmail, "a@b.com", "unsubscribe", nil},
		{"invalid channel", userID, SuppressionChannel("sms"), "a@b.com", "unsubscribe", ErrInvalidSuppressionChannel},
		{"empty address", userID, SuppressionChannelEmail, "", "unsubscribe", ErrSuppressionAddressRequired},
		{"whitespace address", userID, SuppressionChannelEmail, "   ", "unsubscribe", ErrSuppressionAddressRequired},
		{"empty reason", userID, SuppressionChannelEmail, "a@b.com", "", ErrSuppressionReasonRequired},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewSuppression(tt.userID, tt.channel, tt.address, tt.reason)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if tt.wantErr != nil && !errors.Is(err, tt.wantErr) {
				t.Errorf("err = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestNormalizeSuppressionAddress(t *testing.T) {
	if got := NormalizeSuppressionAddress(SuppressionChannelEmail, "  Up@X.io "); got != "up@x.io" {
		t.Errorf("email normalize = %q", got)
	}
	if got := NormalizeSuppressionAddress(SuppressionChannelTelegram, "@Handle"); got != "handle" {
		t.Errorf("telegram normalize = %q", got)
	}
}
