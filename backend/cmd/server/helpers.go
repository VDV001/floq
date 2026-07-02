package main

import (
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	smtpLib "net/smtp"
	"time"

	"github.com/daniil/floq/internal/ai"
	"github.com/daniil/floq/internal/ai/providers"
	"github.com/daniil/floq/internal/config"
	"github.com/daniil/floq/internal/integrations/onec"
	"github.com/daniil/floq/internal/integrations/onec/domain"
	"github.com/daniil/floq/internal/leads"
	"github.com/daniil/floq/internal/proxy"
	"github.com/daniil/floq/internal/settings"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	openaiopt "github.com/openai/openai-go/option"
)

// onecSecretGenerator produces 1C webhook secrets from crypto/rand. This is the
// infra side of the onec.SecretGenerator port — the usecase stays oblivious to
// how the entropy is sourced.
type onecSecretGenerator struct{}

// WebhookSecret returns the hex encoding of WebhookSecretBytes random bytes.
func (onecSecretGenerator) WebhookSecret() (string, error) {
	b := make([]byte, domain.WebhookSecretBytes)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("onec: read random: %w", err)
	}
	return hex.EncodeToString(b), nil
}

// buildOnecSecretGenerator returns the crypto/rand webhook-secret generator.
func buildOnecSecretGenerator() onec.SecretGenerator {
	return onecSecretGenerator{}
}

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

// buildAIProvider constructs the concrete ai.Provider for a provider name,
// filling any empty apiKey/model from .env config. Shared by the connection
// tester and the model lister so the provider matrix lives in one place.
// Unknown provider → settings.ErrAIUnknownProvider.
func buildAIProvider(cfg *config.Config, httpClient *http.Client, provider, model, apiKey string) (ai.Provider, error) {
	switch provider {
	case "claude":
		if apiKey == "" {
			apiKey = cfg.AnthropicAPIKey
		}
		return providers.NewClaudeProvider(apiKey, model, httpClient), nil
	case "openai":
		if apiKey == "" {
			apiKey = cfg.OpenAIAPIKey
		}
		if model == "" {
			model = cfg.OpenAIModel
		}
		var opts []openaiopt.RequestOption
		if httpClient != nil {
			opts = append(opts, openaiopt.WithHTTPClient(httpClient))
		}
		return providers.NewOpenAIProvider(apiKey, model, opts...), nil
	case "groq":
		if apiKey == "" {
			apiKey = cfg.GroqAPIKey
		}
		if model == "" {
			model = cfg.GroqModel
		}
		return providers.NewOpenAICompatibleProvider(apiKey, model, "groq", "https://api.groq.com/openai/v1", httpClient), nil
	case "gemini":
		if apiKey == "" {
			apiKey = cfg.GeminiAPIKey
		}
		if model == "" {
			model = cfg.GeminiModel
		}
		return providers.NewOpenAICompatibleProvider(apiKey, model, "gemini", "https://generativelanguage.googleapis.com/v1beta/openai", httpClient), nil
	case "openrouter":
		if apiKey == "" {
			apiKey = cfg.OpenRouterAPIKey
		}
		if model == "" {
			model = cfg.OpenRouterModel
		}
		return providers.NewOpenAICompatibleProvider(apiKey, model, "openrouter", "https://openrouter.ai/api/v1", httpClient), nil
	case "ollama":
		if model == "" {
			model = cfg.OllamaModel
		}
		return providers.NewOllamaProvider(cfg.OllamaBaseURL, model, httpClient), nil
	default:
		return nil, fmt.Errorf("%w: %s", settings.ErrAIUnknownProvider, provider)
	}
}

func buildAITester(cfg *config.Config, httpClient *http.Client) settings.AITester {
	return func(ctx context.Context, provider, model, apiKey string) (string, error) {
		p, err := buildAIProvider(cfg, httpClient, provider, model, apiKey)
		if err != nil {
			return "", err
		}

		// Every provider above implements HealthChecker, so the connection
		// test uses a cheap liveness probe (Ollama: /api/tags; cloud:
		// free /models) — no billed generation and no cold-start timeout
		// (#227, #235). The Complete fallback below is the documented
		// contract for any future provider added without a health check.
		if hc, ok := p.(ai.HealthChecker); ok {
			if err := hc.CheckHealth(ctx); err != nil {
				return "", mapAIHealthError(err)
			}
			return p.Name(), nil
		}

		if _, err := p.Complete(ctx, ai.CompletionRequest{
			Messages:  []ai.Message{{Role: "user", Content: "Ответь одним словом: привет"}},
			MaxTokens: 256,
		}); err != nil {
			return "", err
		}
		return p.Name(), nil
	}
}

