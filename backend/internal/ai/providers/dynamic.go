package providers

import (
	"context"
	"fmt"
	"sync"

	"github.com/daniil/floq/internal/ai"
	"github.com/daniil/floq/internal/config"
	"github.com/daniil/floq/internal/settings"
	"github.com/google/uuid"
)

// DynamicProvider reads AI settings from user_settings DB on each call,
// caching the underlying provider until settings change.
type DynamicProvider struct {
	store       *settings.Store
	ownerID     uuid.UUID
	fallbackCfg *config.Config

	mu           sync.Mutex
	cached       ai.Provider
	cachedKey    string // "provider:model:apikey" — cache invalidation key
}

func NewDynamicProvider(store *settings.Store, ownerID uuid.UUID, fallbackCfg *config.Config) *DynamicProvider {
	return &DynamicProvider{
		store:       store,
		ownerID:     ownerID,
		fallbackCfg: fallbackCfg,
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

func (d *DynamicProvider) Complete(ctx context.Context, req ai.CompletionRequest) (string, error) {
	provider, err := d.resolve(ctx)
	if err != nil {
		return "", err
	}
	return provider.Complete(ctx, req)
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
	case "ollama":
		if model == "" {
			model = d.fallbackCfg.OllamaModel
		}
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
		p = NewClaudeProvider(apiKey)
	case "openai":
		p = NewOpenAIProvider(apiKey, model)
	case "ollama":
		p = NewOllamaProvider(d.fallbackCfg.OllamaBaseURL, model)
	default:
		return nil, fmt.Errorf("unknown AI provider: %s", providerName)
	}

	d.cached = p
	d.cachedKey = cacheKey
	return p, nil
}
