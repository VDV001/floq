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

// Dispatch sends the reply via the EmailSender port and, only on a
// successful send, writes the outbound message into the inbox
// history. Ordering matters: persisting before sending would risk
// the UI showing a "sent" row for an email that never left the
// server; the reverse risks history loss for a message the customer
// did receive, which we accept as a smaller and more recoverable
// failure mode — mirrors telegramReplyDispatcher's contract.
func (d *emailReplyDispatcher) Dispatch(ctx context.Context, pr *inbox.PendingReply) error {
	if pr.Channel != inbox.ChannelEmail {
		return errors.New("email dispatcher: unsupported channel " + string(pr.Channel))
	}
	lead, err := d.leads.GetLead(ctx, pr.LeadID)
	if err != nil {
		return err
	}
	if lead == nil {
		return errors.New("email dispatcher: lead " + pr.LeadID.String() + " not found")
	}
	if lead.EmailAddress == nil || *lead.EmailAddress == "" {
		return errors.New("email dispatcher: lead " + pr.LeadID.String() + " has no email_address")
	}
	if err := d.sender.SendEmail(ctx, pr.UserID, *lead.EmailAddress, subjectFor(pr.Kind), pr.Body); err != nil {
		return err
	}
	outMsg := inbox.NewInboxMessage(pr.LeadID, inbox.DirectionOutbound, pr.Body)
	return d.inboxRepo.CreateMessage(ctx, outMsg)
}
