package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/daniil/floq/internal/ai/security"
	"github.com/daniil/floq/internal/inbox"
)

// Compile-time check that the decorator satisfies the inbox port.
var _ inbox.ReplyDispatcher = (*guardedReplyDispatcher)(nil)

// errReplyDispatchBlocked is returned when the tool-call firewall refuses
// to deliver a reply because its inbound trigger was Block-flagged. It is
// a sentinel so the usecase/handler can distinguish a deliberate security
// refusal (which must NOT be retried as-is) from a transient transport
// failure. The usecase wraps dispatch errors with %w, so errors.Is still
// matches through that wrap.
var errReplyDispatchBlocked = errors.New("reply dispatch refused: blocked-severity inbound")

// guardedReplyDispatcher decorates an inbox.ReplyDispatcher with the
// tool-call firewall (agent-security L2), mirroring guardedQualifier on the
// reply-send path. It runs ToolCallFirewall.Inspect on the would-be send,
// keyed by the reply's channel (send_email / send_telegram) and the
// InputSeverity of the inbound message that triggered it, BEFORE delegating
// to the transport.
//
// The reply only reaches dispatch after an operator approved it in the HITL
// queue, so the firewall's RequiresHumanConfirm (Warn) is already satisfied —
// the real new protection is the hard refusal on Block: a payload the input
// firewall blocked must never fan out into a customer-visible message, even
// if an operator rubber-stamps the approval.
type guardedReplyDispatcher struct {
	inner    inbox.ReplyDispatcher
	firewall *security.ToolCallFirewall
	logger   *slog.Logger
}

func newGuardedReplyDispatcher(inner inbox.ReplyDispatcher, firewall *security.ToolCallFirewall, logger *slog.Logger) inbox.ReplyDispatcher {
	return &guardedReplyDispatcher{inner: inner, firewall: firewall, logger: logger}
}

func (g *guardedReplyDispatcher) Dispatch(ctx context.Context, pr *inbox.PendingReply) error {
	decision := g.firewall.Inspect(security.ToolCall{
		Action:        replyActionForChannel(pr.Channel),
		InputSeverity: toSecuritySeverity(pr.InputSeverity),
	})
	if !decision.Allowed {
		g.logger.Warn("reply dispatch refused by tool-call firewall",
			"lead_id", pr.LeadID, "channel", pr.Channel,
			"severity", pr.InputSeverity, "reason", decision.Reason)
		return fmt.Errorf("%w: %s", errReplyDispatchBlocked, decision.Reason)
	}
	if decision.RequiresHumanConfirm {
		// The pending-reply approval queue IS the human confirm — the
		// operator already approved this exact draft. Proceed, but record
		// that the trigger was warn-flagged for the audit trail.
		g.logger.Info("dispatching warn-flagged reply (operator approval satisfies confirm)",
			"lead_id", pr.LeadID, "channel", pr.Channel)
	}
	return g.inner.Dispatch(ctx, pr)
}

// replyActionForChannel maps the delivery channel onto the destructive
// tool-call action the firewall reasons about.
func replyActionForChannel(ch inbox.Channel) string {
	if ch == inbox.ChannelEmail {
		return "send_email"
	}
	return "send_telegram"
}

// toSecuritySeverity translates the inbox severity ladder onto the security
// one (the inverse of mapSecuritySeverity). Unknown values default to Info —
// the firewall then treats them as non-blocking, so a malformed verdict can
// never silently turn into a refusal of a legitimate reply.
func toSecuritySeverity(s inbox.Severity) security.Severity {
	switch s {
	case inbox.SeverityBlock:
		return security.SeverityBlock
	case inbox.SeverityWarn:
		return security.SeverityWarn
	default:
		return security.SeverityInfo
	}
}
