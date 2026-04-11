package chat

import (
	"context"

	"github.com/daniil/floq/internal/ai"
	"github.com/google/uuid"
)

// AIClient abstracts AI completion so the handler doesn't depend on *ai.AIClient.
type AIClient interface {
	Complete(ctx context.Context, req ai.CompletionRequest) (string, error)
}

// StatsReader provides aggregated CRM statistics for the chat system prompt.
type StatsReader interface {
	FetchStats(ctx context.Context, userID uuid.UUID) (*userStats, error)
}
