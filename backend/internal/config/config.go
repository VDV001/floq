package config

import (
	"os"
	"strconv"
	"time"
)

// Config holds all application configuration read from environment variables.
type Config struct {
	AppPort     string
	DatabaseURL string
	// AnalyticsDatabaseURL is the DSN for the read-only analytics pool. It
	// defaults to DatabaseURL so the MVP runs against the same instance with
	// a separate pool/config; point it at a read replica in production to
	// move heavy analytics aggregations off the OLTP primary without code
	// changes.
	AnalyticsDatabaseURL string
	// AnalyticsRefreshInterval is how often the background cron rebuilds the
	// analytics materialized views (REFRESH ... CONCURRENTLY). Default 5m —
	// the funnel dashboard tolerates minutes-stale aggregates.
	AnalyticsRefreshInterval time.Duration
	// AnalyticsScoreBucketStep is the qualification-score histogram bucket
	// width for the funnel distribution, normalised to a multiple of 10 in
	// [10, 100]. Default 10.
	AnalyticsScoreBucketStep int
	RedisURL                 string
	JWTSecret                string
	// SecretsKEK is the base64-encoded 32-byte key-encryption-key used to
	// encrypt client credentials at rest. Validated at startup by
	// secrets.NewCipher — the server fails fast if it is missing or not 32
	// bytes, so credentials are never silently stored in plaintext.
	SecretsKEK string
	// SecretsKEKOld is an OPTIONAL base64-encoded 32-byte key-encryption-key
	// used only as a decrypt-fallback during a KEK rotation: secrets still
	// sealed under the previous key stay readable while `server -rotate-secrets`
	// re-encrypts them under SecretsKEK. Leave unset in steady state; set it to
	// the previous KEK during rotation and remove it once `-verify-secrets-kek`
	// reports zero secrets still needing rotation.
	SecretsKEKOld    string
	AIProvider       string
	AnthropicAPIKey  string
	OpenAIAPIKey     string
	OpenAIModel      string
	OllamaBaseURL    string
	OllamaModel      string
	GroqAPIKey       string
	GroqModel        string
	TelegramBotToken string
	ResendAPIKey     string
	SMTPFrom         string
	IMAPHost         string
	IMAPPort         string
	IMAPUser         string
	IMAPPassword     string
	SMTPHost         string
	SMTPPort         string
	SMTPUser         string
	SMTPPassword     string
	OwnerUserID      string
	AppBaseURL       string
	TwoGISAPIKey     string
	BookingLink      string
	SenderName       string
	SenderCompany    string
	SenderPhone      string
	SenderWebsite    string
	StaleDays        int
	ProxyURL         string
	// PendingReplyRateLimitPerMin caps combined approve+reject requests
	// per user per minute on the HITL endpoints. Default 30 — over an
	// order of magnitude above any legitimate human cadence, still
	// capped enough to bound abuse from a compromised JWT.
	PendingReplyRateLimitPerMin int
	// AuthLoginRateLimit caps login attempts per client IP within the
	// login window (5 minutes, fixed in the composition root). Default 5
	// — enough headroom for a fat-fingered human, tight enough to make
	// online brute-force / credential-stuffing impractical.
	AuthLoginRateLimit int
	// AuthRegisterRateLimit caps sign-ups per client IP within the
	// register window (1 hour, fixed in the composition root). Default 3
	// — anti-spam without blocking a legitimate signup retry.
	AuthRegisterRateLimit int
	// TrustProxyHeaders decides whether X-Forwarded-For / X-Real-IP are
	// believed when resolving the client IP for per-IP rate limits.
	// Default false: the committed deploy binds the backend directly
	// (no reverse proxy), where those headers are attacker-controlled
	// and trusting them would let a single host rotate the header to
	// dodge the cap. Set TRUST_PROXY=true only when a reverse proxy that
	// overwrites these headers sits in front of the app.
	TrustProxyHeaders bool
	// AuditRetentionDays is how long per-call audit_log rows are kept
	// before the retention cron aggregates them into audit_log_daily and
	// deletes them. Default 30 — matches the cost-summary default
	// lookback, so recent reports still hit the detailed table.
	AuditRetentionDays int
	// AuditRetentionInterval is how often the retention cron runs.
	// Default 24h — the rollup is day-granular, so sub-daily passes
	// would only add churn.
	AuditRetentionInterval time.Duration
	// OutboundMassSendThreshold: a single dispatch batch larger than this is
	// treated as a mass send and held until OutboundMassSendConfirmed is set
	// (agent-security-defaults layer 3, blast-radius bound). Default 500.
	OutboundMassSendThreshold int
	// OutboundMassSendConfirmed is the out-of-band confirmation that large
	// batches are intended. Default false — an unconfirmed over-threshold
	// batch is held rather than blasted.
	OutboundMassSendConfirmed bool
	// Auto-enrichment (#182): the background worker scrapes a lead/prospect's
	// company domain from its public website.
	EnrichmentRefreshInterval time.Duration // worker tick; default 1m
	EnrichmentTTLDays         int           // how long an enriched record stays fresh; default 30
	EnrichmentMaxAttempts     int           // give up after this many failures; default 3
	EnrichmentBatchLimit      int           // rows scraped per tick; default 20
	EnrichmentRateLimitPerMin int           // per-domain scrape budget per minute; default 10
	// Phase-2 (#186): LLM extraction of industry/company_size from the scraped
	// page. Shipped dark — default off — until validated; cost is bounded by
	// the input/output caps below plus the per-domain scrape rate limit above.
	EnrichmentLLMEnabled       bool // enable the LLM extractor; default false
	EnrichmentLLMMaxInputRunes int  // cap page sent to the LLM; default 8000
	EnrichmentLLMMaxTokens     int  // output token cap; default 64
	// Phase-3 (#188): registry lookup of legal details via DaData. Ship dark —
	// disabled unless explicitly enabled AND an API key is set.
	EnrichmentRegistryEnabled         bool   // enable the registry Enricher; default false
	DaDataAPIKey                      string // DaData API token; empty disables registry
	EnrichmentRegistryRateLimitPerMin int    // global DaData egress cap/min; default 30
	// Outgoing webhooks (#181): per-user subscriptions delivered by a background
	// worker over an SSRF-hardened client. Shipped dark — default off — until
	// the delivery path is validated end-to-end.
	WebhooksEnabled         bool          // enable the webhook delivery worker + API; default false
	WebhooksRefreshInterval time.Duration // delivery worker tick; default 30s
	WebhooksMaxAttempts     int           // give up after this many failed deliveries; default 5
	WebhooksBatchLimit      int           // deliveries claimed per tick; default 50

	// Auto-qualification worker (#206 Part C): drains lead_qualification_jobs.
	QualificationRefreshInterval time.Duration // qualification worker tick; default 10s
	QualificationMaxAttempts     int           // give up after this many failed qualifications; default 5
	QualificationBatchLimit      int           // jobs claimed per tick; default 50

	// Intake retry cap (#208): consecutive fail-closed retries per source
	// (email UID / telegram update_id) before a poison source is quarantined.
	IntakeMaxAttempts int // default 10
}

