package chat

import (
	"context"

	"github.com/google/uuid"
)

// ChatMessage represents a single message in a chat completion request.
type ChatMessage struct {
	Role    string
	Content string
}

// ChatCompletionRequest is a chat-local request for AI completion.
type ChatCompletionRequest struct {
	Messages  []ChatMessage
	MaxTokens int
}

// AIClient abstracts AI completion so the handler doesn't depend on infrastructure.
type AIClient interface {
	Complete(ctx context.Context, req ChatCompletionRequest) (string, error)
	ProviderName() string
}

// StatsReader provides aggregated CRM statistics for the chat system prompt.
type StatsReader interface {
	FetchStats(ctx context.Context, userID uuid.UUID) (*userStats, error)
}
