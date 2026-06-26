package ai

// ModelMode declares the *intent* of a completion request, decoupled from
// the concrete LLM model. Providers map each mode to the model best
// suited for that workload. The default zero value is ModelModeExecute,
// matching the most common Floq use case (structured response to a known
// input). Plan-mode is for synthesis-heavy work where reasoning quality
// matters more than latency or cost; Budget-mode is for high-volume
// classification/tagging where cost dominates.
//
// Rationale: hardcoding "claude-sonnet-4-6" or "gpt-4o" at every call
// site couples business logic to specific models — when a new model
// generation lands or pricing shifts, every call site must change.
// Mode is the stable abstraction; the provider's mode→model map is
// the single rotation point. NGirchev cross-review (habr 1034452)
// found that strong models without intent-controlled selection produce
// "architecturally beautiful code that didn't run" — Plan-mode for
// research, Execute-mode for task completion is the empirical fix.
type ModelMode int

const (
	// ModelModeExecute — default. Structured output, response to a known
	// input, latency- and cost-sensitive. Examples: lead qualification,
	// reply drafts, cold outreach generation.
	ModelModeExecute ModelMode = iota

	// ModelModePlan — synthesis, multi-document analysis, outline
	// generation. Higher reasoning bar, accepts higher latency/cost.
	// Examples: call brief from conversation history, objection analysis
	// across an email thread, commercial-proposal outline.
	ModelModePlan

	// ModelModeBudget — bulk simple classification, tagging, style-check
	// passes. Cost dominates quality. Examples: spam/intent classifier,
	// follow-up reminder triage.
	ModelModeBudget
)

// String returns the canonical lowercase name of the mode for logs and
// telemetry. Unknown modes (out-of-range int) return "unknown" rather
// than panicking — callers may be reading from configuration.
func (m ModelMode) String() string {
	switch m {
	case ModelModeExecute:
		return "execute"
	case ModelModePlan:
		return "plan"
	case ModelModeBudget:
		return "budget"
	default:
		return "unknown"
	}
}
