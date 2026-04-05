package chat

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/daniil/floq/internal/ai"
	"github.com/daniil/floq/internal/httputil"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Handler struct {
	pool     *pgxpool.Pool
	aiClient *ai.AIClient
}

func NewHandler(pool *pgxpool.Pool, aiClient *ai.AIClient) *Handler {
	return &Handler{pool: pool, aiClient: aiClient}
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

	stats, err := h.fetchStats(r.Context(), userID)
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

func (h *Handler) fetchStats(ctx context.Context, userID uuid.UUID) (*userStats, error) {
	s := &userStats{StatusCounts: make(map[string]int)}

	// Total leads
	err := h.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM leads WHERE user_id = $1`, userID).Scan(&s.TotalLeads)
	if err != nil {
		return nil, fmt.Errorf("total leads: %w", err)
	}

	// Leads this month
	err = h.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM leads WHERE user_id = $1 AND created_at >= date_trunc('month', CURRENT_DATE)`,
		userID).Scan(&s.MonthLeads)
	if err != nil {
		return nil, fmt.Errorf("month leads: %w", err)
	}

	// Leads by status
	rows, err := h.pool.Query(ctx,
		`SELECT status::text, COUNT(*) FROM leads WHERE user_id = $1 GROUP BY status`, userID)
	if err != nil {
		return nil, fmt.Errorf("status counts: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var status string
		var count int
		if err := rows.Scan(&status, &count); err != nil {
			return nil, fmt.Errorf("scan status: %w", err)
		}
		s.StatusCounts[status] = count
	}

	// Prospect count
	err = h.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM prospects WHERE user_id = $1`, userID).Scan(&s.ProspectCount)
	if err != nil {
		return nil, fmt.Errorf("prospects: %w", err)
	}

	// Sequence count
	err = h.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM sequences WHERE user_id = $1`, userID).Scan(&s.SequenceCount)
	if err != nil {
		return nil, fmt.Errorf("sequences: %w", err)
	}

	// Queued outbound messages (draft = awaiting approval)
	err = h.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM outbound_messages WHERE status = 'draft' AND sequence_id IN (SELECT id FROM sequences WHERE user_id = $1)`,
		userID).Scan(&s.QueuedMsgs)
	if err != nil {
		s.QueuedMsgs = 0
	}

	// Recent leads (last 10)
	recentRows, err := h.pool.Query(ctx,
		`SELECT contact_name, COALESCE(company, ''), status::text, created_at FROM leads WHERE user_id = $1 ORDER BY created_at DESC LIMIT 10`,
		userID)
	if err != nil {
		return nil, fmt.Errorf("recent leads: %w", err)
	}
	defer recentRows.Close()
	for recentRows.Next() {
		var l recentLead
		if err := recentRows.Scan(&l.Name, &l.Company, &l.Status, &l.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan recent lead: %w", err)
		}
		s.RecentLeads = append(s.RecentLeads, l)
	}

	return s, nil
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
