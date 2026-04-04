package leads

import (
	"context"

	"github.com/daniil/floq/internal/ai"
	"github.com/daniil/floq/internal/leads/domain"
)

// AIAdapter implements domain.AIService by wrapping *ai.AIClient.
type AIAdapter struct {
	client *ai.AIClient
}

// NewAIAdapter creates a new AIAdapter wrapping the given AI client.
func NewAIAdapter(client *ai.AIClient) *AIAdapter {
	return &AIAdapter{client: client}
}

// ProviderName returns the name of the underlying AI provider.
func (a *AIAdapter) ProviderName() string {
	return a.client.ProviderName()
}

// Qualify calls the AI client's Qualify and maps the result to a domain.Qualification.
func (a *AIAdapter) Qualify(ctx context.Context, contactName string, channel domain.Channel, firstMessage string) (*domain.Qualification, error) {
	result, err := a.client.Qualify(ctx, contactName, string(channel), firstMessage)
	if err != nil {
		return nil, err
	}
	return &domain.Qualification{
		IdentifiedNeed:    result.IdentifiedNeed,
		EstimatedBudget:   result.EstimatedBudget,
		Deadline:          result.Deadline,
		Score:             result.Score,
		ScoreReason:       result.ScoreReason,
		RecommendedAction: result.RecommendedAction,
		ProviderUsed:      a.client.ProviderName(),
	}, nil
}

// DraftReply generates a reply draft using the AI client.
func (a *AIAdapter) DraftReply(ctx context.Context, contactName string, firstMessage string) (string, error) {
	return a.client.DraftReply(ctx, contactName, "", "", firstMessage, "{}")
}

// GenerateFollowup generates a follow-up message using the AI client.
func (a *AIAdapter) GenerateFollowup(ctx context.Context, contactName string, company string, days int) (string, error) {
	return a.client.GenerateFollowup(ctx, contactName, company, "", "", "")
}
