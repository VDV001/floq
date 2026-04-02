package sequences

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Sequence struct {
	ID        uuid.UUID `json:"id"`
	UserID    uuid.UUID `json:"user_id"`
	Name      string    `json:"name"`
	IsActive  bool      `json:"is_active"`
	CreatedAt time.Time `json:"created_at"`
}

type SequenceStep struct {
	ID         uuid.UUID `json:"id"`
	SequenceID uuid.UUID `json:"sequence_id"`
	StepOrder  int       `json:"step_order"`
	DelayDays  int       `json:"delay_days"`
	PromptHint string    `json:"prompt_hint"`
	Channel    string    `json:"channel"`
	CreatedAt  time.Time `json:"created_at"`
}

type OutboundMessage struct {
	ID          uuid.UUID  `json:"id"`
	ProspectID  uuid.UUID  `json:"prospect_id"`
	SequenceID  uuid.UUID  `json:"sequence_id"`
	StepOrder   int        `json:"step_order"`
	Channel     string     `json:"channel"`
	Body        string     `json:"body"`
	Status      string     `json:"status"`
	ScheduledAt time.Time  `json:"scheduled_at"`
	SentAt      *time.Time `json:"sent_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
}

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) ListSequences(ctx context.Context, userID uuid.UUID) ([]Sequence, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, user_id, name, is_active, created_at
		 FROM sequences WHERE user_id = $1 ORDER BY created_at DESC`, userID)
	if err != nil {
		return nil, fmt.Errorf("list sequences: %w", err)
	}
	defer rows.Close()

	var seqs []Sequence
	for rows.Next() {
		var s Sequence
		if err := rows.Scan(&s.ID, &s.UserID, &s.Name, &s.IsActive, &s.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan sequence: %w", err)
		}
		seqs = append(seqs, s)
	}
	return seqs, rows.Err()
}

func (r *Repository) GetSequence(ctx context.Context, id uuid.UUID) (*Sequence, error) {
	var s Sequence
	err := r.pool.QueryRow(ctx,
		`SELECT id, user_id, name, is_active, created_at
		 FROM sequences WHERE id = $1`, id).
		Scan(&s.ID, &s.UserID, &s.Name, &s.IsActive, &s.CreatedAt)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get sequence: %w", err)
	}
	return &s, nil
}

func (r *Repository) CreateSequence(ctx context.Context, s *Sequence) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO sequences (id, user_id, name, is_active, created_at)
		 VALUES ($1, $2, $3, $4, $5)`,
		s.ID, s.UserID, s.Name, s.IsActive, s.CreatedAt)
	if err != nil {
		return fmt.Errorf("create sequence: %w", err)
	}
	return nil
}

func (r *Repository) UpdateSequence(ctx context.Context, s *Sequence) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE sequences SET name = $1 WHERE id = $2`,
		s.Name, s.ID)
	if err != nil {
		return fmt.Errorf("update sequence: %w", err)
	}
	return nil
}

func (r *Repository) DeleteSequence(ctx context.Context, id uuid.UUID) error {
	_, err := r.pool.Exec(ctx,
		`DELETE FROM sequences WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete sequence: %w", err)
	}
	return nil
}

func (r *Repository) ToggleActive(ctx context.Context, id uuid.UUID, active bool) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE sequences SET is_active = $1 WHERE id = $2`,
		active, id)
	if err != nil {
		return fmt.Errorf("toggle sequence active: %w", err)
	}
	return nil
}

