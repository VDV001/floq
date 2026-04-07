package sequences

import (
	"context"

	"github.com/daniil/floq/internal/ai"
)

// AIMessageGeneratorAdapter adapts *ai.AIClient to domain.AIMessageGenerator.
type AIMessageGeneratorAdapter struct {
	client *ai.AIClient
}

// NewAIMessageGeneratorAdapter creates a new adapter.
func NewAIMessageGeneratorAdapter(client *ai.AIClient) *AIMessageGeneratorAdapter {
	return &AIMessageGeneratorAdapter{client: client}
}

func (a *AIMessageGeneratorAdapter) GenerateColdMessage(ctx context.Context, name, title, company, prospectContext, stepHint, previousMessage, source, feedbackExamples string) (string, error) {
	return a.client.GenerateColdMessage(ctx, name, title, company, prospectContext, stepHint, previousMessage, source, feedbackExamples)
}

func (a *AIMessageGeneratorAdapter) GenerateTelegramMessage(ctx context.Context, name, title, company, prospectContext, stepHint, previousMessage, source, feedbackExamples string) (string, error) {
	return a.client.GenerateTelegramMessage(ctx, name, title, company, prospectContext, stepHint, previousMessage, source, feedbackExamples)
}

func (a *AIMessageGeneratorAdapter) GenerateCallBrief(ctx context.Context, name, title, company, prospectContext, stepHint, previousMessage string) (string, error) {
	return a.client.GenerateCallBrief(ctx, name, title, company, prospectContext, stepHint, previousMessage)
}
