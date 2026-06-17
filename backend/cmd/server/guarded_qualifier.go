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
	inner     inbox.AIQualifier
	firewall  *security.InputFirewall
	scrubber  *security.PIIScrubber
	validator *security.OutputValidator
	breaker   *security.CostBreaker
	logger    *slog.Logger
}

func newGuardedQualifier(inner inbox.AIQualifier, firewall *security.InputFirewall, scrubber *security.PIIScrubber, validator *security.OutputValidator, breaker *security.CostBreaker, logger *slog.Logger) inbox.AIQualifier {
	return &guardedQualifier{inner: inner, firewall: firewall, scrubber: scrubber, validator: validator, breaker: breaker, logger: logger}
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
	// Layer 4: cap an oversized payload and gate the per-conversation LLM
	// call budget BEFORE spending a token. A budget trip short-circuits to
	// manual_review so a flood of inbound from one source cannot run the bill
	// away or loop unbounded.
	capped, truncated := g.breaker.CapInput(firstMessage)
	if truncated {
		g.logger.Warn("cost breaker truncated oversized input", "channel", channel)
	}
	key := channel + ":" + contactName
	if !g.breaker.Allow(key) {
		g.logger.Warn("cost breaker tripped, skipping qualification", "channel", channel)
		return &inbox.QualificationResult{
			Score:             0,
			ScoreReason:       "[security] qualification skipped: cost breaker tripped",
			RecommendedAction: "manual_review",
		}, nil
	}

	// Layer 1b: strip PII before the payload reaches the model. Qualification
	// scores need/budget/urgency, not the prospect's real email or phone, so
	// the model only ever sees placeholders. The mapping is intentionally
	// discarded here — the qualification result is internal scoring and needs
	// no re-hydration; draft generation (which does) restores separately.
	scrubbed := g.scrubber.Scrub(capped).Scrubbed
	res, err := g.inner.Qualify(ctx, contactName, channel, scrubbed)
	if err != nil {
		return nil, err
	}

	// Layer 2: validate the model's output before it is trusted downstream —
	// clamp score, redact any leaked PII, gate low-confidence to manual_review.
	verdict := g.validator.Validate(security.QualificationView{
		Score:             res.Score,
		ScoreReason:       res.ScoreReason,
		RecommendedAction: res.RecommendedAction,
	})
	if verdict.Flagged {
		g.logger.Warn("output guardrail corrected qualification",
			"channel", channel, "reasons", verdict.Reasons)
	}
	res.Score = verdict.Score
	res.ScoreReason = verdict.ScoreReason
	res.RecommendedAction = verdict.RecommendedAction
	return res, nil
}

func (g *guardedQualifier) ProviderName() string { return g.inner.ProviderName() }