func (r *Repository) ListSteps(ctx context.Context, sequenceID uuid.UUID) ([]SequenceStep, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, sequence_id, step_order, delay_days, prompt_hint, channel, created_at
		 FROM sequence_steps WHERE sequence_id = $1 ORDER BY step_order`, sequenceID)
	if err != nil {
		return nil, fmt.Errorf("list steps: %w", err)
	}
	defer rows.Close()

	var steps []SequenceStep
	for rows.Next() {
		var st SequenceStep
		if err := rows.Scan(&st.ID, &st.SequenceID, &st.StepOrder, &st.DelayDays, &st.PromptHint, &st.Channel, &st.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan step: %w", err)
		}
		steps = append(steps, st)
	}
	return steps, rows.Err()
}

func (r *Repository) CreateStep(ctx context.Context, step *SequenceStep) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO sequence_steps (id, sequence_id, step_order, delay_days, prompt_hint, channel, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		step.ID, step.SequenceID, step.StepOrder, step.DelayDays, step.PromptHint, step.Channel, step.CreatedAt)
	if err != nil {
		return fmt.Errorf("create step: %w", err)
	}
	return nil
}

func (r *Repository) CreateOutboundMessage(ctx context.Context, msg *OutboundMessage) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO outbound_messages (id, prospect_id, sequence_id, step_order, channel, body, status, scheduled_at, sent_at, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
		msg.ID, msg.ProspectID, msg.SequenceID, msg.StepOrder, msg.Channel, msg.Body, msg.Status, msg.ScheduledAt, msg.SentAt, msg.CreatedAt)
	if err != nil {
		return fmt.Errorf("create outbound message: %w", err)
	}
	return nil
}

func (r *Repository) ListOutboundQueue(ctx context.Context, userID uuid.UUID) ([]OutboundMessage, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT om.id, om.prospect_id, om.sequence_id, om.step_order, om.channel, om.body, om.status, om.scheduled_at, om.sent_at, om.created_at
		 FROM outbound_messages om
		 JOIN prospects p ON p.id = om.prospect_id
		 WHERE p.user_id = $1 AND om.status = 'draft'
		 ORDER BY om.scheduled_at`, userID)
	if err != nil {
		return nil, fmt.Errorf("list outbound queue: %w", err)
	}
	defer rows.Close()

	var msgs []OutboundMessage
	for rows.Next() {
		var m OutboundMessage
		if err := rows.Scan(&m.ID, &m.ProspectID, &m.SequenceID, &m.StepOrder, &m.Channel, &m.Body, &m.Status, &m.ScheduledAt, &m.SentAt, &m.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan outbound message: %w", err)
		}
		msgs = append(msgs, m)
	}
	return msgs, rows.Err()
}

func (r *Repository) UpdateOutboundStatus(ctx context.Context, id uuid.UUID, status string) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE outbound_messages SET status = $1 WHERE id = $2`,
		status, id)
	if err != nil {
		return fmt.Errorf("update outbound status: %w", err)
	}
	return nil
}

func (r *Repository) UpdateOutboundBody(ctx context.Context, id uuid.UUID, body string) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE outbound_messages SET body = $1 WHERE id = $2`,
		body, id)
	if err != nil {
		return fmt.Errorf("update outbound body: %w", err)
	}
	return nil
}

func (r *Repository) GetPendingSends(ctx context.Context) ([]OutboundMessage, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, prospect_id, sequence_id, step_order, channel, body, status, scheduled_at, sent_at, created_at
		 FROM outbound_messages
		 WHERE status = 'approved' AND scheduled_at <= NOW()`)
	if err != nil {
		return nil, fmt.Errorf("get pending sends: %w", err)
	}
	defer rows.Close()

	var msgs []OutboundMessage
	for rows.Next() {
		var m OutboundMessage
		if err := rows.Scan(&m.ID, &m.ProspectID, &m.SequenceID, &m.StepOrder, &m.Channel, &m.Body, &m.Status, &m.ScheduledAt, &m.SentAt, &m.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan pending send: %w", err)
		}
		msgs = append(msgs, m)
	}
	return msgs, rows.Err()
}

func (r *Repository) MarkSent(ctx context.Context, id uuid.UUID) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE outbound_messages SET status = 'sent', sent_at = NOW() WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("mark sent: %w", err)
	}
	return nil
}

func (r *Repository) GetStats(ctx context.Context, userID uuid.UUID) (*Stats, error) {
	var stats Stats
	err := r.pool.QueryRow(ctx,
		`SELECT
			COALESCE(SUM(CASE WHEN om.status = 'draft' THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN om.status = 'approved' THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN om.status = 'sent' THEN 1 ELSE 0 END), 0)
		 FROM outbound_messages om
		 JOIN prospects p ON p.id = om.prospect_id
		 WHERE p.user_id = $1`, userID).
		Scan(&stats.Draft, &stats.Approved, &stats.Sent)
	if err != nil {
		return nil, fmt.Errorf("get stats: %w", err)
	}
	return &stats, nil
}
