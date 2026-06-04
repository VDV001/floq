package config

import (
	"os"
	"strconv"
	"time"
)

// Config holds all application configuration read from environment variables.
type Config struct {
	AppPort         string
	DatabaseURL     string
	RedisURL        string
	JWTSecret       string
	AIProvider      string
	AnthropicAPIKey string
	OpenAIAPIKey    string
	OpenAIModel     string
	OllamaBaseURL   string
	OllamaModel     string
	GroqAPIKey      string
	GroqModel       string
	TelegramBotToken string
	ResendAPIKey    string
	SMTPFrom        string
	IMAPHost        string
	IMAPPort        string
	IMAPUser        string
	IMAPPassword    string
	SMTPHost        string
	SMTPPort        string
	SMTPUser        string
	SMTPPassword    string
	OwnerUserID     string
	AppBaseURL      string
	TwoGISAPIKey    string
	BookingLink     string
	SenderName      string
	SenderCompany   string
	SenderPhone     string
	SenderWebsite   string
	StaleDays       int
	ProxyURL        string
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
}

// Load reads configuration from environment variables and returns a Config.
func Load() *Config {
	return &Config{
		AppPort:         getEnv("APP_PORT", "8080"),
		DatabaseURL:     os.Getenv("DATABASE_URL"),
		RedisURL:        os.Getenv("REDIS_URL"),
		JWTSecret:       os.Getenv("JWT_SECRET"),
		AIProvider:      getEnv("AI_PROVIDER", "anthropic"),
		AnthropicAPIKey: os.Getenv("ANTHROPIC_API_KEY"),
		OpenAIAPIKey:    os.Getenv("OPENAI_API_KEY"),
		OpenAIModel:     getEnv("OPENAI_MODEL", "gpt-4o"),
		OllamaBaseURL:   getEnv("OLLAMA_BASE_URL", "http://localhost:11434"),
		OllamaModel:     getEnv("OLLAMA_MODEL", "llama3"),
		GroqAPIKey:      os.Getenv("GROQ_API_KEY"),
		GroqModel:       getEnv("GROQ_MODEL", "openai/gpt-oss-120b"),
		TelegramBotToken: os.Getenv("TELEGRAM_BOT_TOKEN"),
		ResendAPIKey:    os.Getenv("RESEND_API_KEY"),
		SMTPFrom:        os.Getenv("SMTP_FROM"),
		IMAPHost:        os.Getenv("IMAP_HOST"),
		IMAPPort:        getEnv("IMAP_PORT", "993"),
		IMAPUser:        os.Getenv("IMAP_USER"),
		IMAPPassword:    os.Getenv("IMAP_PASSWORD"),
		SMTPHost:        os.Getenv("SMTP_HOST"),
		SMTPPort:        getEnv("SMTP_PORT", "465"),
		SMTPUser:        os.Getenv("SMTP_USER"),
		SMTPPassword:    os.Getenv("SMTP_PASSWORD"),
		OwnerUserID:     getEnv("OWNER_USER_ID", "00000000-0000-0000-0000-000000000001"),
		AppBaseURL:      os.Getenv("APP_BASE_URL"),
		TwoGISAPIKey:    os.Getenv("TWOGIS_API_KEY"),
		BookingLink:     getEnv("BOOKING_LINK", "https://calendar.app.google/CQciFBayHqi6CstB7"),
		SenderName:      getEnv("SENDER_NAME", "Дмитрий"),
		SenderCompany:   getEnv("SENDER_COMPANY", "dev-bot.su"),
		SenderPhone:     os.Getenv("SENDER_PHONE"),
		SenderWebsite:   os.Getenv("SENDER_WEBSITE"),
		StaleDays:       getEnvInt("STALE_DAYS", 2),
		ProxyURL:        os.Getenv("PROXY_URL"),
		PendingReplyRateLimitPerMin: getEnvInt("RATE_LIMIT_PENDING_REPLIES_PER_MIN", 30),
		AuthLoginRateLimit:          getEnvInt("AUTH_LOGIN_RATE_LIMIT", 5),
		AuthRegisterRateLimit:       getEnvInt("AUTH_REGISTER_RATE_LIMIT", 3),
		TrustProxyHeaders:           getEnvBool("TRUST_PROXY", false),
		AuditRetentionDays:          getEnvInt("AUDIT_RETENTION_DAYS", 30),
		AuditRetentionInterval:      getEnvDuration("AUDIT_RETENTION_INTERVAL", 24*time.Hour),
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
