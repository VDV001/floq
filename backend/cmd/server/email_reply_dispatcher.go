package main

import (
	"context"
	"errors"

	"github.com/daniil/floq/internal/inbox"
	leadsdomain "github.com/daniil/floq/internal/leads/domain"
	"github.com/google/uuid"
)

// leadEmailFetcher narrows leadsdomain.Repository to the single
// lookup the email dispatcher needs (the lead's EmailAddress, which
// is stored on Lead, not derivable from a pending reply alone).
type leadEmailFetcher interface {
	GetLead(ctx context.Context, id uuid.UUID) (*leadsdomain.Lead, error)
}

// emailReplyDispatcher delivers an approved PendingReply on the
// email channel via the inbox.EmailSender port (an adapter over the
// outbound package's SMTP/Resend logic) and records the outbound
// message in the inbox history so the operator UI shows the full
// thread — symmetric with telegramReplyDispatcher.
type emailReplyDispatcher struct {
	sender    inbox.EmailSender
	leads     leadEmailFetcher
	inboxRepo inboxMessageWriter
}

func newEmailReplyDispatcher(sender inbox.EmailSender, leads leadEmailFetcher, inboxRepo inboxMessageWriter) *emailReplyDispatcher {
	return &emailReplyDispatcher{sender: sender, leads: leads, inboxRepo: inboxRepo}
}

// subjectFor maps a PendingReplyKind to a customer-visible email
// subject. New kinds added to PendingReplyKind MUST add a case here;
// the default fallback is intentionally generic so a misconfigured
// dispatcher never sends "" as subject.
func subjectFor(kind inbox.PendingReplyKind) string {
	switch kind {
	case inbox.PendingReplyKindBookingLink:
		return "Запись на встречу"
	default:
		return "Сообщение"
	}
}

// Dispatch — stub for the RED step of #53. Real impl lands in the
// matching GREEN commit; tests fail at runtime so the build stays
// bisect-friendly.
func (d *emailReplyDispatcher) Dispatch(_ context.Context, _ *inbox.PendingReply) error {
	return errors.New("emailReplyDispatcher: Dispatch not implemented")
}
