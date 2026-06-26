package main

import (
	"context"
	"errors"
	"testing"

	"github.com/daniil/floq/internal/inbox"
	leadsdomain "github.com/daniil/floq/internal/leads/domain"
	"github.com/google/uuid"
)

// stubLeadSource is a minimal GetLead provider for the reply-target adapter
// test — kept local so it does not depend on fakes that move out of this
// package alongside the dispatchers.
type stubLeadSource struct {
	lead *leadsdomain.Lead
	err  error
}

func (s *stubLeadSource) GetLead(_ context.Context, _ uuid.UUID) (*leadsdomain.Lead, error) {
	return s.lead, s.err
}

func TestLeadReplyTargetAdapter_MapsTelegramAndEmail(t *testing.T) {
	chatID := int64(12345)
	email := "lead@example.com"
	src := &stubLeadSource{lead: &leadsdomain.Lead{
		ID:             uuid.New(),
		TelegramChatID: &chatID,
		EmailAddress:   &email,
	}}

	got, err := newLeadReplyTargetAdapter(src).LookupReplyTarget(context.Background(), uuid.New())
	if err != nil {
		t.Fatalf("LookupReplyTarget error: %v", err)
	}
	if got == nil {
		t.Fatal("want a ReplyTarget, got nil")
	}
	if got.TelegramChatID == nil || *got.TelegramChatID != chatID {
		t.Errorf("TelegramChatID = %v, want %d", got.TelegramChatID, chatID)
	}
	if got.EmailAddress == nil || *got.EmailAddress != email {
		t.Errorf("EmailAddress = %v, want %q", got.EmailAddress, email)
	}
}

// A missing lead maps to (nil, nil): the adapter does not invent the
// leads-domain not-found sentinel for the inbox context — the dispatcher
// turns the nil target into its own clear error.
func TestLeadReplyTargetAdapter_LeadNotFoundReturnsNilNil(t *testing.T) {
	src := &stubLeadSource{lead: nil}

	got, err := newLeadReplyTargetAdapter(src).LookupReplyTarget(context.Background(), uuid.New())
	if err != nil {
		t.Fatalf("want nil error for missing lead, got %v", err)
	}
	if got != nil {
		t.Errorf("want nil target for missing lead, got %+v", got)
	}
}

func TestLeadReplyTargetAdapter_PropagatesLookupError(t *testing.T) {
	sentinel := errors.New("db down")
	src := &stubLeadSource{err: sentinel}

	_, err := newLeadReplyTargetAdapter(src).LookupReplyTarget(context.Background(), uuid.New())
	if !errors.Is(err, sentinel) {
		t.Fatalf("want propagated lookup error, got %v", err)
	}
}

// Compile-time guard: the adapter must satisfy the inbox port so signature
// drift breaks the build at the wiring edge.
var _ inbox.ReplyTargetLookup = (*leadReplyTargetAdapter)(nil)