// Load reads configuration from environment variables and returns a Config.
func Load() *Config {
	return &Config{
		AppPort:                           getEnv("APP_PORT", "8080"),
		DatabaseURL:                       os.Getenv("DATABASE_URL"),
		AnalyticsDatabaseURL:              getEnv("ANALYTICS_DATABASE_URL", os.Getenv("DATABASE_URL")),
		AnalyticsRefreshInterval:          getEnvDuration("ANALYTICS_REFRESH_INTERVAL", 5*time.Minute),
		AnalyticsScoreBucketStep:          getEnvInt("ANALYTICS_SCORE_BUCKET_STEP", 10),
		RedisURL:                          os.Getenv("REDIS_URL"),
		JWTSecret:                         os.Getenv("JWT_SECRET"),
		SecretsKEK:                        os.Getenv("FLOQ_SECRETS_KEK"),
		SecretsKEKOld:                     os.Getenv("FLOQ_SECRETS_KEK_OLD"),
		EnrichmentRefreshInterval:         getEnvDuration("ENRICHMENT_REFRESH_INTERVAL", time.Minute),
		EnrichmentTTLDays:                 getEnvInt("ENRICHMENT_TTL_DAYS", 30),
		EnrichmentMaxAttempts:             getEnvInt("ENRICHMENT_MAX_ATTEMPTS", 3),
		EnrichmentBatchLimit:              getEnvInt("ENRICHMENT_BATCH_LIMIT", 20),
		EnrichmentRateLimitPerMin:         getEnvInt("ENRICHMENT_RATE_LIMIT_PER_MIN", 10),
		EnrichmentLLMEnabled:              getEnvBool("ENRICHMENT_LLM_ENABLED", false),
		EnrichmentLLMMaxInputRunes:        getEnvInt("ENRICHMENT_LLM_MAX_INPUT_RUNES", 8000),
		EnrichmentLLMMaxTokens:            getEnvInt("ENRICHMENT_LLM_MAX_TOKENS", 64),
		EnrichmentRegistryEnabled:         getEnvBool("ENRICHMENT_REGISTRY_ENABLED", false),
		DaDataAPIKey:                      os.Getenv("DADATA_API_KEY"),
		EnrichmentRegistryRateLimitPerMin: getEnvInt("ENRICHMENT_REGISTRY_RATE_LIMIT_PER_MIN", 30),
		WebhooksEnabled:                   getEnvBool("WEBHOOKS_ENABLED", false),
		WebhooksRefreshInterval:           getEnvDuration("WEBHOOKS_REFRESH_INTERVAL", 30*time.Second),
		WebhooksMaxAttempts:               getEnvInt("WEBHOOKS_MAX_ATTEMPTS", 5),
		WebhooksBatchLimit:                getEnvInt("WEBHOOKS_BATCH_LIMIT", 50),
		QualificationRefreshInterval:      getEnvDuration("QUALIFICATION_REFRESH_INTERVAL", 10*time.Second),
		QualificationMaxAttempts:          getEnvInt("QUALIFICATION_MAX_ATTEMPTS", 5),
		QualificationBatchLimit:           getEnvInt("QUALIFICATION_BATCH_LIMIT", 50),
		IntakeMaxAttempts:                 getEnvInt("INBOX_INTAKE_MAX_ATTEMPTS", 10),
		AIProvider:                        getEnv("AI_PROVIDER", "anthropic"),
		AnthropicAPIKey:                   os.Getenv("ANTHROPIC_API_KEY"),
		OpenAIAPIKey:                      os.Getenv("OPENAI_API_KEY"),
		OpenAIModel:                       getEnv("OPENAI_MODEL", "gpt-4o"),
		OllamaBaseURL:                     getEnv("OLLAMA_BASE_URL", "http://localhost:11434"),
		OllamaModel:                       getEnv("OLLAMA_MODEL", "llama3"),
		GroqAPIKey:                        os.Getenv("GROQ_API_KEY"),
		GroqModel:                         getEnv("GROQ_MODEL", "openai/gpt-oss-120b"),
		TelegramBotToken:                  os.Getenv("TELEGRAM_BOT_TOKEN"),
		ResendAPIKey:                      os.Getenv("RESEND_API_KEY"),
		SMTPFrom:                          os.Getenv("SMTP_FROM"),
		IMAPHost:                          os.Getenv("IMAP_HOST"),
		IMAPPort:                          getEnv("IMAP_PORT", "993"),
		IMAPUser:                          os.Getenv("IMAP_USER"),
		IMAPPassword:                      os.Getenv("IMAP_PASSWORD"),
		SMTPHost:                          os.Getenv("SMTP_HOST"),
		SMTPPort:                          getEnv("SMTP_PORT", "465"),
		SMTPUser:                          os.Getenv("SMTP_USER"),
		SMTPPassword:                      os.Getenv("SMTP_PASSWORD"),
		OwnerUserID:                       getEnv("OWNER_USER_ID", "00000000-0000-0000-0000-000000000001"),
		AppBaseURL:                        os.Getenv("APP_BASE_URL"),
		TwoGISAPIKey:                      os.Getenv("TWOGIS_API_KEY"),
		BookingLink:                       getEnv("BOOKING_LINK", "https://calendar.app.google/CQciFBayHqi6CstB7"),
		SenderName:                        getEnv("SENDER_NAME", "Дмитрий"),
		SenderCompany:                     getEnv("SENDER_COMPANY", "dev-bot.su"),
		SenderPhone:                       os.Getenv("SENDER_PHONE"),
		SenderWebsite:                     os.Getenv("SENDER_WEBSITE"),
		StaleDays:                         getEnvInt("STALE_DAYS", 2),
		ProxyURL:                          os.Getenv("PROXY_URL"),
		PendingReplyRateLimitPerMin:       getEnvInt("RATE_LIMIT_PENDING_REPLIES_PER_MIN", 30),
		AuthLoginRateLimit:                getEnvInt("AUTH_LOGIN_RATE_LIMIT", 5),
		AuthRegisterRateLimit:             getEnvInt("AUTH_REGISTER_RATE_LIMIT", 3),
		TrustProxyHeaders:                 getEnvBool("TRUST_PROXY", false),
		AuditRetentionDays:                getEnvInt("AUDIT_RETENTION_DAYS", 30),
		AuditRetentionInterval:            getEnvDuration("AUDIT_RETENTION_INTERVAL", 24*time.Hour),
		OutboundMassSendThreshold:         getEnvInt("OUTBOUND_MASS_SEND_THRESHOLD", 500),
		OutboundMassSendConfirmed:         getEnvBool("OUTBOUND_MASS_SEND_CONFIRMED", false),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}

func getEnvBool(key string, fallback bool) bool {
	if v := os.Getenv(key); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			return b
		}
	}
	return fallback
}

func getEnvDuration(key string, fallback time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return fallback
}
