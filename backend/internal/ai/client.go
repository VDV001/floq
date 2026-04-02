package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

type QualificationResult struct {
	IdentifiedNeed    string `json:"identified_need"`
	EstimatedBudget   string `json:"estimated_budget"`
	Deadline          string `json:"deadline"`
	Score             int    `json:"score"`
	ScoreReason       string `json:"score_reason"`
	RecommendedAction string `json:"recommended_action"`
}

type AIClient struct {
	provider Provider
}

func NewAIClient(provider Provider) *AIClient {
	return &AIClient{provider: provider}
}

func (c *AIClient) ProviderName() string {
	return c.provider.Name()
}

func (c *AIClient) Qualify(ctx context.Context, contactName, channel, firstMessage string) (*QualificationResult, error) {
	r := strings.NewReplacer(
		"{{contact_name}}", contactName,
		"{{channel}}", channel,
		"{{first_message}}", firstMessage,
	)
	userPrompt := r.Replace(QualificationUser)

	resp, err := c.provider.Complete(ctx, CompletionRequest{
		Messages: []Message{
			{Role: "system", Content: QualificationSystem},
			{Role: "user", Content: userPrompt},
		},
		MaxTokens: 1024,
	})
	if err != nil {
		return nil, fmt.Errorf("ai qualify: %w", err)
	}

	var result QualificationResult
	if err := json.Unmarshal([]byte(resp), &result); err != nil {
		return nil, fmt.Errorf("ai qualify parse response: %w", err)
	}
	return &result, nil
}

func (c *AIClient) DraftReply(ctx context.Context, contactName, company, channel, firstMessage, qualificationJSON string) (string, error) {
	r := strings.NewReplacer(
		"{{contact_name}}", contactName,
		"{{company}}", company,
		"{{channel}}", channel,
		"{{first_message}}", firstMessage,
		"{{qualification_json}}", qualificationJSON,
	)
	userPrompt := r.Replace(DraftUser)

	resp, err := c.provider.Complete(ctx, CompletionRequest{
		Messages: []Message{
			{Role: "system", Content: DraftSystem},
			{Role: "user", Content: userPrompt},
		},
		MaxTokens: 1024,
	})
	if err != nil {
		return "", fmt.Errorf("ai draft reply: %w", err)
	}
	return resp, nil
}

func (c *AIClient) GenerateFollowup(ctx context.Context, contactName, company, daysAgo, lastMessage, ourLastReply string) (string, error) {
	r := strings.NewReplacer(
		"{{contact_name}}", contactName,
		"{{company}}", company,
		"{{days_ago}}", daysAgo,
		"{{last_message}}", lastMessage,
		"{{our_last_reply}}", ourLastReply,
	)
	userPrompt := r.Replace(FollowupUser)

	resp, err := c.provider.Complete(ctx, CompletionRequest{
		Messages: []Message{
			{Role: "system", Content: FollowupSystem},
			{Role: "user", Content: userPrompt},
		},
		MaxTokens: 1024,
	})
	if err != nil {
		return "", fmt.Errorf("ai generate followup: %w", err)
	}
	return resp, nil
}

func (c *AIClient) GenerateColdMessage(ctx context.Context, name, title, company, stepHint, previousMessage string) (string, error) {
	previousContext := ""
	if previousMessage != "" {
		previousContext = "Предыдущее отправленное сообщение: \"" + previousMessage + "\""
	}

	r := strings.NewReplacer(
		"{{name}}", name,
		"{{title}}", title,
		"{{company}}", company,
		"{{step_hint}}", stepHint,
		"{{previous_context}}", previousContext,
	)
	userPrompt := r.Replace(ColdOutreachUser)

	resp, err := c.provider.Complete(ctx, CompletionRequest{
		Messages: []Message{
			{Role: "system", Content: ColdOutreachSystem},
			{Role: "user", Content: userPrompt},
		},
		MaxTokens: 1024,
	})
	if err != nil {
		return "", fmt.Errorf("ai cold message: %w", err)
	}
	return resp, nil
}