// buildAIModelLister returns a settings.AIModelLister that enumerates a
// provider's models via the same provider constructors as buildAITester,
// using the ModelLister capability. Errors bubble up; the handler turns
// them into an empty list (UI falls back to manual entry).
func buildAIModelLister(cfg *config.Config, httpClient *http.Client) settings.AIModelLister {
	return func(ctx context.Context, provider, model, apiKey string) ([]settings.AIModel, error) {
		p, err := buildAIProvider(cfg, httpClient, provider, model, apiKey)
		if err != nil {
			return nil, err
		}
		lister, ok := p.(ai.ModelLister)
		if !ok {
			return nil, fmt.Errorf("provider %q cannot list models", provider)
		}
		models, err := lister.ListModels(ctx)
		if err != nil {
			return nil, err
		}
		out := make([]settings.AIModel, 0, len(models))
		for _, m := range models {
			out = append(out, settings.AIModel{ID: m.ID, Meta: m.Meta})
		}
		return out, nil
	}
}

// mapAIHealthError translates a provider-vocabulary health-check error
// into the settings-handler vocabulary, so the handler can render Russian
// copy via errors.Is without importing the providers package. Unknown
// errors pass through unwrapped.
func mapAIHealthError(err error) error {
	switch {
	case errors.Is(err, providers.ErrOllamaModelNotFound):
		return fmt.Errorf("%w: %v", settings.ErrAIModelNotFound, err)
	case errors.Is(err, providers.ErrProviderAuth):
		return fmt.Errorf("%w: %v", settings.ErrAIAuth, err)
	case errors.Is(err, providers.ErrProviderRateLimit):
		return fmt.Errorf("%w: %v", settings.ErrAIRateLimit, err)
	case errors.Is(err, providers.ErrOllamaUnreachable), errors.Is(err, providers.ErrProviderUnreachable):
		return fmt.Errorf("%w: %v", settings.ErrAIUnreachable, err)
	default:
		return err
	}
}

func buildSMTPTester(proxyDialer proxy.ContextDialer) settings.SMTPTester {
	return func(ctx context.Context, host, port, user, password string) error {
		addr := net.JoinHostPort(host, port)

		var conn net.Conn
		var err error

		if port == "465" {
			if proxyDialer != nil {
				rawConn, dialErr := proxyDialer.DialContext(ctx, "tcp", addr)
				if dialErr != nil {
					return fmt.Errorf("%w: %v", settings.ErrSMTPProxyDial, dialErr)
				}
				conn = tls.Client(rawConn, &tls.Config{ServerName: host})
			} else {
				conn, err = tls.DialWithDialer(
					&net.Dialer{Timeout: 10 * time.Second}, "tcp", addr,
					&tls.Config{ServerName: host},
				)
				if err != nil {
					return fmt.Errorf("%w: %v", settings.ErrSMTPDial, err)
				}
			}
		} else {
			if proxyDialer != nil {
				conn, err = proxyDialer.DialContext(ctx, "tcp", addr)
				if err != nil {
					return fmt.Errorf("%w: %v", settings.ErrSMTPProxyDial, err)
				}
			} else {
				conn, err = net.DialTimeout("tcp", addr, 10*time.Second)
				if err != nil {
					return fmt.Errorf("%w: %v", settings.ErrSMTPDial, err)
				}
			}
		}
		defer conn.Close()

		client, err := smtpLib.NewClient(conn, host)
		if err != nil {
			return fmt.Errorf("%w: %v", settings.ErrSMTPClient, err)
		}
		defer client.Close()

		if port != "465" {
			if err := client.StartTLS(&tls.Config{ServerName: host}); err != nil {
				return fmt.Errorf("%w: %v", settings.ErrSMTPStartTLS, err)
			}
		}

		auth := smtpLib.PlainAuth("", user, password, host)
		if err := client.Auth(auth); err != nil {
			return fmt.Errorf("%w: %v", settings.ErrSMTPAuth, err)
		}
		_ = client.Quit()
		return nil
	}
}

func buildResendTester(httpClient *http.Client) settings.ResendTester {
	return func(ctx context.Context, apiKey string) error {
		req, err := http.NewRequestWithContext(ctx, "GET", "https://api.resend.com/api-keys", nil)
		if err != nil {
			return fmt.Errorf("%w: %v", settings.ErrResendRequest, err)
		}
		req.Header.Set("Authorization", "Bearer "+apiKey)

		client := httpClient
		if client == nil {
			client = http.DefaultClient
		}
		resp, err := client.Do(req)
		if err != nil {
			return fmt.Errorf("%w: %v", settings.ErrResendRequest, err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != 200 {
			return settings.ErrResendAuth
		}
		return nil
	}
}

// migrateOrphanProspects links prospects with source_id=NULL to existing sources by text name.
// Called once at startup after all repositories are initialised.
func migrateOrphanProspects(pool *pgxpool.Pool, userID uuid.UUID) {
	ctx := context.Background()
	migrations := map[string]string{"csv": "CSV файл", "manual": "Вручную", "2gis": "2GIS"}
	for oldSource, newName := range migrations {
		_, _ = pool.Exec(ctx,
			`UPDATE prospects SET source_id = (SELECT id FROM lead_sources WHERE user_id = $1 AND name = $2 LIMIT 1)
			 WHERE user_id = $1 AND source = $3 AND source_id IS NULL`,
			userID, newName, oldSource)
	}
	log.Println("orphan prospects migration completed")
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
