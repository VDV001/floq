package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	_ "github.com/golang-migrate/migrate/v4/source/file"

	"github.com/daniil/floq/internal/ai"
	"github.com/daniil/floq/internal/ai/providers"
	"github.com/daniil/floq/internal/auth"
	"github.com/daniil/floq/internal/config"
	"github.com/daniil/floq/internal/leads"
	"github.com/daniil/floq/internal/parser"
	"github.com/daniil/floq/internal/prospects"
	"github.com/daniil/floq/internal/sequences"
	"github.com/daniil/floq/internal/verify"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
)

func main() {
	// Load .env file (ignore error if missing — production uses real env vars).
	_ = godotenv.Load()

	cfg := config.Load()

	// 1. DB pool
	pool, err := pgxpool.New(context.Background(), cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("connect to db: %v", err)
	}
	defer pool.Close()

	// 1b. Run migrations
	migrationsPath := os.Getenv("MIGRATIONS_PATH")
	if migrationsPath == "" {
		migrationsPath = "file://migrations"
	}
	// golang-migrate pgx/v5 driver uses "pgx5://" scheme
	migrateDBURL := strings.Replace(cfg.DatabaseURL, "postgres://", "pgx5://", 1)
	for attempt := 1; attempt <= 5; attempt++ {
		m, err := migrate.New(migrationsPath, migrateDBURL)
		if err != nil {
			log.Printf("migrations init (attempt %d/5): %v", attempt, err)
			time.Sleep(2 * time.Second)
			continue
		}
		if err := m.Up(); err != nil && err != migrate.ErrNoChange {
			m.Close()
			log.Fatalf("migrations: %v", err)
		}
		log.Println("migrations applied")
		m.Close()
		break
	}

	// 2. AI provider
	var aiProvider ai.Provider
	switch cfg.AIProvider {
	case "claude":
		aiProvider = providers.NewClaudeProvider(cfg.AnthropicAPIKey)
	case "openai":
		aiProvider = providers.NewOpenAIProvider(cfg.OpenAIAPIKey, cfg.OpenAIModel)
	case "ollama":
		aiProvider = providers.NewOllamaProvider(cfg.OllamaBaseURL, cfg.OllamaModel)
	default:
		log.Fatalf("unknown AI_PROVIDER: %s", cfg.AIProvider)
	}
	aiClient := ai.NewAIClient(aiProvider)

	// 3. Repositories
	leadsRepo := leads.NewRepository(pool)
	prospectsRepo := prospects.NewRepository(pool)
	sequencesRepo := sequences.NewRepository(pool)

	// 4. Use cases
	leadsUC := leads.NewUseCase(leadsRepo, aiClient)
	prospectsUC := prospects.NewUseCase(prospectsRepo)
	sequencesUC := sequences.NewUseCase(sequencesRepo, aiClient, prospectsRepo, leadsRepo)

	// 5. Auth
	authHandler := auth.NewHandler(pool, cfg.JWTSecret)

	// 6. Router
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(corsMiddleware)

	// Health (public)
	r.Get("/api/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})

	// Auth (public)
	auth.RegisterRoutes(r, authHandler)

	// Protected routes
	r.Group(func(r chi.Router) {
		r.Use(auth.AuthMiddleware(cfg.JWTSecret))
		leads.RegisterRoutes(r, leadsUC)
		prospects.RegisterRoutes(r, prospectsUC)
		sequences.RegisterRoutes(r, sequencesUC)
		verify.RegisterRoutes(r, prospectsRepo, nil) // TG bot passed as nil for now
		parser.RegisterRoutes(r)
	})

	// 7. Optional: Telegram inbox bot
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if cfg.TelegramBotToken != "" {
		// For inbox bot, we need an owner user ID. Use a fixed UUID or env var.
		// For now, start bot only if token is set.
		log.Println("telegram bot token configured but owner_id not set, skipping inbox bot")
	}

	// 8. Server
	srv := &http.Server{
		Addr:    ":" + cfg.AppPort,
		Handler: r,
	}

	go func() {
		log.Printf("server listening on :%s", cfg.AppPort)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %v", err)
		}
	}()

	<-ctx.Done()
	log.Println("shutting down gracefully...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("server shutdown: %v", err)
	}
	log.Println("server stopped")
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
