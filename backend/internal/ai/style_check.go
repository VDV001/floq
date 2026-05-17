package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	auditdomain "github.com/daniil/floq/internal/audit/domain"
)

// StyleResult holds the outcome of a style-check pass over AI-generated
// outreach copy. Score is 0–10 (10 = lively, personal, no jargon; 0 =
// "corporate" boilerplate that smells of ChatGPT). Issues lists the
// concrete tics that pulled the score down; Feedback is a short
// rewrite-direction sentence the generator can use on retry.
type StyleResult struct {
	Score    int      `json:"score"`
	Issues   []string `json:"issues"`
	Feedback string   `json:"feedback"`
}

// StyleCheckPassThreshold is the minimum score for which the generator
// considers the draft "stylistically acceptable". Anything below triggers
// a single retry with the style-check feedback prepended.
const StyleCheckPassThreshold = 7

// StyleCheck runs a Budget-mode LLM pass that judges whether the draft
// reads as a real person's message or as canned AI boilerplate. The
// channel ("email" | "telegram" | "reply" | "followup") tunes the
// strictness — email tolerates a slightly more formal tone than TG.
//
// Returns a parsed StyleResult. JSON errors and provider errors are
// wrapped with "style check:" so callers can match on substring.
func (c *AIClient) StyleCheck(ctx context.Context, draft, channel string) (*StyleResult, error) {
	user := strings.NewReplacer(
		"{{channel}}", channel,
		"{{draft}}", draft,
	).Replace(StyleCheckUser)

	// Re-tag the audit attribution for this inner call: same user/lead
	// as the parent Generate*, but RequestType=style_check so cost
	// reports can break down "how much of this user's spend is the
	// style critic vs. the actual draft". WithRequestType is a no-op
	// when no parent meta is attached (e.g. tests).
	styleCtx := auditdomain.WithRequestType(ctx, auditdomain.RequestTypeStyleCheck)
	resp, err := c.provider.Complete(styleCtx, CompletionRequest{
		Messages: []Message{
			{Role: "system", Content: StyleCheckSystem},
			{Role: "user", Content: user},
		},
		MaxTokens: 512,
		Mode:      ModelModeBudget,
	})
	if err != nil {
		return nil, fmt.Errorf("style check: %w", err)
	}

	var result StyleResult
	cleaned := extractJSON(resp.Text)
	if err := json.Unmarshal([]byte(cleaned), &result); err != nil {
		return nil, fmt.Errorf("style check: parse response: %w (raw: %s)", err, resp.Text[:min(len(resp.Text), 200)])
	}
	return &result, nil
}

// applyStyleCheck runs the post-generation pipeline: if style-check is
// enabled and the draft scores below StyleCheckPassThreshold, regenFn is
// invoked exactly once with the LLM's feedback string to produce a retry.
//
// Failure modes (style-check error, JSON parse error, provider error in
// the second pass): degrade gracefully and return the original draft.
// Outreach generation must not block on a style-pass outage — the original
// draft is still sendable. Warnings are logged through the client's
// injected logger (see WithLogger).
func (c *AIClient) applyStyleCheck(
	ctx context.Context,
	draft, channel string,
	regenFn func(ctx context.Context, feedback string) (string, error),
) string {
	if !c.styleCheckEnabled {
		return draft
	}
	result, err := c.StyleCheck(ctx, draft, channel)
	if err != nil {
		c.logger.WarnContext(ctx, "style check failed; using original draft",
			"channel", channel, "err", err)
		return draft
	}
	if result.Score >= StyleCheckPassThreshold {
		return draft
	}
	regen, err := regenFn(ctx, result.Feedback)
	if err != nil {
		c.logger.WarnContext(ctx, "style retry failed; using original draft",
			"channel", channel, "err", err)
		return draft
	}
	return regen
}

// retryUserPrompt appends the StyleRetryHint to the user-prompt content of
// the last message in msgs, returning a new slice (msgs is not mutated).
// Used by Generate*'s regenFn closures so they don't each rewrite the
// concatenation by hand.
func retryUserPrompt(msgs []Message, feedback string) []Message {
	out := make([]Message, len(msgs))
	copy(out, msgs)
	hint := strings.ReplaceAll(StyleRetryHint, "{{feedback}}", feedback)
	for i := len(out) - 1; i >= 0; i-- {
		if out[i].Role == "user" {
			out[i].Content = out[i].Content + "\n\n" + hint
			break
		}
	}
	return out
}
