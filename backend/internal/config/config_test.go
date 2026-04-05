package config

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoad_Defaults(t *testing.T) {
	// Unset all env vars that Load reads with defaults to ensure we get fallback values.
	for _, key := range []string{
		"APP_PORT", "AI_PROVIDER", "OPENAI_MODEL", "OLLAMA_BASE_URL",
		"OLLAMA_MODEL", "GROQ_MODEL", "IMAP_PORT", "OWNER_USER_ID",
		"BOOKING_LINK", "SENDER_NAME", "SENDER_COMPANY", "STALE_DAYS",
		"DATABASE_URL", "REDIS_URL", "JWT_SECRET",
	} {
		t.Setenv(key, "")
		os.Unsetenv(key)
	}

	cfg := Load()
	require.NotNil(t, cfg)

	assert.Equal(t, "8080", cfg.AppPort)
	assert.Equal(t, "anthropic", cfg.AIProvider)
	assert.Equal(t, "gpt-4o", cfg.OpenAIModel)
	assert.Equal(t, "http://localhost:11434", cfg.OllamaBaseURL)
	assert.Equal(t, "llama3", cfg.OllamaModel)
	assert.Equal(t, "openai/gpt-oss-120b", cfg.GroqModel)
	assert.Equal(t, "993", cfg.IMAPPort)
	assert.Equal(t, "00000000-0000-0000-0000-000000000001", cfg.OwnerUserID)
	assert.Equal(t, "https://calendar.app.google/CQciFBayHqi6CstB7", cfg.BookingLink)
	assert.Equal(t, "Дмитрий", cfg.SenderName)
	assert.Equal(t, "dev-bot.su", cfg.SenderCompany)
	assert.Equal(t, 2, cfg.StaleDays)
	assert.Empty(t, cfg.DatabaseURL)
	assert.Empty(t, cfg.JWTSecret)
}

func TestLoad_CustomEnvVars(t *testing.T) {
	t.Setenv("APP_PORT", "9090")
	t.Setenv("DATABASE_URL", "postgres://localhost/test")
	t.Setenv("JWT_SECRET", "supersecret")
	t.Setenv("AI_PROVIDER", "openai")
	t.Setenv("STALE_DAYS", "7")
	t.Setenv("SENDER_NAME", "Иван")

	cfg := Load()
	require.NotNil(t, cfg)

	assert.Equal(t, "9090", cfg.AppPort)
	assert.Equal(t, "postgres://localhost/test", cfg.DatabaseURL)
	assert.Equal(t, "supersecret", cfg.JWTSecret)
	assert.Equal(t, "openai", cfg.AIProvider)
	assert.Equal(t, 7, cfg.StaleDays)
	assert.Equal(t, "Иван", cfg.SenderName)
}

func TestGetEnvInt_ValidInt(t *testing.T) {
	t.Setenv("TEST_INT", "42")
	result := getEnvInt("TEST_INT", 10)
	assert.Equal(t, 42, result)
}

func TestGetEnvInt_InvalidInt(t *testing.T) {
	t.Setenv("TEST_INT", "not-a-number")
	result := getEnvInt("TEST_INT", 10)
	assert.Equal(t, 10, result)
}

func TestGetEnvInt_MissingKey(t *testing.T) {
	os.Unsetenv("TEST_INT_MISSING")
	result := getEnvInt("TEST_INT_MISSING", 99)
	assert.Equal(t, 99, result)
}
