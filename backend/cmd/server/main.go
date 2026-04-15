package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	_ "github.com/golang-migrate/migrate/v4/source/file"

	"github.com/daniil/floq/internal/ai"
	"github.com/daniil/floq/internal/ai/providers"
	"github.com/daniil/floq/internal/auth"
	"github.com/daniil/floq/internal/chat"
	"github.com/daniil/floq/internal/config"
	"github.com/daniil/floq/internal/db"
	"github.com/daniil/floq/internal/inbox"
	"github.com/daniil/floq/internal/leads"
	"github.com/daniil/floq/internal/notify"
	"github.com/daniil/floq/internal/outbound"
	"github.com/daniil/floq/internal/parser"
	"github.com/daniil/floq/internal/reminders"
	"github.com/daniil/floq/internal/prospects"
	"github.com/daniil/floq/internal/sequences"
	"github.com/daniil/floq/internal/settings"
	"github.com/daniil/floq/internal/sources"
	"github.com/daniil/floq/internal/tgclient"
	"github.com/daniil/floq/internal/verify"
	"github.com/google/uuid"
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

	// 2. Settings store (reads user_settings from DB, used by services)
	settingsStore := settings.NewStore(pool)
	settingsRepo := settings.NewRepository(pool)
	settingsUC := settings.NewUseCase(settingsRepo, &settings.HTTPTelegramValidator{})

	ownerID, err := uuid.Parse(cfg.OwnerUserID)
	if err != nil {
		log.Fatalf("invalid OWNER_USER_ID: %v", err)
	}

	// 2b. AI provider (dynamic: reads provider/model/key from DB, falls back to .env)
	aiProvider := providers.NewDynamicProvider(settingsStore, ownerID, cfg)
	aiClient := ai.NewAIClient(aiProvider, cfg.BookingLink, cfg.SenderName, cfg.SenderCompany, cfg.SenderPhone, cfg.SenderWebsite)

	// 3. Repositories
	leadsRepo := leads.NewRepository(pool)
	prospectsRepo := prospects.NewRepository(pool)
	sourcesRepo := sources.NewRepository(pool)
	sequencesRepo := sequences.NewRepository(pool)

	// 4. Adapters
	leadsAI := leads.NewAIAdapter(aiClient)
	seqAI := sequences.NewAIMessageGeneratorAdapter(aiClient)
	prospectReader := sequences.NewProspectReaderAdapter(prospectsRepo)
	leadCreatorAdapter := sequences.NewLeadCreatorAdapter(leadsRepo)

	// 5. Use cases
	leadsUC := leads.NewUseCase(leadsRepo, leadsAI, nil) // sender set after bot init
	prospectsUC := prospects.NewUseCase(prospectsRepo, prospects.WithLeadChecker(newLeadCheckerAdapter(leadsRepo)))
	sourcesUC := sources.NewUseCase(sourcesRepo, sources.WithStatsReader(sourcesRepo))
	migrateOrphanProspects(pool, ownerID)
	txManager := db.NewTxManager(pool)
	sequencesUC := sequences.NewUseCase(sequencesRepo, seqAI, prospectReader, leadCreatorAdapter, sequences.WithTxManager(txManager))

	// 5. Auth
	authHandler := auth.NewHandler(auth.NewRepository(pool), cfg.JWTSecret)

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

	// Tracking pixel (public, no auth — loaded by email clients)
	sequences.RegisterPublicRoutes(r, sequencesUC)

	// Auth (public)
	auth.RegisterRoutes(r, authHandler)

	// Protected routes
	r.Group(func(r chi.Router) {
		r.Use(auth.AuthMiddleware(cfg.JWTSecret))
		leads.RegisterRoutes(r, leadsUC)
		prospects.RegisterRoutes(r, prospectsUC)
		sequences.RegisterRoutes(r, sequencesUC)
		sources.RegisterRoutes(r, sourcesUC)
		verify.RegisterRoutes(r, prospectsRepo, nil) // TG bot passed as nil for now
		parser.RegisterRoutes(r, cfg.TwoGISAPIKey)
		settings.RegisterRoutes(r, settingsUC, buildAITester(cfg), buildSMTPTester(), buildResendTester(), buildUsageCounter(leadsRepo))
		chat.RegisterRoutes(r, chat.NewHandler(chat.NewRepository(pool), newChatAIAdapter(aiClient)))
		tgclient.RegisterRoutes(r, tgclient.NewClient(), tgclient.NewRepository(pool))
	})

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// 7. Outbound email sender (cron every 30 seconds)
	// Always starts — reads Resend API key from DB each tick (falls back to .env)
	tgRepo := tgclient.NewRepository(pool)
	emailSender := outbound.NewSender(settingsStore, ownerID, cfg.ResendAPIKey, cfg.SMTPFrom, cfg.AppBaseURL, cfg.SMTPHost, cfg.SMTPPort, cfg.SMTPUser, cfg.SMTPPassword, sequencesRepo, prospectsRepo, tgRepo, outbound.NewMTProtoMessenger())
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				if err := emailSender.SendPending(context.Background()); err != nil {
					log.Printf("outbound sender: %v", err)
				}
			case <-ctx.Done():
				return
			}
		}
	}()
	log.Println("outbound email sender started (every 30s)")

	// 8. Optional: Telegram inbox bot
	// Read token from DB first, fall back to .env
	prospectAdapter := newProspectRepoAdapter(prospectsRepo)
	inboxLeadAdapter := newInboxLeadRepoAdapter(leadsRepo)
	inboxAI := newInboxAIAdapter(aiClient)
	inboxCfg := newInboxConfigAdapter(settingsStore)
	tgToken := cfg.TelegramBotToken
	if dbCfg, err := settingsStore.GetConfig(context.Background(), ownerID); err == nil && dbCfg.TelegramBotToken != "" {
		tgToken = dbCfg.TelegramBotToken
	}
	if tgToken != "" {
		tgBot, err := inbox.NewTelegramBot(tgToken, inboxLeadAdapter, prospectAdapter, inboxAI, ownerID, cfg.BookingLink)
		if err != nil {
			log.Printf("telegram bot init failed: %v", err)
		} else {
			go tgBot.Start(ctx)
			// Set the telegram sender on the leads use case
			leadsUC.SetSender(leads.NewTelegramSender(tgBot.Bot()))
		}
	}

	// 9. Email IMAP poller (reads settings from DB, falls back to .env)
	emailPoller := inbox.NewEmailPoller(inboxCfg, ownerID, cfg.IMAPHost, cfg.IMAPPort, cfg.IMAPUser, cfg.IMAPPassword, inboxLeadAdapter, prospectAdapter, sequencesRepo, inboxAI)
	go emailPoller.Start(ctx)

	// 10. Reminders cron (hourly, checks for stale leads)
	var notifier reminders.Notifier
	notifyChatIDStr := os.Getenv("NOTIFY_CHAT_ID")
	if tgToken != "" && notifyChatIDStr != "" {
		chatID, err := strconv.ParseInt(notifyChatIDStr, 10, 64)
		if err == nil {
			n, err := notify.NewTelegramNotifier(tgToken, chatID)
			if err != nil {
				log.Printf("telegram notifier init failed: %v", err)
			} else {
				notifier = n
			}
		}
	}
	remindersCron := reminders.NewCron(leadsRepo, aiClient, notifier, cfg.StaleDays)
	go remindersCron.Start(ctx)
	log.Println("reminders cron started (hourly)")

	// 11. Server
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

