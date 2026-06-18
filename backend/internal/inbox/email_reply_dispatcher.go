package inbox

import (
	"context"
	"errors"
)

// emailReplyDispatcher delivers an approved PendingReply on the
// email channel via the EmailSender port (an adapter over the
// outbound package's SMTP/Resend logic) and records the outbound
// message in the inbox history so the operator UI shows the full
// thread — symmetric with telegramReplyDispatcher.
type emailReplyDispatcher struct {
	sender    EmailSender
	targets   ReplyTargetLookup
	inboxRepo inboxMessageWriter
}

// NewEmailReplyDispatcher builds the email reply dispatcher. The sender and
// inbox message writer are supplied by the composition root; targets resolves
// the lead's email address without the inbox context importing the leads
// domain.
func NewEmailReplyDispatcher(sender EmailSender, targets ReplyTargetLookup, inboxRepo inboxMessageWriter) ReplyDispatcher {
	return &emailReplyDispatcher{sender: sender, targets: targets, inboxRepo: inboxRepo}
}

// Dispatch sends the reply via the EmailSender port and, only on a
// successful send, writes the outbound message into the inbox
// history. Ordering matters: persisting before sending would risk
// the UI showing a "sent" row for an email that never left the
// server; the reverse risks history loss for a message the customer
// did receive, which we accept as a smaller and more recoverable
// failure mode — mirrors telegramReplyDispatcher's contract.
func (d *emailReplyDispatcher) Dispatch(ctx context.Context, pr *PendingReply) error {
	if pr.Channel != ChannelEmail {
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
	if err := d.sender.SendEmail(ctx, pr.UserID, *target.EmailAddress, EmailSubjectFor(pr.Kind), pr.Body); err != nil {
		return err
	}
	outMsg := NewInboxMessage(pr.LeadID, DirectionOutbound, pr.Body)
	return d.inboxRepo.CreateMessage(ctx, outMsg)
}
