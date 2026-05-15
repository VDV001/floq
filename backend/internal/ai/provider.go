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

type Provider interface {
	Complete(ctx context.Context, req CompletionRequest) (string, error)
	Name() string
}
