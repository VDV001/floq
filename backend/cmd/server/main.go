package main

import (
	"context"
	"log"
	"log/slog"
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
	"github.com/daniil/floq/internal/audit"
	"github.com/daniil/floq/internal/auth"
	"github.com/daniil/floq/internal/chat"
	"github.com/daniil/floq/internal/config"
	"github.com/daniil/floq/internal/db"
	"github.com/daniil/floq/internal/httputil"
	"github.com/daniil/floq/internal/inbox"
	"github.com/daniil/floq/internal/inbox/attachments"
	"github.com/daniil/floq/internal/leads"
	"github.com/daniil/floq/internal/notify"
	"github.com/daniil/floq/internal/outbound"
	"github.com/daniil/floq/internal/parser"
	"github.com/daniil/floq/internal/proxy"
	"github.com/daniil/floq/internal/ratelimit"
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
	"github.com/redis/go-redis/v9"
)

// Rate-limit budget for HITL approve/reject combined per user. 30
// requests per minute is over an order of magnitude above any
// legitimate human cadence and still capped enough to bound abuse
// from a compromised JWT.
const (
	pendingReplyRateLimit  = 30
	pendingReplyRateWindow = time.Minute
)

func main() {
	// Load .env file (ignore error if missing — production uses real env vars).
	_ = godotenv.Load()

	cfg := config.Load()

	// 0. Proxy provider (empty PROXY_URL = direct connection)
	proxyProvider, err := proxy.NewFromURL(cfg.ProxyURL)
	if err != nil {
		log.Fatalf("invalid PROXY_URL: %v", err)
	}
	httpClient := proxyProvider.HTTPClient()
	proxyDialer := proxyProvider.Dialer() // non-nil for SOCKS5 only

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
	settingsUC := settings.NewUseCase(settingsRepo, &settings.HTTPTelegramValidator{HTTPClient: httpClient})

	ownerID, err := uuid.Parse(cfg.OwnerUserID)
	if err != nil {
		log.Fatalf("invalid OWNER_USER_ID: %v", err)
	}

	// 2b. AI provider (dynamic: reads provider/model/key from DB, falls back to .env)
	aiProvider := providers.NewDynamicProvider(settingsStore, ownerID, cfg, httpClient)

	// 2c. Audit log: every provider call goes through a RecordingProvider
	// that drops a cost-attributed row into audit_log via an async
	// AsyncRecorder. Buffer overflow drops with a metric counter —
	// audit must never block the AI hot path.
	auditRepo := audit.NewRepository(pool)
	auditRecorder := audit.NewAsyncRecorder(auditRepo,
		audit.WithLogger(slog.Default()))
	auditRecorder.Start()
	wrappedProvider := audit.NewRecordingProvider(aiProvider, auditRecorder, slog.Default())

	// Read the owner's style-check preference once at boot. We don't
	// propagate runtime changes — switching the toggle in the UI requires
	// a server restart, documented in docs/_tools/llm_style_check.md. The
	// alternative (per-request settings lookup) would double our DB reads
	// on every outbound generation.
	styleCheckEnabled := false
	if ownerSettings, err := settingsRepo.GetSettings(context.Background(), ownerID); err == nil {
		styleCheckEnabled = ownerSettings.AIStyleCheckEnabled
	}
	aiClient := ai.NewAIClient(wrappedProvider, cfg.BookingLink, cfg.SenderName, cfg.SenderCompany, cfg.SenderPhone, cfg.SenderWebsite,
		ai.WithStyleCheck(styleCheckEnabled))

	// 3. Repositories
	leadsRepo := leads.NewRepository(pool)
	prospectsRepo := prospects.NewRepository(pool)
	sourcesRepo := sources.NewRepository(pool)
	sequencesRepo := sequences.NewRepository(pool)
	identityRepo := leads.NewIdentityRepository(pool)
	pendingReplyRepo := inbox.NewPendingReplyRepository(pool)
	// pendingReplyUC is constructed early (dispatcher nil) so the
	// protected route registration below can reference it. The
	// dispatcher is injected after the Telegram bot is up (see the
	// "telegram inbox bot" block) — this breaks the
	// bot -> usecase -> dispatcher -> bot cycle.
	pendingReplyUC := inbox.NewPendingReplyUseCase(pendingReplyRepo, nil)

	// Rate limiter for pending-reply approve/reject. Redis-backed when
	// REDIS_URL is set (multi-instance safe); falls back to an in-
	// process sliding-window for single-instance dev/test. Middleware
	// fails open on Limiter errors so a Redis outage does not lock
	// operators out of urgent approvals.
	var pendingReplyLimiter ratelimit.Limiter
	if cfg.RedisURL != "" {
		redisOpt, err := redis.ParseURL(cfg.RedisURL)
		if err != nil {
			log.Fatalf("invalid REDIS_URL: %v", err)
		}
		redisClient := redis.NewClient(redisOpt)
		pingCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		if err := redisClient.Ping(pingCtx).Err(); err != nil {
			slog.Warn("redis ping failed at startup; rate limiter will fail-open until reachable", "err", err)
		}
		cancel()
		pendingReplyLimiter = ratelimit.NewRedisLimiter(redisClient, pendingReplyRateLimit, pendingReplyRateWindow)
	} else {
		slog.Warn("REDIS_URL not set; using in-process rate limiter (single-instance only)")
		pendingReplyLimiter = ratelimit.NewInMemoryLimiter(pendingReplyRateLimit, pendingReplyRateWindow)
	}
	pendingReplyDecideMW := ratelimit.Middleware(pendingReplyLimiter, pendingReplyKeyFn, slog.Default())

	// 4. Adapters
	leadsAI := leads.NewAIAdapter(aiClient)
	seqAI := sequences.NewAIMessageGeneratorAdapter(aiClient)
	prospectReader := sequences.NewProspectReaderAdapter(prospectsRepo)
	leadCreatorAdapter := sequences.NewLeadCreatorAdapter(leadsRepo)
	identityResolver := leads.NewIdentityResolver(identityRepo)
	identityLinker := newIdentityLinkerAdapter(identityResolver, identityRepo)

	// 5. Use cases
	txManager := db.NewTxManager(pool)
	suggestionFinder := newProspectSuggestionFinderAdapter(txManager, leadsRepo, prospectsRepo)
	leadsUC := leads.NewUseCase(leadsRepo, leadsAI, nil,
		leads.WithSuggestionFinder(suggestionFinder),
		leads.WithIdentityReader(identityRepo),
		leads.WithLogger(slog.Default())) // sender set after bot init
	prospectsUC := prospects.NewUseCase(prospectsRepo,
		prospects.WithLeadChecker(newLeadCheckerAdapter(leadsRepo)),
		prospects.WithIdentityLinker(identityLinker))
	sourcesUC := sources.NewUseCase(sourcesRepo, sources.WithStatsReader(sourcesRepo))
	migrateOrphanProspects(pool, ownerID)
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
		verify.RegisterRoutes(r, verify.NewUseCase(prospectsRepo, verify.NewBotTelegramVerifier(nil), proxyDialer)) // TG bot passed as nil for now
		parser.RegisterRoutes(r, cfg.TwoGISAPIKey, httpClient)
		settings.RegisterRoutes(r, settingsUC, buildAITester(cfg, httpClient), buildSMTPTester(proxyDialer), buildResendTester(httpClient), buildUsageCounter(leadsRepo))
		chat.RegisterRoutes(r, chat.NewHandler(chat.NewRepository(pool), newChatAIAdapter(aiClient)))
		inbox.RegisterPendingReplyRoutes(r, pendingReplyUC, leadsUC, pendingReplyDecideMW)
		tgOpts := []tgclient.Option{}
		if proxyDialer != nil {
			tgOpts = append(tgOpts, tgclient.WithDialer(proxyDialer))
		}
		tgclient.RegisterRoutes(r, tgclient.NewClient(tgOpts...), tgclient.NewRepository(pool))
	})

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// 7. Outbound email sender (cron every 30 seconds)
	// Always starts — reads Resend API key from DB each tick (falls back to .env)
	tgRepo := tgclient.NewRepository(pool)
	emailSender := outbound.NewSender(settingsStore, ownerID, cfg.ResendAPIKey, cfg.SMTPFrom, cfg.AppBaseURL, cfg.SMTPHost, cfg.SMTPPort, cfg.SMTPUser, cfg.SMTPPassword, sequencesRepo, prospectsRepo, tgRepo, outbound.NewMTProtoMessenger(), proxyDialer, httpClient)
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
	prospectAdapter := newProspectRepoAdapter(prospectsRepo, txManager)
	inboxLeadAdapter := newInboxLeadRepoAdapter(leadsRepo)
	inboxAI := newInboxAIAdapter(aiClient)
	inboxCfg := newInboxConfigAdapter(settingsStore)
	tgToken := cfg.TelegramBotToken
	if dbCfg, err := settingsStore.GetConfig(context.Background(), ownerID); err == nil && dbCfg.TelegramBotToken != "" {
		tgToken = dbCfg.TelegramBotToken
	}
	if tgToken != "" {
		tgBot, err := inbox.NewTelegramBot(tgToken, inboxLeadAdapter, prospectAdapter, inboxAI, ownerID, cfg.BookingLink, httpClient,
			inbox.WithTelegramIdentityLinker(identityLinker))
		if err != nil {
			log.Printf("telegram bot init failed: %v", err)
		} else {
			// Wire HITL approval: dispatcher uses the bot to deliver
			// approved replies, and the bot uses the usecase to enqueue
			// new proposals. Order matters — SetDispatcher MUST happen
			// before SetPendingProposer so that any inbound message
			// arriving in the gap between bot start and approval flow
			// being fully wired finds at worst a missing dispatcher
			// error (logged) rather than a partially-initialised cycle.
			dispatcher := newTelegramReplyDispatcher(tgBot.Bot(), leadsRepo, inboxLeadAdapter)
			pendingReplyUC.SetDispatcher(dispatcher)
			tgBot.SetPendingProposer(pendingReplyUC)
			go tgBot.Start(ctx)
			// Set the telegram sender on the leads use case
			leadsUC.SetSender(leads.NewTelegramSender(tgBot.Bot()))
		}
	} else {
		// No Telegram token = no bot = no dispatcher. The HITL routes
		// are still registered so the operator UI doesn't 404, but
		// any Approve call will surface ErrPendingReplyDispatcher
		// NotConfigured at runtime. Emit a startup warning so the
		// silent-500-on-approve failure mode is visible in logs.
		log.Println("WARN: telegram token not configured; HITL approve route will return 500 (dispatcher not wired)")
	}

	// 9. Email IMAP poller (reads settings from DB, falls back to .env).
	// The attachment analyser is wired with the same AIClient so the
	// underlying provider's vision capability — when available — is
	// reused for image OCR. Text-only providers degrade gracefully
	// (analyser returns SkipVisionError, lead is still created).
	attachmentAnalyzer := attachments.New(aiClient)
	emailPoller := inbox.NewEmailPoller(inboxCfg, ownerID, cfg.IMAPHost, cfg.IMAPPort, cfg.IMAPUser, cfg.IMAPPassword, inboxLeadAdapter, prospectAdapter, sequencesRepo, inboxAI, proxyDialer,
		inbox.WithAttachmentAnalyzer(attachmentAnalyzer),
		inbox.WithIdentityLinker(identityLinker))
	go emailPoller.Start(ctx)

	// Identity backfill: walk existing leads + prospects once on
	// startup, attach each row to its unified Identity via the same
	// resolver. Idempotent — repeated invocations produce no
	// duplicate links (ON CONFLICT DO NOTHING). Errors per-row are
	// logged and swallowed; ctx cancellation aborts the walk.
	go func() {
		runner := leads.NewIdentityBackfill(newSQLBackfillSource(pool), identityResolver, identityRepo)
		if err := runner.Run(ctx); err != nil {
			log.Printf("identity backfill: %v", err)
		}
	}()

	// 10. Reminders cron (hourly, checks for stale leads)
	var notifier reminders.Notifier
	notifyChatIDStr := os.Getenv("NOTIFY_CHAT_ID")
	if tgToken != "" && notifyChatIDStr != "" {
		chatID, err := strconv.ParseInt(notifyChatIDStr, 10, 64)
		if err == nil {
			n, err := notify.NewTelegramNotifier(tgToken, chatID, httpClient)
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

	// Flush remaining audit entries within the same shutdown budget.
	// Stop drains the buffer and writes one final batch via the repo;
	// ctx-cancel returns ctx.Err but does NOT panic on remaining
	// entries (they're dropped silently for the next process to miss).
	if err := auditRecorder.Stop(shutdownCtx); err != nil {
		log.Printf("audit recorder stop: %v", err)
	}
	log.Println("server stopped")
}

// pendingReplyKeyFn resolves a request to its rate-limit bucket. Keys
// are prefixed so future limiters on other routes do not collide in
// the same Redis namespace, and scoped by user_id so one tenant's
// burst cannot starve another.
func pendingReplyKeyFn(r *http.Request) (string, bool) {
	id, ok := httputil.UserIDFromContext(r.Context())
	if !ok {
		return "", false
	}
	return "ratelimit:pending-replies:" + id.String(), true
}

