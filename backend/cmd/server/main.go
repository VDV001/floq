package main

import (
	"context"
	"flag"
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
	"github.com/daniil/floq/internal/ai/security"
	"github.com/daniil/floq/internal/analytics"
	"github.com/daniil/floq/internal/audit"
	"github.com/daniil/floq/internal/auth"
	"github.com/daniil/floq/internal/chat"
	"github.com/daniil/floq/internal/config"
	"github.com/daniil/floq/internal/db"
	"github.com/daniil/floq/internal/enrichment"
	"github.com/daniil/floq/internal/httputil"
	"github.com/daniil/floq/internal/inbox"
	"github.com/daniil/floq/internal/inbox/attachments"
	"github.com/daniil/floq/internal/integrations/onec"
	"github.com/daniil/floq/internal/leads"
	"github.com/daniil/floq/internal/metrics"
	"github.com/daniil/floq/internal/notify"
	"github.com/daniil/floq/internal/outbound"
	"github.com/daniil/floq/internal/parser"
	"github.com/daniil/floq/internal/prospects"
	"github.com/daniil/floq/internal/proxy"
	"github.com/daniil/floq/internal/ratelimit"
	"github.com/daniil/floq/internal/reminders"
	"github.com/daniil/floq/internal/secrets"
	"github.com/daniil/floq/internal/sequences"
	"github.com/daniil/floq/internal/settings"
	"github.com/daniil/floq/internal/sources"
	"github.com/daniil/floq/internal/tgclient"
	"github.com/daniil/floq/internal/verify"
	"github.com/daniil/floq/internal/webhooks"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
	"github.com/redis/go-redis/v9"
)

// pendingReplyRateWindow is fixed (per-minute is the only sensible
// granularity for human-paced HITL endpoints); the budget itself
// comes from config so ops can tighten on demand without rebuild.
const pendingReplyRateWindow = time.Minute

// Auth rate-limit windows are fixed in the composition root; only the
// per-window budgets come from config (cfg.AuthLoginRateLimit /
// cfg.AuthRegisterRateLimit). Login: 5 attempts / 5 min — sane online
// brute-force protection. Register: 3 / hour — anti-spam without
// blocking a legitimate signup retry.
const (
	authLoginRateWindow    = 5 * time.Minute
	authRegisterRateWindow = time.Hour
)

// verifySecretsExitCode maps the per-module "needs rotation" counts from
// -verify-secrets-kek to a process exit code. Any secret still readable only
// under the old KEK (bad>0 in either module) must block removal of
// FLOQ_SECRETS_KEK_OLD, so a non-zero total yields a non-zero exit.
func verifySecretsExitCode(settingsBad, onecBad int) int {
	if settingsBad+onecBad > 0 {
		return 1
	}
	return 0
}

