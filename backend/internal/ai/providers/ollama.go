package providers

import (
	"context"
	"fmt"
	"net/http"

	"github.com/daniil/floq/internal/ai"
	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	"github.com/openai/openai-go/packages/param"
)

type OllamaProvider struct {
	client openai.Client
	model  string
	// baseURL and httpClient are retained (in addition to the OpenAI
	// client built from them) so CheckHealth can hit Ollama's native
	// GET {baseURL}/api/tags endpoint, which lives outside the /v1
	// OpenAI-compat surface.
	baseURL    string
	httpClient *http.Client
}

func NewOllamaProvider(baseURL, model string, httpClient *http.Client) *OllamaProvider {
	opts := []option.RequestOption{
		option.WithBaseURL(baseURL + "/v1"),
		option.WithAPIKey("ollama"),
	}
	if httpClient != nil {
		opts = append(opts, option.WithHTTPClient(httpClient))
	}
	client := openai.NewClient(opts...)
	return &OllamaProvider{client: client, model: model, baseURL: baseURL, httpClient: httpClient}
}

// ollamaTagsResponse mirrors the subset of GET /api/tags we consume.
type ollamaTagsResponse struct {
	Models []struct {
		Name string `json:"name"`
	} `json:"models"`
}

// CheckHealth verifies Ollama is reachable and the configured model is
// available locally, without triggering a (slow, cold-start) generation.
func (p *OllamaProvider) CheckHealth(_ context.Context) error {
	// RED stub — replaced with the real probe in the GREEN commit.
	return nil
}

// ollamaModelMatches reports whether an installed tag satisfies the
// configured model name. Ollama implicitly tags a bare name as ":latest".
func ollamaModelMatches(_, _ string) bool {
	return false
}

func (p *OllamaProvider) Name() string { return "ollama" }

// modelForMode returns the configured Ollama model regardless of the
// requested mode. Ollama runs locally and typically hosts a single model
// at a time — switching models per mode would require multiple
// concurrently-loaded models, which is rarely the local-hardware setup.
// The mode parameter is accepted for interface symmetry.
func (p *OllamaProvider) modelForMode(_ ai.ModelMode) string {
	return p.model
}

func (p *OllamaProvider) Complete(ctx context.Context, req ai.CompletionRequest) (*ai.CompletionResult, error) {
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
		return nil, fmt.Errorf("ollama complete: %w", err)
	}

	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("ollama complete: no choices in response")
	}

	// Ollama's /v1 endpoint mirrors the OpenAI schema, including the
	// Usage block. Local back-ends sometimes leave it zero — we record
	// what the response says, the pricing layer special-cases the
	// "ollama" provider name to cost=0 regardless of token counts.
	return &ai.CompletionResult{
		Text: resp.Choices[0].Message.Content,
		Usage: ai.TokenUsage{
			InputTokens:  int(resp.Usage.PromptTokens),
			OutputTokens: int(resp.Usage.CompletionTokens),
		},
		Model: model,
	}, nil
}
