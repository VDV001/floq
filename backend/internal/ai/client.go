package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// extractJSON strips markdown code fences (```json ... ```) that some models wrap around JSON.
func extractJSON(s string) string {
	s = strings.TrimSpace(s)
	// Strip ```json ... ``` or ``` ... ```
	if strings.HasPrefix(s, "```") {
		// Remove opening fence line
		if idx := strings.Index(s, "\n"); idx != -1 {
			s = s[idx+1:]
		}
		// Remove closing fence
		if idx := strings.LastIndex(s, "```"); idx != -1 {
			s = s[:idx]
		}
		s = strings.TrimSpace(s)
	}
	// Also try to extract first { ... } if there's extra text
	start := strings.Index(s, "{")
	end := strings.LastIndex(s, "}")
	if start != -1 && end != -1 && end > start {
		s = s[start : end+1]
	}
	return s
}

type QualificationResult struct {
	IdentifiedNeed    string `json:"identified_need"`
	EstimatedBudget   string `json:"estimated_budget"`
	Deadline          string `json:"deadline"`
	Score             int    `json:"score"`
	ScoreReason       string `json:"score_reason"`
	RecommendedAction string `json:"recommended_action"`
}

type AIClient struct {
	provider      Provider
	bookingLink   string
	senderName    string
	senderCompany string
}

func NewAIClient(provider Provider, bookingLink, senderName, senderCompany string) *AIClient {
	return &AIClient{
		provider:      provider,
		bookingLink:   bookingLink,
		senderName:    senderName,
		senderCompany: senderCompany,
	}
}

func (c *AIClient) resolveSystemPrompt(prompt string) string {
	return strings.ReplaceAll(prompt, "{{booking_link}}", c.bookingLink)
}

func (c *AIClient) resolveSenderVars(prompt string) string {
	r := strings.NewReplacer(
		"{{sender_name}}", c.senderName,
		"{{sender_company}}", c.senderCompany,
	)
	return r.Replace(prompt)
}

func (c *AIClient) ProviderName() string {
	return c.provider.Name()
}

// Complete exposes the underlying provider's Complete method for direct use.
func (c *AIClient) Complete(ctx context.Context, req CompletionRequest) (string, error) {
	return c.provider.Complete(ctx, req)
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
	cleaned := extractJSON(resp)
	if err := json.Unmarshal([]byte(cleaned), &result); err != nil {
		return nil, fmt.Errorf("ai qualify parse response: %w (raw: %s)", err, resp[:min(len(resp), 200)])
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
			{Role: "system", Content: c.resolveSystemPrompt(DraftSystem)},
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

func (c *AIClient) GenerateColdMessage(ctx context.Context, name, title, company, prospectContext, stepHint, previousMessage, source string) (string, error) {
	previousContext := ""
	if previousMessage != "" {
		previousContext = "Предыдущее отправленное сообщение: \"" + previousMessage + "\""
	}

	r := strings.NewReplacer(
		"{{name}}", name,
		"{{title}}", title,
		"{{company}}", company,
		"{{prospect_context}}", prospectContext,
		"{{step_hint}}", stepHint,
		"{{previous_context}}", previousContext,
		"{{source}}", source,
	)
	userPrompt := r.Replace(ColdOutreachUser)

	resp, err := c.provider.Complete(ctx, CompletionRequest{
		Messages: []Message{
			{Role: "system", Content: c.resolveSenderVars(c.resolveSystemPrompt(ColdOutreachSystem))},
			{Role: "user", Content: userPrompt},
		},
		MaxTokens: 2048,
	})
	if err != nil {
		return "", fmt.Errorf("ai cold message: %w", err)
	}
	return resp, nil
}

func (c *AIClient) GenerateTelegramMessage(ctx context.Context, name, title, company, prospectContext, stepHint, previousMessage, source string) (string, error) {
	previousContext := ""
	if previousMessage != "" {
		previousContext = "Предыдущее сообщение: \"" + previousMessage + "\""
	}

	r := strings.NewReplacer(
		"{{name}}", name,
		"{{title}}", title,
		"{{company}}", company,
		"{{prospect_context}}", prospectContext,
		"{{step_hint}}", stepHint,
		"{{previous_context}}", previousContext,
		"{{source}}", source,
	)

	systemPrompt := c.resolveSystemPrompt(TelegramOutreachSystem)
	systemPrompt = c.resolveSenderVars(systemPrompt)

	resp, err := c.provider.Complete(ctx, CompletionRequest{
		Messages: []Message{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: r.Replace(TelegramOutreachUser)},
		},
		MaxTokens: 512,
	})
	if err != nil {
		return "", fmt.Errorf("ai telegram message: %w", err)
	}
	return resp, nil
}

// TelegramReplyResult holds the AI response and whether escalation to a manager is needed.
type TelegramReplyResult struct {
	Text            string
	NeedsEscalation bool
	EscalationNote  string
}

func (c *AIClient) GenerateTelegramReply(ctx context.Context, name, title, company, prospectContext, conversationHistory, lastMessage string) (*TelegramReplyResult, error) {
	r := strings.NewReplacer(
		"{{name}}", name,
		"{{title}}", title,
		"{{company}}", company,
		"{{prospect_context}}", prospectContext,
		"{{conversation_history}}", conversationHistory,
		"{{last_message}}", lastMessage,
	)

	systemPrompt := c.resolveSystemPrompt(TelegramConversationSystem)
	systemPrompt = c.resolveSenderVars(systemPrompt)

	resp, err := c.provider.Complete(ctx, CompletionRequest{
		Messages: []Message{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: r.Replace(TelegramConversationUser)},
		},
		MaxTokens: 512,
	})
	if err != nil {
		return nil, fmt.Errorf("ai telegram reply: %w", err)
	}

	result := &TelegramReplyResult{Text: resp}
	if strings.Contains(resp, "[ТРЕБУЕТСЯ МЕНЕДЖЕР]") {
		result.NeedsEscalation = true
		result.EscalationNote = strings.TrimPrefix(resp, "[ТРЕБУЕТСЯ МЕНЕДЖЕР]")
		result.EscalationNote = strings.TrimSpace(result.EscalationNote)
	}
	return result, nil
}

func (c *AIClient) GenerateCallBrief(ctx context.Context, name, title, company, prospectContext, stepHint, previousMessage string) (string, error) {
	previousContext := ""
	if previousMessage != "" {
		previousContext = "Предыдущее сообщение: \"" + previousMessage + "\""
	}

	r := strings.NewReplacer(
		"{{name}}", name,
		"{{title}}", title,
		"{{company}}", company,
		"{{prospect_context}}", prospectContext,
		"{{step_hint}}", stepHint,
		"{{previous_context}}", previousContext,
	)

	resp, err := c.provider.Complete(ctx, CompletionRequest{
		Messages: []Message{
			{Role: "system", Content: PhoneCallBriefSystem},
			{Role: "user", Content: r.Replace(PhoneCallBriefUser)},
		},
		MaxTokens: 1024,
	})
	if err != nil {
		return "", fmt.Errorf("ai call brief: %w", err)
	}
	return resp, nil
}
