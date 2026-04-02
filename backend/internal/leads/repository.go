package leads

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Lead struct {
	ID             uuid.UUID `json:"id"`
	UserID         uuid.UUID `json:"user_id"`
	Channel        string    `json:"channel"`
	ContactName    string    `json:"contact_name"`
	Company        string    `json:"company"`
	FirstMessage   string    `json:"first_message"`
	Status         string    `json:"status"`
	TelegramChatID *int64    `json:"telegram_chat_id,omitempty"`
	EmailAddress   *string   `json:"email_address,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type Message struct {
	ID        uuid.UUID `json:"id"`
	LeadID    uuid.UUID `json:"lead_id"`
	Direction string    `json:"direction"`
	Body      string    `json:"body"`
	SentAt    time.Time `json:"sent_at"`
}

type Qualification struct {
	ID                uuid.UUID `json:"id"`
	LeadID            uuid.UUID `json:"lead_id"`
	IdentifiedNeed    string    `json:"identified_need"`
	EstimatedBudget   string    `json:"estimated_budget"`
	Deadline          string    `json:"deadline"`
	Score             int       `json:"score"`
	ScoreReason       string    `json:"score_reason"`
	RecommendedAction string    `json:"recommended_action"`
	ProviderUsed      string    `json:"provider_used"`
	GeneratedAt       time.Time `json:"generated_at"`
}

type Draft struct {
	ID        uuid.UUID `json:"id"`
	LeadID    uuid.UUID `json:"lead_id"`
	Body      string    `json:"body"`
	CreatedAt time.Time `json:"created_at"`
}

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) ListLeads(ctx context.Context, userID uuid.UUID) ([]Lead, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, user_id, channel, contact_name, company, first_message, status, telegram_chat_id, email_address, created_at, updated_at
		 FROM leads WHERE user_id = $1 ORDER BY created_at DESC`, userID)
	if err != nil {
		return nil, fmt.Errorf("list leads: %w", err)
	}
	defer rows.Close()

	var leads []Lead
	for rows.Next() {
		var l Lead
		if err := rows.Scan(&l.ID, &l.UserID, &l.Channel, &l.ContactName, &l.Company, &l.FirstMessage, &l.Status, &l.TelegramChatID, &l.EmailAddress, &l.CreatedAt, &l.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan lead: %w", err)
		}
		leads = append(leads, l)
	}
	return leads, rows.Err()
}

func (r *Repository) GetLead(ctx context.Context, id uuid.UUID) (*Lead, error) {
	var l Lead
	err := r.pool.QueryRow(ctx,
		`SELECT id, user_id, channel, contact_name, company, first_message, status, telegram_chat_id, email_address, created_at, updated_at
		 FROM leads WHERE id = $1`, id).
		Scan(&l.ID, &l.UserID, &l.Channel, &l.ContactName, &l.Company, &l.FirstMessage, &l.Status, &l.TelegramChatID, &l.EmailAddress, &l.CreatedAt, &l.UpdatedAt)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get lead: %w", err)
	}
	return &l, nil
}

func (r *Repository) CreateLead(ctx context.Context, lead *Lead) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO leads (id, user_id, channel, contact_name, company, first_message, status, telegram_chat_id, email_address, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`,
		lead.ID, lead.UserID, lead.Channel, lead.ContactName, lead.Company, lead.FirstMessage, lead.Status, lead.TelegramChatID, lead.EmailAddress, lead.CreatedAt, lead.UpdatedAt)
	if err != nil {
		return fmt.Errorf("create lead: %w", err)
	}
	return nil
}

func (r *Repository) UpdateLeadStatus(ctx context.Context, id uuid.UUID, status string) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE leads SET status = $1, updated_at = $2 WHERE id = $3`,
		status, time.Now().UTC(), id)
	if err != nil {
		return fmt.Errorf("update lead status: %w", err)
	}
	return nil
}

func (r *Repository) ListMessages(ctx context.Context, leadID uuid.UUID) ([]Message, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, lead_id, direction, body, sent_at
		 FROM messages WHERE lead_id = $1 ORDER BY sent_at`, leadID)
	if err != nil {
		return nil, fmt.Errorf("list messages: %w", err)
	}
	defer rows.Close()

	var msgs []Message
	for rows.Next() {
		var m Message
		if err := rows.Scan(&m.ID, &m.LeadID, &m.Direction, &m.Body, &m.SentAt); err != nil {
			return nil, fmt.Errorf("scan message: %w", err)
		}
		msgs = append(msgs, m)
	}
	return msgs, rows.Err()
}

func (r *Repository) CreateMessage(ctx context.Context, msg *Message) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO messages (id, lead_id, direction, body, sent_at)
		 VALUES ($1, $2, $3, $4, $5)`,
		msg.ID, msg.LeadID, msg.Direction, msg.Body, msg.SentAt)
	if err != nil {
		return fmt.Errorf("create message: %w", err)
	}
	return nil
}

func (r *Repository) GetQualification(ctx context.Context, leadID uuid.UUID) (*Qualification, error) {
	var q Qualification
	err := r.pool.QueryRow(ctx,
		`SELECT id, lead_id, identified_need, estimated_budget, deadline, score, score_reason, recommended_action, provider_used, generated_at
		 FROM qualifications WHERE lead_id = $1`, leadID).
		Scan(&q.ID, &q.LeadID, &q.IdentifiedNeed, &q.EstimatedBudget, &q.Deadline, &q.Score, &q.ScoreReason, &q.RecommendedAction, &q.ProviderUsed, &q.GeneratedAt)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get qualification: %w", err)
	}
	return &q, nil
}

