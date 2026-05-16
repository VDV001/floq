package ai

import "context"

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
// This is the RED stub: it returns a high score so downstream callers
// don't trigger a retry. The GREEN commit replaces this with a real
// provider call that parses StyleResult JSON.
func (c *AIClient) StyleCheck(_ context.Context, _ string, _ string) (*StyleResult, error) {
	return &StyleResult{Score: 10}, nil
}

// EnableStyleCheck toggles the post-generation style-check pass on. By
// default AIClient does not perform style checks (additive behaviour).
// The composition root wires this from user settings.
func (c *AIClient) EnableStyleCheck() {
	c.styleCheckEnabled = true
}
