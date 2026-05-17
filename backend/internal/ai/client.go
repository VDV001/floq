package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
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
	provider          Provider
	bookingLink       string
	senderName        string
	senderCompany     string
	senderPhone       string
	senderWebsite     string
	styleCheckEnabled bool
	logger            *slog.Logger
}

// AIClientOption is a functional option for NewAIClient. Options keep the
// configurable bits (style-check toggle, logger) outside the long fixed
// argument list and let callers set them at construction without later
// mutation.
type AIClientOption func(*AIClient)

// WithStyleCheck wires the boot-time style-check preference into the
// client. Pass true to enable the post-generation style pass for all
// outreach Generate* calls, false to leave it off. The value is fixed
// once NewAIClient returns — there is no runtime mutator.
func WithStyleCheck(enabled bool) AIClientOption {
	return func(c *AIClient) { c.styleCheckEnabled = enabled }
}

// WithLogger injects a slog.Logger the client uses for warnings (today
// only the graceful-degradation paths in applyStyleCheck). Defaults to
// slog.Default() when omitted.
func WithLogger(logger *slog.Logger) AIClientOption {
	return func(c *AIClient) { c.logger = logger }
}

func NewAIClient(provider Provider, bookingLink, senderName, senderCompany, senderPhone, senderWebsite string, opts ...AIClientOption) *AIClient {
	c := &AIClient{
		provider:      provider,
		bookingLink:   bookingLink,
		senderName:    senderName,
		senderCompany: senderCompany,
		senderPhone:   senderPhone,
		senderWebsite: senderWebsite,
	}
	for _, opt := range opts {
		opt(c)
	}
	if c.logger == nil {
		c.logger = slog.Default()
	}
	return c
}

func (c *AIClient) resolveSystemPrompt(prompt string) string {
	return strings.ReplaceAll(prompt, "{{booking_link}}", c.bookingLink)
}

func (c *AIClient) resolveSenderVars(prompt string) string {
	r := strings.NewReplacer(
		"{{sender_name}}", c.senderName,
		"{{sender_company}}", c.senderCompany,
		"{{sender_phone}}", c.senderPhone,
		"{{sender_website}}", c.senderWebsite,
	)
	return r.Replace(prompt)
}

func (c *AIClient) ProviderName() string {
	return c.provider.Name()
}

// Complete exposes the underlying provider's Complete method for direct
// use. Returns only the text — call sites that need usage/cost go
// through the audit-aware RecordingProvider wrapper, not the bare
// AIClient.
func (c *AIClient) Complete(ctx context.Context, req CompletionRequest) (string, error) {
	resp, err := c.provider.Complete(ctx, req)
	if err != nil {
		return "", err
	}
	return resp.Text, nil
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
		Mode:      ModelModeExecute,
	})
	if err != nil {
		return nil, fmt.Errorf("ai qualify: %w", err)
	}

	var result QualificationResult
	cleaned := extractJSON(resp.Text)
	if err := json.Unmarshal([]byte(cleaned), &result); err != nil {
		return nil, fmt.Errorf("ai qualify parse response: %w (raw: %s)", err, resp.Text[:min(len(resp.Text), 200)])
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

	req := CompletionRequest{
		Messages: []Message{
			{Role: "system", Content: c.resolveSystemPrompt(DraftSystem)},
			{Role: "user", Content: userPrompt},
		},
		MaxTokens: 1024,
		Mode:      ModelModeExecute,
	}
	resp, err := c.provider.Complete(ctx, req)
	if err != nil {
		return "", fmt.Errorf("ai draft reply: %w", err)
	}
	return c.applyStyleCheck(ctx, resp.Text, "reply", c.retryText(req)), nil
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

	req := CompletionRequest{
		Messages: []Message{
			{Role: "system", Content: FollowupSystem},
			{Role: "user", Content: userPrompt},
		},
		MaxTokens: 1024,
		Mode:      ModelModeExecute,
	}
	resp, err := c.provider.Complete(ctx, req)
	if err != nil {
		return "", fmt.Errorf("ai generate followup: %w", err)
	}
	return c.applyStyleCheck(ctx, resp.Text, "followup", c.retryText(req)), nil
}

func (c *AIClient) GenerateColdMessage(ctx context.Context, name, title, company, prospectContext, stepHint, previousMessage, source, feedbackExamples string) (string, error) {
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

	systemPrompt := c.resolveSenderVars(c.resolveSystemPrompt(ColdOutreachSystem))
	if feedbackExamples != "" {
		systemPrompt += "\n\n" + feedbackExamples
	}

	req := CompletionRequest{
		Messages: []Message{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userPrompt},
		},
		MaxTokens: 2048,
		Mode:      ModelModeExecute,
	}
	resp, err := c.provider.Complete(ctx, req)
	if err != nil {
		return "", fmt.Errorf("ai cold message: %w", err)
	}
	return c.applyStyleCheck(ctx, resp.Text, "email", c.retryText(req)), nil
}

func (c *AIClient) GenerateTelegramMessage(ctx context.Context, name, title, company, prospectContext, stepHint, previousMessage, source, feedbackExamples string) (string, error) {
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
	if feedbackExamples != "" {
		systemPrompt += "\n\n" + feedbackExamples
	}

	req := CompletionRequest{
		Messages: []Message{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: r.Replace(TelegramOutreachUser)},
		},
		MaxTokens: 2048,
		Mode:      ModelModeExecute,
	}
	resp, err := c.provider.Complete(ctx, req)
	if err != nil {
		return "", fmt.Errorf("ai telegram message: %w", err)
	}
	return c.applyStyleCheck(ctx, resp.Text, "telegram", c.retryText(req)), nil
}

// retryText is the shared style-check regenerate callback: it folds the
// reviewer's feedback into the original system+user prompt and asks the
// provider for a second draft, returning the text only (the style-check
// loop has no use for the usage stats — they're already captured by the
// recording layer wrapping each Complete call).
func (c *AIClient) retryText(req CompletionRequest) func(context.Context, string) (string, error) {
	return func(ctx context.Context, feedback string) (string, error) {
		retry := req
		retry.Messages = retryUserPrompt(req.Messages, feedback)
		resp, err := c.provider.Complete(ctx, retry)
		if err != nil {
			return "", err
		}
		return resp.Text, nil
	}
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
		MaxTokens: 2048,
		Mode:      ModelModeExecute,
	})
	if err != nil {
		return nil, fmt.Errorf("ai telegram reply: %w", err)
	}

	text := resp.Text
	result := &TelegramReplyResult{Text: text}
	if strings.Contains(text, "[ТРЕБУЕТСЯ МЕНЕДЖЕР]") {
		result.NeedsEscalation = true
		result.EscalationNote = strings.TrimPrefix(text, "[ТРЕБУЕТСЯ МЕНЕДЖЕР]")
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
		Mode:      ModelModePlan,
	})
	if err != nil {
		return "", fmt.Errorf("ai call brief: %w", err)
	}
	return resp.Text, nil
}
