package providers

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/daniil/floq/internal/ai"
	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	"github.com/openai/openai-go/packages/param"
)

// Compile-time check: OpenAIProvider satisfies ai.VisionProvider so
// AIClient.AnalyzeImage (which type-asserts the provider) reaches the
// implementation below in production.
var _ ai.VisionProvider = (*OpenAIProvider)(nil)

type OpenAIProvider struct {
	client openai.Client
	// overrideModel, if non-empty, is used for every Complete call
	// regardless of the request's ModelMode (set via user settings
	// AIModel). Empty string means "use the per-mode default map".
	overrideModel string
	// name is the provider's reported identity. Plain OpenAI is "openai";
	// OpenAI-compatible back-ends (Groq, Together, …) set their own so the
	// connection test and audit log name the real provider (#238).
	name string
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
		name:          "openai",
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
// name is the provider's reported identity (e.g. "groq"), so the
// connection test names the real back-end rather than "openai" (#238).
func NewOpenAICompatibleProvider(apiKey, model, name, baseURL string, httpClient *http.Client) *OpenAIProvider {
	opts := []option.RequestOption{option.WithBaseURL(baseURL)}
	if httpClient != nil {
		opts = append(opts, option.WithHTTPClient(httpClient))
	}
	p := NewOpenAIProvider(apiKey, model, opts...)
	p.name = name
	return p
}

func (p *OpenAIProvider) Name() string { return p.name }

// Compile-time check: OpenAIProvider offers a cheap liveness probe so the
// connection test avoids a (billed) generation.
var _ ai.HealthChecker = (*OpenAIProvider)(nil)

// CheckHealth verifies the API key and reachability via the free
// GET /models endpoint — no generation, so the connection test neither
// bills tokens nor trips a generation timeout. Retries are disabled so a
// throttled or down back-end fails fast instead of stalling the test.
func (p *OpenAIProvider) CheckHealth(ctx context.Context) error {
	if _, err := p.client.Models.List(ctx, option.WithMaxRetries(0)); err != nil {
		var apiErr *openai.Error
		if errors.As(err, &apiErr) {
			return fmt.Errorf("%w: %v", classifyProviderStatus(apiErr.StatusCode), err)
		}
		return fmt.Errorf("%w: %v", ErrProviderUnreachable, err)
	}
	return nil
}

// Compile-time check: OpenAIProvider can enumerate its models (#229).
var _ ai.ModelLister = (*OpenAIProvider)(nil)

// ListModels returns the models available for the configured key via the
// OpenAI-compatible GET /models endpoint. Meta is left empty — that
// endpoint reports only an id (no size/price). Retries disabled so a
// down back-end fails fast rather than stalling the settings form.
func (p *OpenAIProvider) ListModels(ctx context.Context) ([]ai.ModelInfo, error) {
	page, err := p.client.Models.List(ctx, option.WithMaxRetries(0))
	if err != nil {
		return nil, fmt.Errorf("list models: %w", err)
	}
	models := make([]ai.ModelInfo, 0, len(page.Data))
	for _, m := range page.Data {
		models = append(models, ai.ModelInfo{ID: m.ID})
	}
	return models, nil
}

func (p *OpenAIProvider) Complete(ctx context.Context, req ai.CompletionRequest) (*ai.CompletionResult, error) {
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

	model := p.modelForMode(req.Mode)
	resp, err := p.client.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
		Model:     model,
		Messages:  messages,
		MaxTokens: param.NewOpt(int64(req.MaxTokens)),
	})
	if err != nil {
		return nil, fmt.Errorf("openai complete: %w", err)
	}

	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("openai complete: no choices in response")
	}

	usage := ai.TokenUsage{
		InputTokens:  int(resp.Usage.PromptTokens),
		OutputTokens: int(resp.Usage.CompletionTokens),
	}

	content := resp.Choices[0].Message.Content
	if content != "" {
		return &ai.CompletionResult{Text: content, Usage: usage, Model: model}, nil
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
			return &ai.CompletionResult{Text: msgFields.Reasoning, Usage: usage, Model: model}, nil
		}
	}

	return &ai.CompletionResult{Text: "", Usage: usage, Model: model}, nil
}

// AnalyzeImage sends imageData together with prompt as a multimodal
// chat-completion request and returns the assistant's text response.
// Always uses Budget mode (gpt-4o-mini by default) — vision passes are
// volume-driven, cost dominates reasoning depth.
func (p *OpenAIProvider) AnalyzeImage(ctx context.Context, imageData []byte, mimeType, prompt string) (*ai.CompletionResult, error) {
	dataURI := fmt.Sprintf("data:%s;base64,%s", mimeType, base64.StdEncoding.EncodeToString(imageData))
	parts := []openai.ChatCompletionContentPartUnionParam{
		openai.TextContentPart(prompt),
		openai.ImageContentPart(openai.ChatCompletionContentPartImageImageURLParam{URL: dataURI}),
	}

	model := p.modelForMode(ai.ModelModeBudget)
	resp, err := p.client.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
		Model:     model,
		Messages:  []openai.ChatCompletionMessageParamUnion{openai.UserMessage(parts)},
		MaxTokens: param.NewOpt(int64(1024)),
	})
	if err != nil {
		return nil, fmt.Errorf("analyze image: %w", err)
	}
	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("analyze image: no choices in response")
	}
	return &ai.CompletionResult{
		Text:  resp.Choices[0].Message.Content,
		Usage: ai.TokenUsage{
			InputTokens:  int(resp.Usage.PromptTokens),
			OutputTokens: int(resp.Usage.CompletionTokens),
		},
		Model: model,
	}, nil
}
