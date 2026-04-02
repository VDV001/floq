package providers

import (
	"context"
	"fmt"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/daniil/floq/internal/ai"
)

type ClaudeProvider struct {
	client anthropic.Client
	model  anthropic.Model
}

func NewClaudeProvider(apiKey string) *ClaudeProvider {
	return &ClaudeProvider{
		client: anthropic.NewClient(option.WithAPIKey(apiKey)),
		model:  anthropic.ModelClaude3_7SonnetLatest,
	}
}

func (p *ClaudeProvider) Name() string { return "claude" }

func (p *ClaudeProvider) Complete(ctx context.Context, req ai.CompletionRequest) (string, error) {
	var system []anthropic.TextBlockParam
	var messages []anthropic.MessageParam

	for _, msg := range req.Messages {
		switch msg.Role {
		case "system":
			system = append(system, anthropic.TextBlockParam{Text: msg.Content})
		case "user":
			messages = append(messages, anthropic.NewUserMessage(anthropic.NewTextBlock(msg.Content)))
		case "assistant":
			messages = append(messages, anthropic.NewAssistantMessage(anthropic.NewTextBlock(msg.Content)))
		}
	}

	resp, err := p.client.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     p.model,
		MaxTokens: int64(req.MaxTokens),
		System:    system,
		Messages:  messages,
	})
	if err != nil {
		return "", fmt.Errorf("claude complete: %w", err)
	}

	if len(resp.Content) == 0 {
		return "", fmt.Errorf("claude complete: empty response")
	}

	return resp.Content[0].AsText().Text, nil
}
