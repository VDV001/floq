package providers

import (
	"context"
	"fmt"

	"github.com/daniil/floq/internal/ai"
	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	"github.com/openai/openai-go/packages/param"
)

type OllamaProvider struct {
	client openai.Client
	model  string
}

func NewOllamaProvider(baseURL, model string) *OllamaProvider {
	client := openai.NewClient(
		option.WithBaseURL(baseURL+"/v1"),
		option.WithAPIKey("ollama"),
	)
	return &OllamaProvider{client: client, model: model}
}

func (p *OllamaProvider) Name() string { return "ollama" }

func (p *OllamaProvider) Complete(ctx context.Context, req ai.CompletionRequest) (string, error) {
	var messages []openai.ChatCompletionMessageParamUnion

	for _, msg := range req.Messages {
		switch msg.Role {
		case "system":
			messages = append(messages, openai.SystemMessage(msg.Content))
		case "user":
			messages = append(messages, openai.UserMessage(msg.Content))
		case "assistant":
			messages = append(messages, openai.AssistantMessage(msg.Content))
		}
	}

	resp, err := p.client.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
		Model:     p.model,
		Messages:  messages,
		MaxTokens: param.NewOpt(int64(req.MaxTokens)),
	})
	if err != nil {
		return "", fmt.Errorf("ollama complete: %w", err)
	}

	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("ollama complete: no choices in response")
	}

	return resp.Choices[0].Message.Content, nil
}