func (r *Repository) UpsertQualification(ctx context.Context, q *Qualification) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO qualifications (id, lead_id, identified_need, estimated_budget, deadline, score, score_reason, recommended_action, provider_used, generated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		 ON CONFLICT (lead_id) DO UPDATE SET
		   identified_need = EXCLUDED.identified_need,
		   estimated_budget = EXCLUDED.estimated_budget,
		   deadline = EXCLUDED.deadline,
		   score = EXCLUDED.score,
		   score_reason = EXCLUDED.score_reason,
		   recommended_action = EXCLUDED.recommended_action,
		   provider_used = EXCLUDED.provider_used,
		   generated_at = EXCLUDED.generated_at`,
		q.ID, q.LeadID, q.IdentifiedNeed, q.EstimatedBudget, q.Deadline, q.Score, q.ScoreReason, q.RecommendedAction, q.ProviderUsed, q.GeneratedAt)
	if err != nil {
		return fmt.Errorf("upsert qualification: %w", err)
	}
	return nil
}

func (r *Repository) GetLatestDraft(ctx context.Context, leadID uuid.UUID) (*Draft, error) {
	var d Draft
	err := r.pool.QueryRow(ctx,
		`SELECT id, lead_id, body, created_at
		 FROM drafts WHERE lead_id = $1 ORDER BY created_at DESC LIMIT 1`, leadID).
		Scan(&d.ID, &d.LeadID, &d.Body, &d.CreatedAt)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get latest draft: %w", err)
	}
	return &d, nil
}

func (r *Repository) CreateDraft(ctx context.Context, d *Draft) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO drafts (id, lead_id, body, created_at)
		 VALUES ($1, $2, $3, $4)`,
		d.ID, d.LeadID, d.Body, d.CreatedAt)
	if err != nil {
		return fmt.Errorf("create draft: %w", err)
	}
	return nil
}

// GetLeadByTelegramChatID looks up a lead by telegram_chat_id for a given user.
func (r *Repository) GetLeadByTelegramChatID(ctx context.Context, userID uuid.UUID, chatID int64) (*Lead, error) {
	var l Lead
	err := r.pool.QueryRow(ctx,
		`SELECT id, user_id, channel, contact_name, company, first_message, status, telegram_chat_id, email_address, created_at, updated_at
		 FROM leads WHERE user_id = $1 AND telegram_chat_id = $2`, userID, chatID).
		Scan(&l.ID, &l.UserID, &l.Channel, &l.ContactName, &l.Company, &l.FirstMessage, &l.Status, &l.TelegramChatID, &l.EmailAddress, &l.CreatedAt, &l.UpdatedAt)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get lead by telegram chat id: %w", err)
	}
	return &l, nil
}

// GetLeadByEmailAddress looks up a lead by email address for a given user.
func (r *Repository) GetLeadByEmailAddress(ctx context.Context, userID uuid.UUID, email string) (*Lead, error) {
	var l Lead
	err := r.pool.QueryRow(ctx,
		`SELECT id, user_id, channel, contact_name, company, first_message, status, telegram_chat_id, email_address, created_at, updated_at
		 FROM leads WHERE user_id = $1 AND email_address = $2`, userID, email).
		Scan(&l.ID, &l.UserID, &l.Channel, &l.ContactName, &l.Company, &l.FirstMessage, &l.Status, &l.TelegramChatID, &l.EmailAddress, &l.CreatedAt, &l.UpdatedAt)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get lead by email: %w", err)
	}
	return &l, nil
}

// StaleLeadsWithoutReminder returns leads where the last message is older than
// staleDays, the lead is not closed, and there is no active (non-dismissed) reminder.
func (r *Repository) StaleLeadsWithoutReminder(ctx context.Context, staleDays int) ([]Lead, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT l.id, l.user_id, l.channel, l.contact_name, l.company, l.first_message, l.status, l.telegram_chat_id, l.email_address, l.created_at, l.updated_at
		 FROM leads l
		 WHERE l.status NOT IN ('closed')
		   AND NOT EXISTS (
		     SELECT 1 FROM reminders r WHERE r.lead_id = l.id AND r.dismissed = FALSE
		   )
		   AND (
		     SELECT MAX(m.sent_at) FROM messages m WHERE m.lead_id = l.id
		   ) < NOW() - make_interval(days => $1)
		 ORDER BY l.updated_at ASC`, staleDays)
	if err != nil {
		return nil, fmt.Errorf("stale leads: %w", err)
	}
	defer rows.Close()

	var leads []Lead
	for rows.Next() {
		var l Lead
		if err := rows.Scan(&l.ID, &l.UserID, &l.Channel, &l.ContactName, &l.Company, &l.FirstMessage, &l.Status, &l.TelegramChatID, &l.EmailAddress, &l.CreatedAt, &l.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan stale lead: %w", err)
		}
		leads = append(leads, l)
	}
	return leads, rows.Err()
}

// CreateReminder inserts a new reminder for a lead.
func (r *Repository) CreateReminder(ctx context.Context, leadID uuid.UUID, message string) error {
	id := uuid.New()
	_, err := r.pool.Exec(ctx,
		`INSERT INTO reminders (id, lead_id, message, created_at)
		 VALUES ($1, $2, $3, $4)`,
		id, leadID, message, time.Now().UTC())
	if err != nil {
		return fmt.Errorf("create reminder: %w", err)
	}
	return nil
}
