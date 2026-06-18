package main

import (
	"context"
	"errors"

	"github.com/daniil/floq/internal/inbox"
)

// emailReplyDispatcher delivers an approved PendingReply on the
// email channel via the inbox.EmailSender port (an adapter over the
// outbound package's SMTP/Resend logic) and records the outbound
// message in the inbox history so the operator UI shows the full
// thread — symmetric with telegramReplyDispatcher.
type emailReplyDispatcher struct {
	sender    inbox.EmailSender
	targets   inbox.ReplyTargetLookup
	inboxRepo inboxMessageWriter
}

func newEmailReplyDispatcher(sender inbox.EmailSender, targets inbox.ReplyTargetLookup, inboxRepo inboxMessageWriter) *emailReplyDispatcher {
	return &emailReplyDispatcher{sender: sender, targets: targets, inboxRepo: inboxRepo}
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
	target, err := d.targets.LookupReplyTarget(ctx, pr.LeadID)
	if err != nil {
		return err
	}
	if target == nil {
		return errors.New("email dispatcher: lead " + pr.LeadID.String() + " not found")
	}
	if target.EmailAddress == nil || *target.EmailAddress == "" {
		return errors.New("email dispatcher: lead " + pr.LeadID.String() + " has no email_address")
	}
	if err := d.sender.SendEmail(ctx, pr.UserID, *target.EmailAddress, inbox.EmailSubjectFor(pr.Kind), pr.Body); err != nil {
		return err
	}
	outMsg := inbox.NewInboxMessage(pr.LeadID, inbox.DirectionOutbound, pr.Body)
	return d.inboxRepo.CreateMessage(ctx, outMsg)
}
