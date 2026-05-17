package providers

import (
	"context"
	"fmt"
	"net/http"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/daniil/floq/internal/ai"
)

type ClaudeProvider struct {
	client anthropic.Client
	// overrideModel, if non-empty, is used for every Complete call
	// regardless of the request's ModelMode. Set from user settings
	// (cfg.AIModel). Empty string means "use the per-mode default map".
	overrideModel string
}

// claudeModelByMode maps each workload intent to the Claude model best
// suited for it as of 2026-05. Plan→Opus 4.7 (synthesis-heavy),
// Execute→Sonnet 4.6 (default — structured fast response), Budget→Haiku
// 4.5 (cheap classification). Update this map when Anthropic ships a new
// generation; call sites need not change.
var claudeModelByMode = map[ai.ModelMode]string{
	ai.ModelModePlan:    "claude-opus-4-7",
	ai.ModelModeExecute: "claude-sonnet-4-6",
	ai.ModelModeBudget:  "claude-haiku-4-5-20251001",
}

// NewClaudeProvider creates an Anthropic-backed provider. If overrideModel
// is non-empty (typically set via user settings AIModel), that model is
// used for every request regardless of mode. Otherwise the per-mode map
// is consulted.
func NewClaudeProvider(apiKey, overrideModel string, httpClient *http.Client) *ClaudeProvider {
	opts := []option.RequestOption{option.WithAPIKey(apiKey)}
	if httpClient != nil {
		opts = append(opts, option.WithHTTPClient(httpClient))
	}
	return &ClaudeProvider{
		client:        anthropic.NewClient(opts...),
		overrideModel: overrideModel,
	}
}

func (p *ClaudeProvider) Name() string { return "claude" }

// modelForMode resolves the concrete model name. User-set overrideModel
// always wins (per the principle that user configuration beats defaults).
// Falls back to the Execute-mode default for unknown modes — defensive
// against future enum additions reaching this provider before the map
// is updated.
func (p *ClaudeProvider) modelForMode(mode ai.ModelMode) string {
	if p.overrideModel != "" {
		return p.overrideModel
	}
	if m, ok := claudeModelByMode[mode]; ok {
		return m
	}
	return claudeModelByMode[ai.ModelModeExecute]
}

func (p *ClaudeProvider) Complete(ctx context.Context, req ai.CompletionRequest) (*ai.CompletionResult, error) {
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

	model := p.modelForMode(req.Mode)
	resp, err := p.client.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     anthropic.Model(model),
		MaxTokens: int64(req.MaxTokens),
		System:    system,
		Messages:  messages,
	})
	if err != nil {
		return nil, fmt.Errorf("claude complete: %w", err)
	}

	if len(resp.Content) == 0 {
		return nil, fmt.Errorf("claude complete: empty response")
	}

	return &ai.CompletionResult{
		Text:  resp.Content[0].AsText().Text,
		Usage: ai.TokenUsage{
			InputTokens:  int(resp.Usage.InputTokens),
			OutputTokens: int(resp.Usage.OutputTokens),
		},
		Model: model,
	}, nil
}