func main() {
	// Load .env file (ignore error if missing — production uses real env vars).
	_ = godotenv.Load()

	// -backfill-secrets encrypts legacy plaintext secrets into their *_enc
	// columns and exits WITHOUT running migrations. Run it before deploying the
	// migration 047 drop if its guard reports un-backfilled rows. Requires a
	// schema that already has the *_enc columns (>= migration 037).
	backfillSecrets := flag.Bool("backfill-secrets", false,
		"encrypt legacy plaintext secrets into their *_enc columns, then exit")
	// -rotate-secrets re-encrypts every stored secret under the primary KEK
	// (FLOQ_SECRETS_KEK), decrypting via the fallback (FLOQ_SECRETS_KEK_OLD)
	// when needed, then exits WITHOUT running migrations. Run it during a KEK
	// rotation, after deploying with both keys set.
	rotateSecrets := flag.Bool("rotate-secrets", false,
		"re-encrypt every stored secret under the primary KEK, then exit")
	// -verify-secrets-kek is a read-only check that every stored secret
	// decrypts under the PRIMARY KEK alone. It exits non-zero if any secret
	// still needs rotation, so it gates removing FLOQ_SECRETS_KEK_OLD.
	verifySecretsKEK := flag.Bool("verify-secrets-kek", false,
		"verify every stored secret decrypts under the primary KEK (exit non-zero if not), then exit")
	flag.Parse()

	cfg := config.Load()

	// 0a. Secret cipher (at-rest encryption for client credentials). Fail
	// fast: a missing or malformed FLOQ_SECRETS_KEK must crash the server,
	// never fall back to storing credentials in plaintext. The optional
	// FLOQ_SECRETS_KEK_OLD is a decrypt-only fallback active during a rotation
	// window; a present-but-malformed old KEK also fails fast.
	secretCipher, err := secrets.NewCipherWithFallback(cfg.SecretsKEK, cfg.SecretsKEKOld)
	if err != nil {
		log.Fatalf("FLOQ_SECRETS_KEK/FLOQ_SECRETS_KEK_OLD invalid (must be base64-encoded 32 bytes): %v", err)
	}

	// 1. DB pool
	pool, err := pgxpool.New(context.Background(), cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("connect to db: %v", err)
	}
	defer pool.Close()

	// 1a2. One-off secret backfill (run before migration 047). Encrypts any
	// legacy plaintext secret into its *_enc/*_nonce columns, then exits WITHOUT
	// running migrations — so it can prepare a pre-047 schema for the 047
	// drop-guard even when this binary already contains migration 047. Requires
	// a schema between migrations 037 and 047 (the plaintext + *_enc columns
	// must both exist); idempotent (only touches plaintext-set, *_enc-NULL
	// rows). Placed before the proxy/analytics wiring so the command pulls up
	// only the one pool it needs.
	if *backfillSecrets {
		nSettings, err := settings.BackfillSecrets(context.Background(), pool, secretCipher)
		if err != nil {
			log.Fatalf("backfill settings secrets: %v", err)
		}
		nOnec, err := onec.BackfillSecrets(context.Background(), pool, secretCipher)
		if err != nil {
			log.Fatalf("backfill onec secrets: %v", err)
		}
		log.Printf("secret backfill complete: %d settings secrets, %d 1C secrets encrypted", nSettings, nOnec)
		return
	}

	// 1a3. One-off KEK rotation (run during a key rotation, after deploying with
	// both FLOQ_SECRETS_KEK=<new> and FLOQ_SECRETS_KEK_OLD=<old>). Re-encrypts
	// every stored secret under the primary KEK, decrypting old-key ciphertext
	// via the fallback, then exits WITHOUT migrating. Convergent and safe to
	// repeat; aborts loudly on a secret that decrypts under neither key.
	if *rotateSecrets {
		nSettings, err := settings.RotateSecrets(context.Background(), pool, secretCipher)
		if err != nil {
			log.Fatalf("rotate settings secrets: %v", err)
		}
		nOnec, err := onec.RotateSecrets(context.Background(), pool, secretCipher)
		if err != nil {
			log.Fatalf("rotate onec secrets: %v", err)
		}
		log.Printf("secret rotation complete: %d settings secrets, %d 1C secrets re-encrypted under the primary KEK", nSettings, nOnec)
		return
	}

	// 1a4. One-off rotation verification (read-only). Proves every stored secret
	// decrypts under the PRIMARY KEK alone, using a primary-only cipher (no
	// fallback). Exits non-zero if any secret still needs rotation, so it gates
	// safely removing FLOQ_SECRETS_KEK_OLD after a rotation.
	if *verifySecretsKEK {
		primaryOnly, err := secrets.NewCipher(cfg.SecretsKEK)
		if err != nil {
			log.Fatalf("FLOQ_SECRETS_KEK invalid: %v", err)
		}
		okS, badS, err := settings.VerifySecretsKEK(context.Background(), pool, primaryOnly)
		if err != nil {
			log.Fatalf("verify settings secrets: %v", err)
		}
		okO, badO, err := onec.VerifySecretsKEK(context.Background(), pool, primaryOnly)
		if err != nil {
			log.Fatalf("verify onec secrets: %v", err)
		}
		log.Printf("secret KEK verification: under-primary=%d needs-rotation=%d (settings ok=%d bad=%d, 1C ok=%d bad=%d)",
			okS+okO, badS+badO, okS, badS, okO, badO)
		if code := verifySecretsExitCode(badS, badO); code != 0 {
			log.Printf("WARNING: %d secret(s) still need rotation — do NOT remove FLOQ_SECRETS_KEK_OLD", badS+badO)
			os.Exit(code)
		}
		return
	}

	// 0. Proxy provider (empty PROXY_URL = direct connection)
	proxyProvider, err := proxy.NewFromURL(cfg.ProxyURL)
	if err != nil {
		log.Fatalf("invalid PROXY_URL: %v", err)
	}
	httpClient := proxyProvider.HTTPClient()
	proxyDialer := proxyProvider.Dialer() // non-nil for SOCKS5 only

	// 1a. Read-only analytics pool. A separate pool/config from its own DSN
	// so heavy analytics aggregations are isolated from the OLTP path; all
	// analytics reads go through it. In the MVP its DSN defaults to the
	// primary's (same instance, separate pool) — point ANALYTICS_DATABASE_URL
	// at a read replica in production to offload the primary without code
	// changes.
	analyticsPool, err := pgxpool.New(context.Background(), cfg.AnalyticsDatabaseURL)
	if err != nil {
		log.Fatalf("connect to analytics db: %v", err)
	}
	defer analyticsPool.Close()

	// 1b. Run migrations
	migrationsPath := os.Getenv("MIGRATIONS_PATH")
	if migrationsPath == "" {
		migrationsPath = "file://migrations"
	}
	// golang-migrate pgx/v5 driver uses "pgx5://" scheme
	migrateDBURL := strings.Replace(cfg.DatabaseURL, "postgres://", "pgx5://", 1)
	migrated := false
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
		migrated = true
		break
	}
	// Fail fast instead of booting against an un-migrated schema: if every
	// attempt to reach the DB failed, the server would otherwise come up
	// half-initialised and serve against stale/missing tables.
	if !migrated {
		log.Fatalf("migrations: could not connect to the database after 5 attempts")
	}

	// 2. Settings store (reads user_settings from DB, used by services)
	settingsStore := settings.NewStore(pool, secretCipher)
	settingsRepo := settings.NewRepository(pool, secretCipher)
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
	// Prometheus metrics. Built here so the RecordingProvider can feed
	// AI-cost metrics through the observer hook; the HTTP middleware and
	// /metrics endpoint are wired into the router below.
	appMetrics := metrics.New()
	appMetrics.RegisterDropsSource(func() float64 { return float64(auditRecorder.Dropped()) })
	wrappedProvider := audit.NewRecordingProvider(aiProvider, auditRecorder, slog.Default(),
		audit.WithObserver(appMetrics.OnAuditEntry))

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
	// Stamp every proposed reply with the InputFirewall verdict of the
	// inbound message that triggered it, so the dispatch gate can refuse a
	// reply provoked by a Block-flagged payload (agent-security L2).
	pendingReplyUC.SetClassifier(newInboxInputClassifier(security.NewInputFirewall()))

	// Rate limiters. Redis-backed when REDIS_URL is set (multi-instance
	// safe); falls back to an in-process sliding-window for single-
	// instance dev/test. The Middleware fails open on Limiter errors so
	// a Redis outage cannot lock legitimate users out. One Redis client
	// is shared by every limiter so the connection pool is not
	// duplicated per route.
	var redisClient redis.UniversalClient
	if cfg.RedisURL != "" {
		redisOpt, err := redis.ParseURL(cfg.RedisURL)
		if err != nil {
			log.Fatalf("invalid REDIS_URL: %v", err)
		}
		rc := redis.NewClient(redisOpt)
		pingCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		if err := rc.Ping(pingCtx).Err(); err != nil {
			slog.Warn("redis ping failed at startup; rate limiter will fail-open until reachable", "err", err)
		}
		cancel()
		redisClient = rc
	} else {
		slog.Warn("REDIS_URL not set; using in-process rate limiter (single-instance only)")
	}
	newLimiter := func(limit int, window time.Duration) ratelimit.Limiter {
		if redisClient != nil {
			return ratelimit.NewRedisLimiter(redisClient, limit, window)
		}
		return ratelimit.NewInMemoryLimiter(limit, window)
	}

	// HITL approve/reject — keyed per authenticated user_id.
	pendingReplyDecideMW := ratelimit.Middleware(
		newLimiter(cfg.PendingReplyRateLimitPerMin, pendingReplyRateWindow),
		pendingReplyKeyFn, slog.Default())

	// Public auth endpoints — keyed per client IP (caller is not yet
	// authenticated). Separate key prefixes keep login and register
	// buckets from colliding in the shared Redis keyspace.
	authLoginMW := ratelimit.Middleware(
		newLimiter(cfg.AuthLoginRateLimit, authLoginRateWindow),
		ratelimit.IPKeyFunc("ratelimit:auth-login:", cfg.TrustProxyHeaders), slog.Default())
	authRegisterMW := ratelimit.Middleware(
		newLimiter(cfg.AuthRegisterRateLimit, authRegisterRateWindow),
		ratelimit.IPKeyFunc("ratelimit:auth-register:", cfg.TrustProxyHeaders), slog.Default())

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
		leads.WithPendingReplyCounter(newPendingReplyCounterAdapter(pendingReplyRepo)),
		leads.WithLogger(slog.Default())) // sender set after bot init
	// Auto-enrichment (#182): background scraping of a lead/prospect's company
	// domain. The same usecase is the cron worker and the read API, and is
	// injected as the best-effort enqueuer into prospects + inbox.
	// Phase-2 (#186): when ENRICHMENT_LLM_ENABLED, wrap the deterministic HTML
	// extractor in a ChainExtractor that overlays LLM-classified industry/size.
	// Default off (ship dark); the LLM call goes through the audit-recording
	// provider via a cost-capped adapter.
	var enrichmentExtractor enrichment.Extractor = enrichment.NewHTMLExtractor()
	if cfg.EnrichmentLLMEnabled {
		llmExtractor := enrichment.NewLLMExtractor(
			newEnrichmentLLMAdapter(wrappedProvider, cfg.EnrichmentLLMMaxInputRunes, cfg.EnrichmentLLMMaxTokens),
		)
		enrichmentExtractor = enrichment.NewChainExtractor(enrichment.NewHTMLExtractor(), llmExtractor, slog.Default())
		slog.Default().Info("enrichment: LLM extractor enabled (#186)")
	}
	// Phase-3 (#188): when ENRICHMENT_REGISTRY_ENABLED and a DaData key is set,
	// add a best-effort registry Enricher that merges legal details (ИНН/ОГРН/…)
	// after extraction. Ship dark: an empty key keeps it disabled even if the
	// flag is on.
	var enrichmentOpts []enrichment.Option
	if cfg.EnrichmentRegistryEnabled && cfg.DaDataAPIKey != "" {
		registryEnricher := newDaDataEnricher(httpClient, cfg.DaDataAPIKey, "",
			newLimiter(cfg.EnrichmentRegistryRateLimitPerMin, time.Minute))
		registryEnricher.observe = appMetrics.OnRegistryEnrichment
		enrichmentOpts = append(enrichmentOpts, enrichment.WithEnricher(registryEnricher))
		slog.Default().Info("enrichment: registry enricher enabled (#188)")
	}
	enrichmentUC := enrichment.NewUseCase(
		enrichment.NewRepository(pool),
		enrichment.NewWebsiteFetcher(),
		enrichmentExtractor,
		newLimiter(cfg.EnrichmentRateLimitPerMin, time.Minute),
		enrichment.Config{
			TTLSeconds:  cfg.EnrichmentTTLDays * 24 * 60 * 60,
			MaxAttempts: cfg.EnrichmentMaxAttempts,
			BatchLimit:  cfg.EnrichmentBatchLimit,
		},
		slog.Default(), enrichmentOpts...)
	prospectsUC := prospects.NewUseCase(prospectsRepo,
		prospects.WithLeadChecker(newLeadCheckerAdapter(leadsRepo)),
		prospects.WithIdentityLinker(identityLinker),
		prospects.WithEnricher(enrichmentUC))
	sourcesUC := sources.NewUseCase(sourcesRepo, sources.WithStatsReader(sourcesRepo))

	// Outgoing webhooks (#181). Shipped dark — built and mounted only when
	// WEBHOOKS_ENABLED. The delivery worker POSTs signed payloads over an
	// SSRF-hardened client; appMetrics satisfies the DeliveryObserver seam.
	var webhooksUC *webhooks.UseCase
	if cfg.WebhooksEnabled {
		webhooksUC = webhooks.NewUseCase(
			webhooks.NewRepository(pool, secretCipher),
			webhooks.NewHTTPDeliveryClient(),
			webhooks.Config{MaxAttempts: cfg.WebhooksMaxAttempts, BatchLimit: cfg.WebhooksBatchLimit},
			appMetrics,
			slog.Default(),
		)
	}
	// Bridge domain state changes (lead/inbox/outbound events) to the outgoing-
	// webhooks outbox as transactional emitters (#199). The emitter wiring below
	// is gated on webhooks being enabled (webhookPub != nil) so a disabled feature
	// keeps the legacy non-transactional write paths.
	var webhookPub eventPublisher
	if webhooksUC != nil {
		webhookPub = webhooksUC
	}
	webhookBridge := newWebhookEventPublisher(webhookPub, slog.Default())
	migrateOrphanProspects(pool, ownerID)
	emailConfigChecker := emailConfigCheckerAdapter{
		store:           settingsStore,
		envResendKey:    cfg.ResendAPIKey,
		envSMTPHost:     cfg.SMTPHost,
		envSMTPUser:     cfg.SMTPUser,
		envSMTPPassword: cfg.SMTPPassword,
	}
	autopilotChecker := autopilotCheckerAdapter{settings: settingsRepo}
	sequencesUC := sequences.NewUseCase(sequencesRepo, seqAI, prospectReader, leadCreatorAdapter,
		sequences.WithTxManager(txManager),
		sequences.WithEmailConfigChecker(emailConfigChecker),
		sequences.WithAutopilotChecker(autopilotChecker))

	// 5. Auth
	authHandler := auth.NewHandler(auth.NewRepository(pool), cfg.JWTSecret)

	// 6. Router
	r := chi.NewRouter()
	// Metrics middleware is outermost so it observes the FINAL status,
	// including the 500 the Recoverer writes for a panicked handler.
	r.Use(appMetrics.HTTPMiddleware)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(corsMiddleware)
	// Defence in depth on request body size:
	//   1) MaxBodyBytesWithUploads (outer) — unconditional ceiling so a client
	//      omitting or spoofing Content-Type cannot stream past the cap. General
	//      routes get 10 MiB; the multipart CSV-import routes (/…/import) get
	//      50 MiB so legitimate enterprise imports fit. Path-based rather than a
	//      per-route override because MaxBytesReader is smallest-wins — a higher
	//      inner cap cannot loosen a lower ancestor (#99).
	//   2) JSONBodyCap (inner, 1 MiB) — tighter cap that fires only for
	//      application/json bodies; MaxBytesReader composes downward so
	//      JSON clients trip the inner cap first.
	//   3) Handler-local caps (e.g. bulk endpoint's 256 KiB) still win
	//      when wrapped after the middleware — smallest in chain wins.
	r.Use(httputil.MaxBodyBytesWithUploads(httputil.DefaultMaxBodyBytes, httputil.DefaultMaxUploadBytes))
	r.Use(httputil.JSONBodyCap(httputil.DefaultMaxJSONBodyBytes))

	// Health (public)
	r.Get("/api/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})

	// Prometheus scrape endpoint (public, no auth — pull model). The
	// HTTP middleware skips this route so scrapes don't self-inflate.
	r.Handle("/metrics", appMetrics.Handler())

	// Tracking pixel (public, no auth — loaded by email clients)
	sequences.RegisterPublicRoutes(r, sequencesUC)

	// Unsubscribe (public — authorized by the signed token in the URL, not JWT;
	// reached from email link clicks and RFC 8058 one-click POSTs).
	prospects.RegisterUnsubscribeRoutes(r, prospects.NewUnsubscribeService(prospectsRepo, cfg.JWTSecret))

	// Auth (public) — login/register rate-limited per client IP.
	auth.RegisterRoutes(r, authHandler, authLoginMW, authRegisterMW)

	// 1C inbound webhook (public — authenticated by per-user secret, not JWT).
	// Mapping resolves event kinds + counterparty fields; the applier routes
	// mapped events to leads/prospects via a cross-context adapter.
	onecRepo := onec.NewRepository(pool, secretCipher)
	onecApplier := newOnecApplierAdapter(leadsRepo, leadsUC, prospectsUC, slog.Default())
	onecUC := onec.NewUseCase(onecRepo,
		onec.WithMapping(onecRepo),
		onec.WithApplier(onecApplier),
		onec.WithLogger(slog.Default()))
	onec.RegisterRoutes(r, onec.NewHandler(onecUC), onecRepo)

	// Floq→1C outbound: one proxy-aware HTTP client serves both the counterparty
	// write path and the reconciliation read path.
	onecClient := onec.NewHTTPClient(httpClient)
	onecOutboundUC := onec.NewOutboundUseCase(onecRepo, onecClient, slog.Default())
	// Qualification notifies the 1C counterparty push post-commit (best-effort,
	// #108): it detaches into its own goroutine and must never fail or roll back
	// qualification, so it stays a post-commit observer.
	leadsUC.SetQualificationObserver(newOnecQualificationAdapter(onecOutboundUC, slog.Default()))
	// Webhook emission is transactional (#199): the lead.qualified / lead.archived
	// / pending_reply.approved rows are enqueued in the same transaction as their
	// domain write. Wired only when webhooks are enabled, so a disabled feature
	// keeps the legacy non-transactional write path (no extra transaction).
	if webhookPub != nil {
		leadsUC.SetTxManager(txManager)
		leadsUC.SetQualificationEmitter(webhookBridge)
		leadsUC.SetLeadArchivedEmitter(webhookBridge)
		pendingReplyUC.SetTxManager(txManager)
		pendingReplyUC.SetApprovedEmitter(webhookBridge)
	}

	// Reconciliation safety net (#109): re-feed recent 1C events through the
	// inbound use case to recover any webhook that was lost. Cron started below
	// once the app-lifecycle context exists.
	onecReconcileUC := onec.NewReconcileUseCase(onecRepo, onecClient, onecUC, slog.Default())

	// Protected routes
	r.Group(func(r chi.Router) {
		r.Use(auth.AuthMiddleware(cfg.JWTSecret))
		leads.RegisterRoutes(r, leadsUC)
		prospects.RegisterRoutes(r, prospectsUC)
		enrichment.RegisterRoutes(r, enrichmentUC)
		if webhooksUC != nil {
			webhooks.RegisterRoutes(r, webhooksUC)
		}
		sequences.RegisterRoutes(r, sequencesUC)
		sources.RegisterRoutes(r, sourcesUC)
		verify.RegisterRoutes(r, verify.NewUseCase(prospectsRepo, verify.NewBotTelegramVerifier(nil), proxyDialer)) // TG bot passed as nil for now
		parser.RegisterRoutes(r, cfg.TwoGISAPIKey, httpClient)
		settings.RegisterRoutes(r, settingsUC, buildAITester(cfg, httpClient), buildSMTPTester(proxyDialer), buildResendTester(httpClient), buildUsageCounter(leadsRepo))
		chat.RegisterRoutes(r, chat.NewHandler(chat.NewRepository(pool), newChatAIAdapter(aiClient)))
		inbox.RegisterPendingReplyRoutes(r, pendingReplyUC, leadsUC, pendingReplyDecideMW)
		audit.RegisterRoutes(r, audit.NewHandler(audit.NewUseCase(auditRepo)))
		onec.RegisterConfigRoutes(r, onec.NewConfigHandler(
			onec.NewConfigUseCase(onecRepo, onecRepo, onecClient, buildOnecSecretGenerator())))
		// All analytics reads go through the isolated analytics pool.
		analyticsRepo := analytics.NewRepository(analyticsPool)
		analytics.RegisterRoutes(r, analytics.NewUseCase(analyticsRepo, analyticsRepo,
			analytics.WithHotLeadsReader(analyticsRepo),
			analytics.WithInboxFlowReader(analyticsRepo),
			analytics.WithFunnelReader(analyticsRepo),
			analytics.WithScoreBucketStep(cfg.AnalyticsScoreBucketStep)))
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
	// Sign unsubscribe tokens with the same secret the public /unsubscribe route
	// verifies them with, so campaign emails carry working one-click links.
	emailSender.SetUnsubscribeSecret(cfg.JWTSecret)
	// agent-security-defaults layer 3: validate channel/recipient schema and
	// hold unconfirmed mass sends before the outbound dispatch loop.
	emailSender.SetSendGuard(newOutboundGuardAdapter(security.NewOutboundGuard(security.OutboundPolicy{
		AllowedChannels:   []string{"email", "telegram"},
		MassSendThreshold: cfg.OutboundMassSendThreshold,
		MassSendConfirmed: cfg.OutboundMassSendConfirmed,
	})))
	// Emit sequence.completed to outgoing webhooks when a prospect's sequence run
	// finishes sending — transactionally with the dispatch's sent/bounced mark
	// (#199). Only wired when webhooks are enabled so a disabled feature adds no
	// per-send completion-count query or transaction.
	if webhookPub != nil {
		emailSender.SetTxManager(txManager)
		emailSender.SetSequenceCompletionEmitter(webhookBridge)
	}
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

	// 1C reconciliation cron — recovers lost webhooks every 15 minutes, stops on ctx.
	go onec.NewReconcileCron(onecReconcileUC, 15*time.Minute, slog.Default()).Start(ctx)

	// Audit-log retention cron (#101) — rolls per-call rows older than the
	// retention window into audit_log_daily and purges them, bounding
	// unbounded growth of the cost ledger. Stops on ctx.
	auditRetentionUC := audit.NewRetentionUseCase(auditRepo, cfg.AuditRetentionDays)
	go audit.NewRetentionCron(auditRetentionUC, cfg.AuditRetentionInterval, slog.Default()).Start(ctx)

	// Auto-enrichment worker (#182): scrapes due company domains each tick.
	go enrichment.NewEnrichmentCron(enrichmentUC, cfg.EnrichmentRefreshInterval, slog.Default()).Start(ctx)
	if webhooksUC != nil {
		go webhooks.NewDeliveryCron(webhooksUC, cfg.WebhooksRefreshInterval, slog.Default()).Start(ctx)
	}

	// Analytics matview refresh cron — rebuilds the funnel materialized views
	// CONCURRENTLY off the OLTP path so the dashboard serves fresh aggregates
	// without running the heavy GROUP BYs inline. Reads through the analytics
	// pool; stops on ctx.
	go analytics.NewRefreshCron(analytics.NewRepository(analyticsPool), cfg.AnalyticsRefreshInterval, slog.Default(),
		analytics.WithRefreshObserver(appMetrics.ObserveMatviewRefresh)).Start(ctx)

	// Queue-depth metric (#94) — periodically publishes the pending-reply
	// backlog (aggregate by kind) into Prometheus. Stops on ctx.
	go appMetrics.StartQueueScanner(ctx, queueDepthAdapter{repo: pendingReplyRepo}, 30*time.Second, slog.Default())

	// 8. Optional: Telegram inbox bot
	// Read token from DB first, fall back to .env
	prospectAdapter := newProspectRepoAdapter(prospectsRepo, txManager)
	inboxLeadAdapter := newInboxLeadRepoAdapter(leadsRepo)
	// agent-security-defaults layer 1: the inbound payload passes the input
	// firewall before it can reach the qualification LLM.
	inboxAI := newGuardedQualifier(newInboxAIAdapter(aiClient), security.NewInputFirewall(), security.NewPIIScrubber(), security.NewOutputValidator(security.DefaultMinConfidence), security.NewDefaultCostBreaker(), slog.Default())
	inboxCfg := newInboxConfigAdapter(settingsStore)
	tgToken := cfg.TelegramBotToken
	if dbCfg, err := settingsStore.GetConfig(context.Background(), ownerID); err == nil && dbCfg.TelegramBotToken != "" {
		tgToken = dbCfg.TelegramBotToken
	}
	// Email-side HITL dispatcher is always wired; the email branch in
	// the channel router uses outbound.Sender (built above) via an
	// adapter, so it works even when no Telegram bot is configured.
	emailHITLSender := newInboxEmailSenderAdapter(emailSender)
	// Resolves a reply's channel destination (telegram chat id / email) by
	// lead id, mapping leadsdomain.Lead -> inbox.ReplyTarget so the inbox
	// dispatchers stay free of the leads domain.
	leadReplyTargets := newLeadReplyTargetAdapter(leadsRepo)
	emailDispatcher := inbox.NewEmailReplyDispatcher(emailHITLSender, leadReplyTargets, inboxLeadAdapter)

	// L2 reply gate (agent-security): both wiring branches below route the
	// channel dispatcher through the tool-call firewall, so a reply whose
	// inbound trigger was Block-flagged is refused at send even after
	// operator approval. KnownActions is set per the standard (default-deny
	// unknown actions); the two send_* actions are the only ones this path
	// ever inspects.
	replyFirewall := security.NewToolCallFirewall(security.ToolCallPolicy{
		KnownActions: []string{"send_email", "send_telegram"},
	})
	guardReply := func(d inbox.ReplyDispatcher) inbox.ReplyDispatcher {
		return newGuardedReplyDispatcher(d, replyFirewall, slog.Default())
	}

	var telegramDispatcher inbox.ReplyDispatcher
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
			telegramDispatcher = inbox.NewTelegramReplyDispatcher(tgBot.Bot(), leadReplyTargets, inboxLeadAdapter)
			pendingReplyUC.SetDispatcher(guardReply(inbox.NewChannelReplyDispatcher(telegramDispatcher, emailDispatcher)))
			tgBot.SetPendingProposer(pendingReplyUC)
			if webhookPub != nil {
				// Poller intake events are best-effort post-commit (#206): no tx.
				tgBot.SetLeadCreatedEmitter(webhookBridge)
				tgBot.SetLeadQualifiedEmitter(inboxLeadQualifiedEmitterFunc(webhookBridge.emitInboxLeadQualified))
			}
			go tgBot.Start(ctx)
			// Set the telegram sender on the leads use case
			leadsUC.SetSender(leads.NewTelegramSender(tgBot.Bot()))
		}
	} else {
		// No Telegram token = no telegram bot = no telegram dispatch
		// branch. The email branch still works through the channel
		// router; an approve on a telegram-channel pending reply will
		// surface ErrChannelDispatcherUnsupported instead of the older
		// dispatcher-not-configured 500.
		log.Println("WARN: telegram token not configured; HITL telegram dispatch will return unsupported-channel error")
	}
	if telegramDispatcher == nil {
		// Wire the router with only the email branch so the email
		// HITL surface works in deployments without a Telegram bot.
		pendingReplyUC.SetDispatcher(guardReply(inbox.NewChannelReplyDispatcher(nil, emailDispatcher)))
	}

	// 9. Email IMAP poller (reads settings from DB, falls back to .env).
	// The attachment analyser is wired with the same AIClient so the
	// underlying provider's vision capability — when available — is
	// reused for image OCR. Text-only providers degrade gracefully
	// (analyser returns SkipVisionError, lead is still created).
	attachmentAnalyzer := attachments.New(aiClient)
	emailPoller := inbox.NewEmailPoller(inboxCfg, ownerID, cfg.IMAPHost, cfg.IMAPPort, cfg.IMAPUser, cfg.IMAPPassword, inboxLeadAdapter, prospectAdapter, sequencesRepo, inboxAI, proxyDialer,
		inbox.WithAttachmentAnalyzer(attachmentAnalyzer),
		inbox.WithIdentityLinker(identityLinker),
		inbox.WithEmailEnricher(enrichmentUC),
		inbox.WithEmailBookingLink(cfg.BookingLink))
	// Cycle-break the same way the telegram bot does: poller depends
	// on a proposer that depends on a dispatcher that may depend on
	// the poller's collaborators. SetPendingProposer after the poller
	// is constructed but before Start so no inbound email is processed
	// without the HITL gate wired.
	emailPoller.SetPendingProposer(pendingReplyUC)
	if webhookPub != nil {
		// #206: lead.created is fail-closed — CreateLead + enqueue commit in one
		// tx; on failure the email is left unseen for retry. lead.qualified
		// (auto-qualification) stays best-effort post-commit until #206 Part C.
		emailPoller.SetLeadCreatedEmitter(webhookBridge)
		emailPoller.SetTxManager(txManager)
		emailPoller.SetLeadQualifiedEmitter(inboxLeadQualifiedEmitterFunc(webhookBridge.emitInboxLeadQualified))
	}
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
//
// IMPORTANT: returning ok=false bypasses the limiter entirely — this
// hinges on auth.AuthMiddleware running FIRST so the user_id is in
// context for every request that reaches this point. Mounting the
// pending-reply routes outside the protected group would silently
// disable rate-limiting (every request falls back to bypass). See
// the wire-up in main.go where decideMW is passed into
// RegisterPendingReplyRoutes inside the auth.AuthMiddleware-scoped
// chi.Group block.
func pendingReplyKeyFn(r *http.Request) (string, bool) {
	id, ok := httputil.UserIDFromContext(r.Context())
	if !ok {
		return "", false
	}
	return "ratelimit:pending-replies:" + id.String(), true
}
