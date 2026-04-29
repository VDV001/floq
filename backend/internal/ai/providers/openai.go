package providers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/daniil/floq/internal/ai"
	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	"github.com/openai/openai-go/packages/param"
)

type OpenAIProvider struct {
	client openai.Client
	model  string
}

func NewOpenAIProvider(apiKey, model string, opts ...option.RequestOption) *OpenAIProvider {
	clientOpts := []option.RequestOption{option.WithAPIKey(apiKey)}
	clientOpts = append(clientOpts, opts...)
	return &OpenAIProvider{
		client: openai.NewClient(clientOpts...),
		model:  model,
	}
}

// NewOpenAICompatibleProvider creates an OpenAI-compatible provider with a custom base URL.
// Works with Groq, Together, Fireworks, etc.
func NewOpenAICompatibleProvider(apiKey, model, baseURL string, httpClient *http.Client) *OpenAIProvider {
	opts := []option.RequestOption{option.WithBaseURL(baseURL)}
	if httpClient != nil {
		opts = append(opts, option.WithHTTPClient(httpClient))
	}
	return NewOpenAIProvider(apiKey, model, opts...)
}

func (p *OpenAIProvider) Name() string { return "openai" }

func (p *OpenAIProvider) Complete(ctx context.Context, req ai.CompletionRequest) (string, error) {
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
		return "", fmt.Errorf("openai complete: %w", err)
	}

	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("openai complete: no choices in response")
	}

	content := resp.Choices[0].Message.Content
	if content != "" {
		return content, nil
	}

	// Reasoning models (e.g. gpt-oss on Groq) may return empty content
	// with the actual text in a non-standard "reasoning" field.
	// Extract it from the raw JSON response.
	rawJSON := resp.Choices[0].Message.RawJSON()
	if rawJSON != "" {
		var msgFields struct {
			Reasoning string `json:"reasoning"`
		}
		if err := json.Unmarshal([]byte(rawJSON), &msgFields); err == nil && msgFields.Reasoning != "" {
			return msgFields.Reasoning, nil
		}
	}

	return "", nil
}
