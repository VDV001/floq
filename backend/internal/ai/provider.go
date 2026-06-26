package ai

import "context"

type Message struct {
	Role    string // "system" | "user" | "assistant"
	Content string
}

type CompletionRequest struct {
	Messages  []Message
	MaxTokens int
	// Mode declares the workload intent (Plan/Execute/Budget). Providers
	// map this to a concrete model. Zero value = ModelModeExecute.
	Mode ModelMode
}

// TokenUsage holds the prompt/response token counts the provider
// reported, in the units that provider counts them in (one provider's
// "token" is another's "byte-pair fragment" — we record what we got,
// the audit layer attaches the model so cost math can be applied
// later). Zero values are valid: Ollama and other local back-ends do
// not surface usage, and we don't synthesize numbers we cannot trust.
type TokenUsage struct {
	InputTokens  int
	OutputTokens int
}

// CompletionResult bundles the generated text with the usage stats from
// the same response, so the RecordingProvider decorator has everything
// it needs to compute cost without a second round-trip to the SDK.
type CompletionResult struct {
	Text  string
	Usage TokenUsage
	// Model is the concrete model name actually used (e.g. "gpt-4o-mini"
	// — not the per-mode default). The audit layer keys pricing on this.
	Model string
}

type Provider interface {
	Complete(ctx context.Context, req CompletionRequest) (*CompletionResult, error)
	Name() string
}
