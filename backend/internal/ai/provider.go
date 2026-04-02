package ai

import "context"

type Message struct {
	Role    string // "system" | "user" | "assistant"
	Content string
}

type CompletionRequest struct {
	Messages  []Message
	MaxTokens int
}

type Provider interface {
	Complete(ctx context.Context, req CompletionRequest) (string, error)
	Name() string
}
