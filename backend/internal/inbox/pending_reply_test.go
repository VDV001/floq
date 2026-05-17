package inbox

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestNewPendingReply_RequiresUserID(t *testing.T) {
	_, err := NewPendingReply(uuid.Nil, uuid.New(), ChannelTelegram, PendingReplyKindBookingLink, "body")
	if !errors.Is(err, ErrPendingReplyMissingUser) {
		t.Fatalf("want ErrPendingReplyMissingUser, got %v", err)
	}
}

func TestNewPendingReply_RequiresLeadID(t *testing.T) {
	_, err := NewPendingReply(uuid.New(), uuid.Nil, ChannelTelegram, PendingReplyKindBookingLink, "body")
	if !errors.Is(err, ErrPendingReplyMissingLead) {
		t.Fatalf("want ErrPendingReplyMissingLead, got %v", err)
	}
}

func TestNewPendingReply_RejectsUnknownChannel(t *testing.T) {
	_, err := NewPendingReply(uuid.New(), uuid.New(), Channel("smoke-signal"), PendingReplyKindBookingLink, "body")
	if !errors.Is(err, ErrPendingReplyInvalidChannel) {
		t.Fatalf("want ErrPendingReplyInvalidChannel, got %v", err)
	}
}

func TestNewPendingReply_RejectsUnknownKind(t *testing.T) {
	_, err := NewPendingReply(uuid.New(), uuid.New(), ChannelTelegram, PendingReplyKind("mystery"), "body")
	if !errors.Is(err, ErrPendingReplyInvalidKind) {
		t.Fatalf("want ErrPendingReplyInvalidKind, got %v", err)
	}
}

func TestNewPendingReply_RejectsEmptyBody(t *testing.T) {
	cases := []string{"", "   ", "\t\n"}
	for _, body := range cases {
		t.Run(body, func(t *testing.T) {
			_, err := NewPendingReply(uuid.New(), uuid.New(), ChannelTelegram, PendingReplyKindBookingLink, body)
			if !errors.Is(err, ErrPendingReplyEmptyBody) {
				t.Fatalf("want ErrPendingReplyEmptyBody, got %v", err)
			}
		})
	}
}

func TestNewPendingReply_StartsPendingWithGeneratedIDAndTimestamp(t *testing.T) {
	userID := uuid.New()
	leadID := uuid.New()
	before := time.Now().UTC()

	pr, err := NewPendingReply(userID, leadID, ChannelTelegram, PendingReplyKindBookingLink, "hello")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	after := time.Now().UTC()

	if pr.ID == uuid.Nil {
		t.Fatal("expected generated ID, got Nil UUID")
	}
	if pr.UserID != userID {
		t.Errorf("UserID = %v, want %v", pr.UserID, userID)
	}
	if pr.LeadID != leadID {
		t.Errorf("LeadID = %v, want %v", pr.LeadID, leadID)
	}
	if pr.Channel != ChannelTelegram {
		t.Errorf("Channel = %v, want telegram", pr.Channel)
	}
	if pr.Kind != PendingReplyKindBookingLink {
		t.Errorf("Kind = %v, want booking_link", pr.Kind)
	}
	if pr.Body != "hello" {
		t.Errorf("Body = %q, want hello", pr.Body)
	}
	if pr.Status != PendingReplyStatusPending {
		t.Errorf("Status = %v, want pending", pr.Status)
	}
	if pr.CreatedAt.Before(before) || pr.CreatedAt.After(after) {
		t.Errorf("CreatedAt = %v, expected within [%v, %v]", pr.CreatedAt, before, after)
	}
	if pr.DecidedAt != nil {
		t.Errorf("DecidedAt = %v, want nil for pending reply", pr.DecidedAt)
	}
	if pr.SentAt != nil {
		t.Errorf("SentAt = %v, want nil for pending reply", pr.SentAt)
	}
}

func TestNewPendingReply_TrimsBodyWhitespace(t *testing.T) {
	pr, err := NewPendingReply(uuid.New(), uuid.New(), ChannelTelegram, PendingReplyKindBookingLink, "  hi  ")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.TrimSpace(pr.Body) != pr.Body {
		t.Errorf("Body = %q, expected trimmed value", pr.Body)
	}
	if pr.Body != "hi" {
		t.Errorf("Body = %q, want %q", pr.Body, "hi")
	}
}
