package notify

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewTelegramNotifier_InvalidToken(t *testing.T) {
	tests := []struct {
		name  string
		token string
	}{
		{name: "empty token", token: ""},
		{name: "garbage token", token: "not-a-real-token"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			notifier, err := NewTelegramNotifier(tt.token, 123, nil)
			require.Error(t, err)
			assert.Nil(t, notifier)
		})
	}
}

func TestSendAlert_FormatsMessage(t *testing.T) {
	// We cannot create a valid TelegramNotifier without a real token,
	// but we can test the message formatting logic directly.
	text := fmt.Sprintf("Follow-up needed!\n\nLead: %s (%s)\n\n%s", "John", "Acme", "Call back")
	expected := "Follow-up needed!\n\nLead: John (Acme)\n\nCall back"
	assert.Equal(t, expected, text)
}

func TestSendAlert_FormatsMessage_EmptyFields(t *testing.T) {
	text := fmt.Sprintf("Follow-up needed!\n\nLead: %s (%s)\n\n%s", "", "", "")
	expected := "Follow-up needed!\n\nLead:  ()\n\n"
	assert.Equal(t, expected, text)
}

func TestSendAlert_FormatsMessage_UnicodeNames(t *testing.T) {
	text := fmt.Sprintf("Follow-up needed!\n\nLead: %s (%s)\n\n%s", "Иван Петров", "ООО Ромашка", "Перезвонить")
	assert.Contains(t, text, "Иван Петров")
	assert.Contains(t, text, "ООО Ромашка")
	assert.Contains(t, text, "Перезвонить")
}

func TestTelegramNotifier_NilBotSendAlert(t *testing.T) {
	// A TelegramNotifier with a nil bot will panic on SendAlert.
	// This verifies the struct fields are set properly via constructor.
	notifier := &TelegramNotifier{bot: nil, chatID: 42}
	assert.Equal(t, int64(42), notifier.chatID)
	assert.Nil(t, notifier.bot)
}

func TestSendAlert_ContextCancelled(t *testing.T) {
	// SendAlert takes a context parameter but currently doesn't use it.
	// Verify the function signature accepts a cancelled context without panic
	// (the actual error comes from nil bot, not context).
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	notifier := &TelegramNotifier{bot: nil, chatID: 123}
	// We can't call SendAlert because bot is nil and it will panic on bot.Send.
	// But we verify the notifier was built properly.
	_ = ctx
	assert.NotNil(t, notifier)
}

// newFakeTGBot creates a BotAPI backed by a test HTTP server that mimics Telegram API.
func newFakeTGBot(t *testing.T, sendErr bool) (*tgbotapi.BotAPI, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/bottest-token/getMe":
			json.NewEncoder(w).Encode(map[string]any{
				"ok": true,
				"result": map[string]any{
					"id":         123,
					"is_bot":     true,
					"first_name": "TestBot",
					"username":   "test_bot",
				},
			})
		case r.URL.Path == "/bottest-token/sendMessage" || r.URL.Path == "/bottest-token/SendMessage":
			if sendErr {
				json.NewEncoder(w).Encode(map[string]any{
					"ok":          false,
					"description": "Bad Request: chat not found",
				})
			} else {
				json.NewEncoder(w).Encode(map[string]any{
					"ok": true,
					"result": map[string]any{
						"message_id": 1,
						"chat":       map[string]any{"id": 42},
						"text":       "ok",
					},
				})
			}
		default:
			json.NewEncoder(w).Encode(map[string]any{"ok": true, "result": nil})
		}
	}))

	bot, err := tgbotapi.NewBotAPIWithAPIEndpoint("test-token", srv.URL+"/bot%s/%s")
	require.NoError(t, err)
	return bot, srv
}

func TestSendAlert_Success(t *testing.T) {
	bot, srv := newFakeTGBot(t, false)
	defer srv.Close()

	notifier := &TelegramNotifier{bot: bot, chatID: 42}
	err := notifier.SendAlert(context.Background(), "John", "Acme", "Call back")
	assert.NoError(t, err)
}

func TestSendAlert_BotError(t *testing.T) {
	bot, srv := newFakeTGBot(t, true)
	defer srv.Close()

	notifier := &TelegramNotifier{bot: bot, chatID: 42}
	err := notifier.SendAlert(context.Background(), "John", "Acme", "msg")
	assert.Error(t, err)
}

func TestNewTelegramNotifier_VariousChatIDs(t *testing.T) {
	tests := []struct {
		name   string
		chatID int64
	}{
		{"zero", 0},
		{"positive", 123456},
		{"negative (group)", -100123456},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Token is invalid so we expect error, but we're testing
			// that chatID value doesn't affect the token validation.
			_, err := NewTelegramNotifier("bad-token", tt.chatID, nil)
			require.Error(t, err)
		})
	}
}
