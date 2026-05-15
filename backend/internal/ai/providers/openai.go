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
	// overrideModel, if non-empty, is used for every Complete call
	// regardless of the request's ModelMode (set via user settings
	// AIModel). Empty string means "use the per-mode default map".
	overrideModel string
}

// openaiModelByMode maps workload intent to the OpenAI model best suited
// for it as of 2026-05. Plan→o1 (reasoning-heavy synthesis), Execute→
// gpt-4o (default — balanced quality/latency), Budget→gpt-4o-mini
// (cheapest path for high-volume tagging). Update when a new generation
// (e.g. gpt-5-5) ships and is generally available.
var openaiModelByMode = map[ai.ModelMode]string{
	ai.ModelModePlan:    "o1",
	ai.ModelModeExecute: "gpt-4o",
	ai.ModelModeBudget:  "gpt-4o-mini",
}

func NewOpenAIProvider(apiKey, overrideModel string, opts ...option.RequestOption) *OpenAIProvider {
	clientOpts := []option.RequestOption{option.WithAPIKey(apiKey)}
	clientOpts = append(clientOpts, opts...)
	return &OpenAIProvider{
		client:        openai.NewClient(clientOpts...),
		overrideModel: overrideModel,
	}
}

// modelForMode resolves the concrete model name. User-set overrideModel
// wins; otherwise the per-mode default is used. Unknown modes fall back
// to Execute-mode default.
func (p *OpenAIProvider) modelForMode(mode ai.ModelMode) string {
	if p.overrideModel != "" {
		return p.overrideModel
	}
	if m, ok := openaiModelByMode[mode]; ok {
		return m
	}
	return openaiModelByMode[ai.ModelModeExecute]
}

// NewOpenAICompatibleProvider creates an OpenAI-compatible provider with a custom base URL.
// Works with Groq, Together, Fireworks, etc. The model parameter is used
// as overrideModel — Groq/Together users typically pin a specific model,
// not Floq's per-mode defaults (which target the official OpenAI catalog).
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
		Model:     p.modelForMode(req.Mode),
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
