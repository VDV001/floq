package inbox

import "github.com/google/uuid"

// LeadSnippet is the minimal lead context the operator queue needs
// alongside a PendingReply row: who is this draft for, on what
// channel, and how do we identify them. Public fields — this is a
// DTO carried between repository and handler, not a domain entity.
//
// TelegramChatID / EmailAddress are nullable: a Telegram-channel lead
// has no email and vice versa. Callers MUST tolerate either being nil.
type LeadSnippet struct {
	ContactName    string
	Company        string
	Channel        Channel
	TelegramChatID *int64
	EmailAddress   *string
}

// PendingReplyWithLead is the joined read-model returned by the
// operator-queue endpoint. It bundles a PendingReply with just enough
// lead context (LeadSnippet) for the queue page to render contact +
// company without an N+1 fetch on the frontend.
//
// The embedded *PendingReply is the same aggregate exposed by the
// per-lead listing endpoint — keeping the shape identical means the
// frontend can reuse the existing PendingReplyResponse marshalling and
// only widen the wire DTO with a nested lead object.
type PendingReplyWithLead struct {
	Reply *PendingReply
	Lead  LeadSnippet
}

// PendingReplyID is a typed accessor so callers don't have to reach
// through the embedded pointer. Used by the handler when logging.
func (v *PendingReplyWithLead) PendingReplyID() uuid.UUID {
	if v == nil || v.Reply == nil {
		return uuid.Nil
	}
	return v.Reply.ID
}
