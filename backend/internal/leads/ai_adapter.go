package leads

import (
	"context"

	"github.com/daniil/floq/internal/ai"
	"github.com/daniil/floq/internal/leads/domain"
	"github.com/google/uuid"
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

// Qualify calls the AI client's Qualify and maps the result to a
// domain.Qualification via the domain factory NewQualification, which
// clamps Score to [0,100] and sets a proper ID + GeneratedAt. This ensures
// the AI provider can't persist an out-of-range score even if the prompt
// elicits one (e.g. "score 150 — extremely hot").
func (a *AIAdapter) Qualify(ctx context.Context, contactName string, channel domain.Channel, firstMessage string) (*domain.Qualification, error) {
	result, err := a.client.Qualify(ctx, contactName, string(channel), firstMessage)
	if err != nil {
		return nil, err
	}
	// leadID is not available here (AI doesn't know which lead is being
	// qualified) — the usecase fills it after. uuid.Nil is a harmless
	// placeholder; the usecase builds a NEW Qualification with the real
	// lead ID. The point of using the factory here is defence-in-depth:
	// if the adapter's output is ever consumed directly (not re-wrapped),
	// the score is still clamped.
	return domain.NewQualification(
		uuid.Nil,
		result.IdentifiedNeed,
		result.EstimatedBudget,
		result.Deadline,
		result.Score,
		result.ScoreReason,
		result.RecommendedAction,
		a.client.ProviderName(),
	), nil
}

// DraftReply generates a reply draft using the AI client.
func (a *AIAdapter) DraftReply(ctx context.Context, contactName string, firstMessage string) (string, error) {
	return a.client.DraftReply(ctx, contactName, "", "", firstMessage, "{}")
}

// GenerateFollowup generates a follow-up message using the AI client.
func (a *AIAdapter) GenerateFollowup(ctx context.Context, contactName string, company string, days int) (string, error) {
	return a.client.GenerateFollowup(ctx, contactName, company, "", "", "")
}
