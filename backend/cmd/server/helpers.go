package main

import (
	"context"
	"fmt"
	"net/http"

	"github.com/daniil/floq/internal/ai"
	"github.com/daniil/floq/internal/ai/providers"
	"github.com/daniil/floq/internal/config"
	"github.com/daniil/floq/internal/leads"
	"github.com/daniil/floq/internal/settings"
	"github.com/google/uuid"
)

func buildUsageCounter(repo *leads.Repository) settings.UsageCounter {
	return func(ctx context.Context, userID uuid.UUID) (int, int, error) {
		monthLeads, err := repo.CountMonthLeads(ctx, userID)
		if err != nil {
			return 0, 0, err
		}
		totalLeads, err := repo.CountTotalLeads(ctx, userID)
		if err != nil {
			return 0, 0, err
		}
		return monthLeads, totalLeads, nil
	}
}

func buildAITester(cfg *config.Config) settings.AITester {
	return func(ctx context.Context, provider, model, apiKey string) (string, error) {
		var p ai.Provider
		switch provider {
		case "claude":
			if apiKey == "" {
				apiKey = cfg.AnthropicAPIKey
			}
			p = providers.NewClaudeProvider(apiKey)
		case "openai":
			if apiKey == "" {
				apiKey = cfg.OpenAIAPIKey
			}
			if model == "" {
				model = cfg.OpenAIModel
			}
			p = providers.NewOpenAIProvider(apiKey, model)
		case "groq":
			if apiKey == "" {
				apiKey = cfg.GroqAPIKey
			}
			if model == "" {
				model = cfg.GroqModel
			}
			p = providers.NewOpenAICompatibleProvider(apiKey, model, "https://api.groq.com/openai/v1")
		case "ollama":
			if model == "" {
				model = cfg.OllamaModel
			}
			p = providers.NewOllamaProvider(cfg.OllamaBaseURL, model)
		default:
			return "", fmt.Errorf("неизвестный провайдер: %s", provider)
		}

		resp, err := p.Complete(ctx, ai.CompletionRequest{
			Messages:  []ai.Message{{Role: "user", Content: "Ответь одним словом: привет"}},
			MaxTokens: 256,
		})
		if err != nil {
			return "", err
		}
		_ = resp
		return p.Name(), nil
	}
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
