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

// HealthChecker is an optional capability a Provider may implement when a
// full Complete() round-trip is a poor connection test. Local back-ends
// (Ollama) load the model into memory on the first generation, so a
// "test connection" that generates trips the cold-start timeout even
// when the server is healthy — CheckHealth offers a cheap liveness probe
// instead. Callers type-assert for it and fall back to Complete when a
// provider does not implement it.
type HealthChecker interface {
	CheckHealth(ctx context.Context) error
}

// ModelInfo is one selectable model as surfaced to the settings UI.
// Meta is an optional short descriptor (e.g. Ollama's parameter size
// "4B") shown next to the id; it is empty for providers whose list
// endpoint returns only an identifier (the OpenAI-compatible /models).
type ModelInfo struct {
	ID   string
	Meta string
}

// ModelLister is an optional capability a Provider may implement to
// enumerate the models available for the configured credentials, so the
// settings form can offer a searchable picker instead of free-text entry
// (#229). Callers type-assert for it and fall back to manual entry when a
// provider does not implement it or the call fails.
type ModelLister interface {
	ListModels(ctx context.Context) ([]ModelInfo, error)
}
