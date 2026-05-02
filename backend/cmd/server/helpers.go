package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"log"
	"net"
	"net/http"
	smtpLib "net/smtp"
	"time"

	"github.com/daniil/floq/internal/ai"
	"github.com/daniil/floq/internal/ai/providers"
	"github.com/daniil/floq/internal/config"
	"github.com/daniil/floq/internal/leads"
	"github.com/daniil/floq/internal/proxy"
	"github.com/daniil/floq/internal/settings"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	openaiopt "github.com/openai/openai-go/option"
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

func buildAITester(cfg *config.Config, httpClient *http.Client) settings.AITester {
	return func(ctx context.Context, provider, model, apiKey string) (string, error) {
		var p ai.Provider
		switch provider {
		case "claude":
			if apiKey == "" {
				apiKey = cfg.AnthropicAPIKey
			}
			p = providers.NewClaudeProvider(apiKey, httpClient)
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
			p = providers.NewOpenAIProvider(apiKey, model, opts...)
		case "groq":
			if apiKey == "" {
				apiKey = cfg.GroqAPIKey
			}
			if model == "" {
				model = cfg.GroqModel
			}
			p = providers.NewOpenAICompatibleProvider(apiKey, model, "https://api.groq.com/openai/v1", httpClient)
		case "ollama":
			if model == "" {
				model = cfg.OllamaModel
			}
			p = providers.NewOllamaProvider(cfg.OllamaBaseURL, model, httpClient)
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
			return fmt.Errorf("Ошибка запроса: %v", err)
		}
		req.Header.Set("Authorization", "Bearer "+apiKey)

		client := httpClient
		if client == nil {
			client = http.DefaultClient
		}
		resp, err := client.Do(req)
		if err != nil {
			return fmt.Errorf("Ошибка запроса: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != 200 {
			return fmt.Errorf("Неверный API ключ Resend")
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
