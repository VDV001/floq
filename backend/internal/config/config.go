package config

import (
	"os"
	"strconv"
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
