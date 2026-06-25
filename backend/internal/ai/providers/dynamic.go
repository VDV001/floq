package providers

import (
	"context"
	"fmt"
	"net/http"
	"sync"

	"github.com/daniil/floq/internal/ai"
	"github.com/daniil/floq/internal/config"
	"github.com/daniil/floq/internal/settings"
	"github.com/google/uuid"
	"github.com/openai/openai-go/option"
)

// Compile-time assertion that DynamicProvider satisfies VisionProvider.
// Loss of this assertion is the bug that hid behind issue #25 PR3's
// first review pass: type assertion in the audit recording wrapper
// silently fell through to ErrVisionUnsupported in production while
// the acceptance test wrapped a vision-capable stub directly.
var (
	_ ai.Provider       = (*DynamicProvider)(nil)
	_ ai.VisionProvider = (*DynamicProvider)(nil)
)

// DynamicProvider reads AI settings from user_settings DB on each call,
// caching the underlying provider until settings change.
type DynamicProvider struct {
	store       *settings.Store
	ownerID     uuid.UUID
	fallbackCfg *config.Config
	httpClient  *http.Client

	mu           sync.Mutex
	cached       ai.Provider
	cachedKey    string // "provider:model:apikey" — cache invalidation key
}

func NewDynamicProvider(store *settings.Store, ownerID uuid.UUID, fallbackCfg *config.Config, httpClient *http.Client) *DynamicProvider {
	return &DynamicProvider{
		store:       store,
		ownerID:     ownerID,
		fallbackCfg: fallbackCfg,
		httpClient:  httpClient,
	}
}

func (d *DynamicProvider) Name() string {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.cached != nil {
		return d.cached.Name()
	}
	return d.fallbackCfg.AIProvider
}

func (d *DynamicProvider) Complete(ctx context.Context, req ai.CompletionRequest) (*ai.CompletionResult, error) {
	provider, err := d.resolve(ctx)
	if err != nil {
		return nil, err
	}
	return provider.Complete(ctx, req)
}

// AnalyzeImage proxies vision calls through to the resolved provider
// when the latter supports VisionProvider. Without this, an audit-
// layer type assertion on the production AIClient (which wraps a
// DynamicProvider) would always fail and image_analysis attachments
// would never reach OpenAI — see issue #25 acceptance.
func (d *DynamicProvider) AnalyzeImage(ctx context.Context, imageData []byte, mimeType, prompt string) (*ai.CompletionResult, error) {
	provider, err := d.resolve(ctx)
	if err != nil {
		return nil, err
	}
	vp, ok := provider.(ai.VisionProvider)
	if !ok {
		return nil, ai.ErrVisionUnsupported
	}
	return vp.AnalyzeImage(ctx, imageData, mimeType, prompt)
}

// providerNeedsAPIKey reports whether the named provider requires an API key.
// Ollama runs locally and needs none; unknown names are not flagged here
// (resolve rejects them separately as "unknown AI provider").
func providerNeedsAPIKey(provider string) bool {
	switch provider {
	case "claude", "openai", "groq":
		return true
	default:
		return false
	}
}

// validateProviderConfig returns ai.ErrNotConfigured when a key-requiring
// provider is selected without a key, so the failure is detectable up front
// instead of surfacing as an opaque auth error from the upstream call.
func validateProviderConfig(provider, apiKey string) error {
	if providerNeedsAPIKey(provider) && apiKey == "" {
		return ai.ErrNotConfigured
	}
	return nil
}

func (d *DynamicProvider) resolve(ctx context.Context) (ai.Provider, error) {
	// Read current settings from DB
	providerName := d.fallbackCfg.AIProvider
	model := ""
	apiKey := ""

	if cfg, err := d.store.GetConfig(ctx, d.ownerID); err == nil {
		if cfg.AIProvider != "" {
			providerName = cfg.AIProvider
		}
		if cfg.AIModel != "" {
			model = cfg.AIModel
		}
		if cfg.AIAPIKey != "" {
			apiKey = cfg.AIAPIKey
		}
	}

	// Fill in fallbacks from .env config
	switch providerName {
	case "claude":
		if apiKey == "" {
			apiKey = d.fallbackCfg.AnthropicAPIKey
		}
	case "openai":
		if apiKey == "" {
			apiKey = d.fallbackCfg.OpenAIAPIKey
		}
		if model == "" {
			model = d.fallbackCfg.OpenAIModel
		}
	case "groq":
		if apiKey == "" {
			apiKey = d.fallbackCfg.GroqAPIKey
		}
		if model == "" {
			model = d.fallbackCfg.GroqModel
		}
	case "ollama":
		if model == "" {
			model = d.fallbackCfg.OllamaModel
		}
	}

	if err := validateProviderConfig(providerName, apiKey); err != nil {
		return nil, err
	}

	cacheKey := providerName + ":" + model + ":" + apiKey

	d.mu.Lock()
	defer d.mu.Unlock()

	if d.cached != nil && d.cachedKey == cacheKey {
		return d.cached, nil
	}

	// Create new provider
	var p ai.Provider
	switch providerName {
	case "claude":
		// Pass user-configured model as override; empty string falls back
		// to the per-mode default map inside ClaudeProvider.
		p = NewClaudeProvider(apiKey, model, d.httpClient)
	case "openai":
		var opts []option.RequestOption
		if d.httpClient != nil {
			opts = append(opts, option.WithHTTPClient(d.httpClient))
		}
		p = NewOpenAIProvider(apiKey, model, opts...)
	case "groq":
		p = NewOpenAICompatibleProvider(apiKey, model, "https://api.groq.com/openai/v1", d.httpClient)
	case "ollama":
		p = NewOllamaProvider(d.fallbackCfg.OllamaBaseURL, model, d.httpClient)
	default:
		return nil, fmt.Errorf("unknown AI provider: %s", providerName)
	}

	d.cached = p
	d.cachedKey = cacheKey
	return p, nil
}
