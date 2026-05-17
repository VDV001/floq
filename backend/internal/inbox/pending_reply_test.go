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

func freshPendingReply(t *testing.T) *PendingReply {
	t.Helper()
	pr, err := NewPendingReply(uuid.New(), uuid.New(), ChannelTelegram, PendingReplyKindBookingLink, "body")
	if err != nil {
		t.Fatalf("unexpected error building fixture: %v", err)
	}
	return pr
}

func TestPendingReply_Approve_FromPending(t *testing.T) {
	pr := freshPendingReply(t)
	at := time.Now().UTC()
	if err := pr.Approve(at); err != nil {
		t.Fatalf("Approve from pending should succeed, got %v", err)
	}
	if pr.Status != PendingReplyStatusApproved {
		t.Errorf("Status = %v, want approved", pr.Status)
	}
	if pr.DecidedAt == nil || !pr.DecidedAt.Equal(at) {
		t.Errorf("DecidedAt = %v, want %v", pr.DecidedAt, at)
	}
}

func TestPendingReply_Reject_FromPending(t *testing.T) {
	pr := freshPendingReply(t)
	at := time.Now().UTC()
	if err := pr.Reject(at); err != nil {
		t.Fatalf("Reject from pending should succeed, got %v", err)
	}
	if pr.Status != PendingReplyStatusRejected {
		t.Errorf("Status = %v, want rejected", pr.Status)
	}
	if pr.DecidedAt == nil || !pr.DecidedAt.Equal(at) {
		t.Errorf("DecidedAt = %v, want %v", pr.DecidedAt, at)
	}
}

func TestPendingReply_MarkSent_FromApproved(t *testing.T) {
	pr := freshPendingReply(t)
	if err := pr.Approve(time.Now().UTC()); err != nil {
		t.Fatalf("Approve setup failed: %v", err)
	}
	sentAt := time.Now().UTC().Add(time.Second)
	if err := pr.MarkSent(sentAt); err != nil {
		t.Fatalf("MarkSent from approved should succeed, got %v", err)
	}
	if pr.Status != PendingReplyStatusSent {
		t.Errorf("Status = %v, want sent", pr.Status)
	}
	if pr.SentAt == nil || !pr.SentAt.Equal(sentAt) {
		t.Errorf("SentAt = %v, want %v", pr.SentAt, sentAt)
	}
}

func TestPendingReply_IllegalTransitions(t *testing.T) {
	cases := []struct {
		name string
		op   func(*PendingReply) error
		seed func(t *testing.T) *PendingReply
	}{
		{
			name: "MarkSent from pending is illegal",
			op:   func(pr *PendingReply) error { return pr.MarkSent(time.Now().UTC()) },
			seed: freshPendingReply,
		},
		{
			name: "Approve from approved is illegal",
			op:   func(pr *PendingReply) error { return pr.Approve(time.Now().UTC()) },
			seed: func(t *testing.T) *PendingReply {
				pr := freshPendingReply(t)
				if err := pr.Approve(time.Now().UTC()); err != nil {
					t.Fatalf("seed Approve failed: %v", err)
				}
				return pr
			},
		},
		{
			name: "Reject from approved is illegal",
			op:   func(pr *PendingReply) error { return pr.Reject(time.Now().UTC()) },
			seed: func(t *testing.T) *PendingReply {
				pr := freshPendingReply(t)
				if err := pr.Approve(time.Now().UTC()); err != nil {
					t.Fatalf("seed Approve failed: %v", err)
				}
				return pr
			},
		},
		{
			name: "Approve from rejected is illegal",
			op:   func(pr *PendingReply) error { return pr.Approve(time.Now().UTC()) },
			seed: func(t *testing.T) *PendingReply {
				pr := freshPendingReply(t)
				if err := pr.Reject(time.Now().UTC()); err != nil {
					t.Fatalf("seed Reject failed: %v", err)
				}
				return pr
			},
		},
		{
			name: "MarkSent from sent is illegal",
			op:   func(pr *PendingReply) error { return pr.MarkSent(time.Now().UTC()) },
			seed: func(t *testing.T) *PendingReply {
				pr := freshPendingReply(t)
				if err := pr.Approve(time.Now().UTC()); err != nil {
					t.Fatalf("seed Approve failed: %v", err)
				}
				if err := pr.MarkSent(time.Now().UTC()); err != nil {
					t.Fatalf("seed MarkSent failed: %v", err)
				}
				return pr
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			pr := tc.seed(t)
			before := pr.Status
			if err := tc.op(pr); !errors.Is(err, ErrPendingReplyInvalidTransition) {
				t.Fatalf("want ErrPendingReplyInvalidTransition, got %v", err)
			}
			if pr.Status != before {
				t.Errorf("status mutated on rejected transition: %v -> %v", before, pr.Status)
			}
		})
	}
}
