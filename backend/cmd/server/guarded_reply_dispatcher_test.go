package main

import (
	"context"
	"errors"
	"testing"

	"github.com/daniil/floq/internal/ai/security"
	"github.com/daniil/floq/internal/inbox"
	"github.com/google/uuid"
)

type spyReplyDispatcher struct {
	calls int
	last  *inbox.PendingReply
	err   error
}

func (s *spyReplyDispatcher) Dispatch(_ context.Context, pr *inbox.PendingReply) error {
	s.calls++
	s.last = pr
	return s.err
}

func guardedReplyFixture(inner inbox.ReplyDispatcher) inbox.ReplyDispatcher {
	fw := security.NewToolCallFirewall(security.ToolCallPolicy{
		KnownActions: []string{"send_email", "send_telegram"},
	})
	return newGuardedReplyDispatcher(inner, fw, quietLogger())
}

// A reply whose inbound trigger was Block-flagged must be refused at
// dispatch — even though it reached the dispatcher (i.e. an operator
// approved it). The blocked payload must never fan out to the customer.
func TestGuardedReplyDispatcher_RefusesBlockSeverity(t *testing.T) {
	spy := &spyReplyDispatcher{}
	g := guardedReplyFixture(spy)

	pr := &inbox.PendingReply{LeadID: uuid.New(), Channel: inbox.ChannelTelegram, InputSeverity: inbox.SeverityBlock}
	err := g.Dispatch(context.Background(), pr)

	if !errors.Is(err, errReplyDispatchBlocked) {
		t.Fatalf("want errReplyDispatchBlocked, got %v", err)
	}
	if spy.calls != 0 {
		t.Fatalf("inner dispatched %d times, want 0 — blocked reply must not be sent", spy.calls)
	}
}

// Info and Warn pass through to the inner dispatcher: Info is benign, and
// Warn's required human-confirm is already satisfied by the operator's
// approval in the HITL queue (the reply only reaches dispatch post-approval).
func TestGuardedReplyDispatcher_PassesInfoAndWarn(t *testing.T) {
	for _, sev := range []inbox.Severity{inbox.SeverityInfo, inbox.SeverityWarn} {
		t.Run(sev.String(), func(t *testing.T) {
			spy := &spyReplyDispatcher{}
			g := guardedReplyFixture(spy)

			pr := &inbox.PendingReply{LeadID: uuid.New(), Channel: inbox.ChannelEmail, InputSeverity: sev}
			if err := g.Dispatch(context.Background(), pr); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if spy.calls != 1 {
				t.Fatalf("inner dispatched %d times, want 1", spy.calls)
			}
		})
	}
}
