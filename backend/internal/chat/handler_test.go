package chat

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/daniil/floq/internal/ai"
	"github.com/daniil/floq/internal/httputil"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Mock AI Provider ---

type mockProvider struct {
	response string
	err      error
}

func (m *mockProvider) Complete(_ context.Context, _ ai.CompletionRequest) (string, error) {
	return m.response, m.err
}

func (m *mockProvider) Name() string {
	return "mock"
}

// --- buildSystemPrompt tests ---

func TestBuildSystemPrompt_IncludesStats(t *testing.T) {
	stats := &userStats{
		TotalLeads:    42,
		MonthLeads:    10,
		StatusCounts:  map[string]int{"new": 5, "qualified": 3, "closed": 2},
		ProspectCount: 15,
		SequenceCount: 4,
		QueuedMsgs:    7,
	}

	prompt := buildSystemPrompt(stats, "")

	assert.Contains(t, prompt, "42")
	assert.Contains(t, prompt, "10")
	assert.Contains(t, prompt, "new: 5")
	assert.Contains(t, prompt, "qualified: 3")
	assert.Contains(t, prompt, "closed: 2")
	assert.Contains(t, prompt, "15")
	assert.Contains(t, prompt, "Секвенций: 4")
	assert.Contains(t, prompt, "Сообщений в очереди: 7")
}

func TestBuildSystemPrompt_IncludesRecentLeads(t *testing.T) {
	stats := &userStats{
		StatusCounts: map[string]int{},
		RecentLeads: []recentLead{
			{
				Name:      "Алексей",
				Company:   "ООО Тест",
				Status:    "new",
				CreatedAt: time.Date(2025, 3, 15, 0, 0, 0, 0, time.UTC),
			},
			{
				Name:      "Мария",
				Company:   "",
				Status:    "qualified",
				CreatedAt: time.Date(2025, 3, 14, 0, 0, 0, 0, time.UTC),
			},
		},
	}

	prompt := buildSystemPrompt(stats, "")

	assert.Contains(t, prompt, "Последние лиды")
	assert.Contains(t, prompt, "Алексей")
	assert.Contains(t, prompt, "ООО Тест")
	assert.Contains(t, prompt, "Мария")
	// Empty company should become a dash.
	assert.Contains(t, prompt, "—")
	assert.Contains(t, prompt, "15.03.2025")
}

func TestBuildSystemPrompt_IncludesExtraContext(t *testing.T) {
	stats := &userStats{
		StatusCounts: map[string]int{},
	}

	prompt := buildSystemPrompt(stats, "Фокус на IT-компании")

	assert.Contains(t, prompt, "Дополнительный контекст от пользователя")
	assert.Contains(t, prompt, "Фокус на IT-компании")
}

func TestBuildSystemPrompt_NoExtraContext(t *testing.T) {
	stats := &userStats{
		StatusCounts: map[string]int{},
	}

	prompt := buildSystemPrompt(stats, "")

	assert.NotContains(t, prompt, "Дополнительный контекст от пользователя")
}

func TestBuildSystemPrompt_NoRecentLeads(t *testing.T) {
	stats := &userStats{
		StatusCounts: map[string]int{},
		RecentLeads:  nil,
	}

	prompt := buildSystemPrompt(stats, "")

	assert.NotContains(t, prompt, "Последние лиды")
}

// --- Chat handler tests (input validation, no DB) ---

func TestChat_MissingMessage(t *testing.T) {
	provider := &mockProvider{response: "hi"}
	client := ai.NewAIClient(provider, "", "", "")
	h := NewHandler(nil, client)

	body, _ := json.Marshal(chatRequest{Message: "", History: nil})
	req := httptest.NewRequest(http.MethodPost, "/api/chat", bytes.NewReader(body))
	// Set user ID in context so we pass the auth check.
	ctx := httputil.WithUserID(req.Context(), uuid.New())
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	h.Chat(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "message is required")
}

func TestChat_WhitespaceOnlyMessage(t *testing.T) {
	provider := &mockProvider{response: "hi"}
	client := ai.NewAIClient(provider, "", "", "")
	h := NewHandler(nil, client)

	body, _ := json.Marshal(chatRequest{Message: "   \t\n  ", History: nil})
	req := httptest.NewRequest(http.MethodPost, "/api/chat", bytes.NewReader(body))
	ctx := httputil.WithUserID(req.Context(), uuid.New())
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	h.Chat(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "message is required")
}

func TestChat_Unauthorized(t *testing.T) {
	provider := &mockProvider{response: "hi"}
	client := ai.NewAIClient(provider, "", "", "")
	h := NewHandler(nil, client)

	body, _ := json.Marshal(chatRequest{Message: "hello"})
	req := httptest.NewRequest(http.MethodPost, "/api/chat", bytes.NewReader(body))
	// No user ID in context.
	rec := httptest.NewRecorder()

	h.Chat(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.Contains(t, rec.Body.String(), "unauthorized")
}

func TestChat_InvalidJSON(t *testing.T) {
	provider := &mockProvider{response: "hi"}
	client := ai.NewAIClient(provider, "", "", "")
	h := NewHandler(nil, client)

	req := httptest.NewRequest(http.MethodPost, "/api/chat", bytes.NewReader([]byte("not json")))
	ctx := httputil.WithUserID(req.Context(), uuid.New())
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	h.Chat(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "invalid request body")
}


// --- Mock provider test ---

func TestMockProvider(t *testing.T) {
	provider := &mockProvider{response: "test response", err: nil}
	client := ai.NewAIClient(provider, "https://book.me", "Дмитрий", "dev-bot.su")

	resp, err := client.Complete(context.Background(), ai.CompletionRequest{
		Messages:  []ai.Message{{Role: "user", Content: "hello"}},
		MaxTokens: 100,
	})
	require.NoError(t, err)
	assert.Equal(t, "test response", resp)
	assert.Equal(t, "mock", client.ProviderName())
}
