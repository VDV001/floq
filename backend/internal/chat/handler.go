package chat

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/daniil/floq/internal/ai"
	"github.com/daniil/floq/internal/httputil"
	"github.com/go-chi/chi/v5"
)

type Handler struct {
	stats    StatsReader
	aiClient AIClient
}

func NewHandler(stats StatsReader, aiClient AIClient) *Handler {
	return &Handler{stats: stats, aiClient: aiClient}
}

func RegisterRoutes(r chi.Router, h *Handler) {
	r.Post("/api/chat", h.Chat)
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatRequest struct {
	Message string        `json:"message"`
	History []chatMessage `json:"history"`
	Context string        `json:"context"`
}

type chatResponse struct {
	Reply string `json:"reply"`
}

// userStats holds aggregated CRM data for the system prompt.
type userStats struct {
	TotalLeads    int
	MonthLeads    int
	StatusCounts  map[string]int
	ProspectCount int
	SequenceCount int
	QueuedMsgs    int
	RecentLeads   []recentLead
}

type recentLead struct {
	Name      string
	Company   string
	Status    string
	CreatedAt time.Time
}

func (h *Handler) Chat(w http.ResponseWriter, r *http.Request) {
	userID, ok := httputil.UserIDFromContext(r.Context())
	if !ok {
		httputil.WriteError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req chatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if strings.TrimSpace(req.Message) == "" {
		httputil.WriteError(w, http.StatusBadRequest, "message is required")
		return
	}

	stats, err := h.stats.FetchStats(r.Context(), userID)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "failed to fetch stats")
		return
	}

	systemPrompt := buildSystemPrompt(stats, req.Context)

	// Build messages for the AI provider.
	messages := []ai.Message{{Role: "system", Content: systemPrompt}}
	for _, m := range req.History {
		if m.Role == "user" || m.Role == "assistant" {
			messages = append(messages, ai.Message{Role: m.Role, Content: m.Content})
		}
	}
	messages = append(messages, ai.Message{Role: "user", Content: req.Message})

	reply, err := h.aiClient.Complete(r.Context(), ai.CompletionRequest{
		Messages:  messages,
		MaxTokens: 4096,
	})
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "AI provider error")
		return
	}

	httputil.WriteJSON(w, http.StatusOK, chatResponse{Reply: reply})
}

func buildSystemPrompt(s *userStats, extraContext string) string {
	var b strings.Builder

	b.WriteString("Ты — AI-ассистент по продажам в CRM Floq. Отвечай на русском языке.\n")
	b.WriteString("Будь конкретным и полезным. Используй markdown для форматирования (списки, таблицы, жирный текст).\n")
	b.WriteString("Если вопрос требует детального анализа — дай полный ответ. Если простой — ответь кратко.\n\n")
	b.WriteString("Данные пользователя:\n")
	b.WriteString(fmt.Sprintf("- Лидов всего: %d\n", s.TotalLeads))
	b.WriteString(fmt.Sprintf("- Лидов в этом месяце: %d\n", s.MonthLeads))

	b.WriteString("- По статусам: ")
	statuses := []string{"new", "qualified", "in_conversation", "followup", "closed"}
	parts := make([]string, 0, len(statuses))
	for _, st := range statuses {
		parts = append(parts, fmt.Sprintf("%s: %d", st, s.StatusCounts[st]))
	}
	b.WriteString(strings.Join(parts, ", "))
	b.WriteString("\n")

	b.WriteString(fmt.Sprintf("- Проспектов: %d\n", s.ProspectCount))
	b.WriteString(fmt.Sprintf("- Секвенций: %d\n", s.SequenceCount))
	b.WriteString(fmt.Sprintf("- Сообщений в очереди: %d\n", s.QueuedMsgs))

	if len(s.RecentLeads) > 0 {
		b.WriteString("\nПоследние лиды:\n")
		for _, l := range s.RecentLeads {
			company := l.Company
			if company == "" {
				company = "—"
			}
			b.WriteString(fmt.Sprintf("- %s (%s) — %s — %s\n",
				l.Name, company, l.Status, l.CreatedAt.Format("02.01.2006")))
		}
	}

	if extraContext != "" {
		b.WriteString("\nДополнительный контекст от пользователя:\n")
		b.WriteString(extraContext)
		b.WriteString("\n")
	}

	b.WriteString(`
Ты можешь помочь с:
- Анализом воронки и конверсий
- Советами по работе с лидами
- Рекомендациями по фоллоуапам
- Ответами на вопросы о функциях Floq

Не выдумывай данные. Если не знаешь — скажи прямо.
`)

	return b.String()
}
