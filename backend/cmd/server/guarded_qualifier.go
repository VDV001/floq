package main

import (
	"context"
	"log/slog"

	"github.com/daniil/floq/internal/ai/security"
	"github.com/daniil/floq/internal/inbox"
)

// guardedQualifier decorates an inbox.AIQualifier with the pre-LLM input
// firewall (agent-security-defaults layer 1). The architectural fence sits
// here, in the composition root, BEFORE the inbound payload reaches the
// model — not in the system prompt (manifesto #7, dissociation). The security
// primitives stay context-free domain services in internal/ai/security; this
// decorator is the thin adapter that applies them on the inbox→LLM boundary.
type guardedQualifier struct {
	inner    inbox.AIQualifier
	firewall *security.InputFirewall
	scrubber *security.PIIScrubber
	logger   *slog.Logger
}

func newGuardedQualifier(inner inbox.AIQualifier, firewall *security.InputFirewall, scrubber *security.PIIScrubber, logger *slog.Logger) inbox.AIQualifier {
	return &guardedQualifier{inner: inner, firewall: firewall, scrubber: scrubber, logger: logger}
}

// Qualify scans the inbound first message before delegating. A Block verdict
// short-circuits: the LLM never sees the payload and the lead is flagged for
// manual review instead of being qualified on attacker-controlled text. Info
// and Warn verdicts pass through (Warn is consumed downstream by the tool-call
// firewall, which refuses to fan a warn-flagged input into a destructive send).
func (g *guardedQualifier) Qualify(ctx context.Context, contactName, channel, firstMessage string) (*inbox.QualificationResult, error) {
	scan := g.firewall.Scan(firstMessage)
	if !scan.Allowed {
		g.logger.Warn("input firewall blocked qualification",
			"channel", channel,
			"patterns", scan.MatchedPatterns,
			"reason", scan.Reason)
		return &inbox.QualificationResult{
			Score:             0,
			ScoreReason:       "[security] input blocked by firewall: " + scan.Reason,
			RecommendedAction: "manual_review",
		}, nil
	}
	return g.inner.Qualify(ctx, contactName, channel, firstMessage)
}

func (g *guardedQualifier) ProviderName() string { return g.inner.ProviderName() }
