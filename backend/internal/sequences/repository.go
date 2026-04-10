package sequences

import (
	"context"
	"fmt"
	"time"

	"github.com/daniil/floq/internal/sequences/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Compile-time check that Repository implements domain.Repository.
var _ domain.Repository = (*Repository)(nil)

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) ListSequences(ctx context.Context, userID uuid.UUID) ([]domain.Sequence, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, user_id, name, is_active, created_at
		 FROM sequences WHERE user_id = $1 ORDER BY created_at DESC`, userID)
	if err != nil {
		return nil, fmt.Errorf("list sequences: %w", err)
	}
	defer rows.Close()

	var seqs []domain.Sequence
	for rows.Next() {
		var s domain.Sequence
		if err := rows.Scan(&s.ID, &s.UserID, &s.Name, &s.IsActive, &s.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan sequence: %w", err)
		}
		seqs = append(seqs, s)
	}
	return seqs, rows.Err()
}

func (r *Repository) GetSequence(ctx context.Context, id uuid.UUID) (*domain.Sequence, error) {
	var s domain.Sequence
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

func (r *Repository) CreateSequence(ctx context.Context, s *domain.Sequence) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO sequences (id, user_id, name, is_active, created_at)
		 VALUES ($1, $2, $3, $4, $5)`,
		s.ID, s.UserID, s.Name, s.IsActive, s.CreatedAt)
	if err != nil {
		return fmt.Errorf("create sequence: %w", err)
	}
	return nil
}

func (r *Repository) UpdateSequence(ctx context.Context, s *domain.Sequence) error {
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

func (r *Repository) ListSteps(ctx context.Context, sequenceID uuid.UUID) ([]domain.SequenceStep, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, sequence_id, step_order, delay_days, prompt_hint, channel, created_at
		 FROM sequence_steps WHERE sequence_id = $1 ORDER BY step_order`, sequenceID)
	if err != nil {
		return nil, fmt.Errorf("list steps: %w", err)
	}
	defer rows.Close()

	var steps []domain.SequenceStep
	for rows.Next() {
		var st domain.SequenceStep
		if err := rows.Scan(&st.ID, &st.SequenceID, &st.StepOrder, &st.DelayDays, &st.PromptHint, &st.Channel, &st.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan step: %w", err)
		}
		steps = append(steps, st)
	}
	return steps, rows.Err()
}

func (r *Repository) CreateStep(ctx context.Context, step *domain.SequenceStep) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO sequence_steps (id, sequence_id, step_order, delay_days, prompt_hint, channel, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		step.ID, step.SequenceID, step.StepOrder, step.DelayDays, step.PromptHint, step.Channel, step.CreatedAt)
	if err != nil {
		return fmt.Errorf("create step: %w", err)
	}
	return nil
}

func (r *Repository) DeleteStep(ctx context.Context, stepID uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM sequence_steps WHERE id = $1`, stepID)
	if err != nil {
		return fmt.Errorf("delete step: %w", err)
	}
	return nil
}

func (r *Repository) CreateOutboundMessage(ctx context.Context, msg *domain.OutboundMessage) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO outbound_messages (id, prospect_id, sequence_id, step_order, channel, body, status, scheduled_at, sent_at, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
		msg.ID, msg.ProspectID, msg.SequenceID, msg.StepOrder, msg.Channel, msg.Body, msg.Status, msg.ScheduledAt, msg.SentAt, msg.CreatedAt)
	if err != nil {
		return fmt.Errorf("create outbound message: %w", err)
	}
	return nil
}

func (r *Repository) ListOutboundQueue(ctx context.Context, userID uuid.UUID) ([]domain.OutboundMessage, error) {
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

	var msgs []domain.OutboundMessage
	for rows.Next() {
		var m domain.OutboundMessage
		if err := rows.Scan(&m.ID, &m.ProspectID, &m.SequenceID, &m.StepOrder, &m.Channel, &m.Body, &m.Status, &m.ScheduledAt, &m.SentAt, &m.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan outbound message: %w", err)
		}
		msgs = append(msgs, m)
	}
	return msgs, rows.Err()
}

func (r *Repository) ListSentMessages(ctx context.Context, userID uuid.UUID) ([]domain.OutboundMessage, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT om.id, om.prospect_id, om.sequence_id, om.step_order, om.channel, om.body, om.status, om.scheduled_at, om.sent_at, om.created_at
		 FROM outbound_messages om
		 JOIN prospects p ON p.id = om.prospect_id
		 WHERE p.user_id = $1 AND om.status IN ('sent', 'approved', 'rejected')
		 ORDER BY om.created_at DESC
		 LIMIT 100`, userID)
	if err != nil {
		return nil, fmt.Errorf("list sent messages: %w", err)
	}
	defer rows.Close()

	var msgs []domain.OutboundMessage
	for rows.Next() {
		var m domain.OutboundMessage
		if err := rows.Scan(&m.ID, &m.ProspectID, &m.SequenceID, &m.StepOrder, &m.Channel, &m.Body, &m.Status, &m.ScheduledAt, &m.SentAt, &m.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan sent message: %w", err)
		}
		msgs = append(msgs, m)
	}
	return msgs, rows.Err()
}

func (r *Repository) UpdateOutboundStatus(ctx context.Context, id uuid.UUID, status domain.OutboundStatus) error {
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

func (r *Repository) GetPendingSends(ctx context.Context) ([]domain.OutboundMessage, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, prospect_id, sequence_id, step_order, channel, body, status, scheduled_at, sent_at, created_at
		 FROM outbound_messages
		 WHERE status = 'approved' AND scheduled_at <= NOW()`)
	if err != nil {
		return nil, fmt.Errorf("get pending sends: %w", err)
	}
	defer rows.Close()

	var msgs []domain.OutboundMessage
	for rows.Next() {
		var m domain.OutboundMessage
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

func (r *Repository) MarkBounced(ctx context.Context, id uuid.UUID) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE outbound_messages SET status = 'bounced' WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("mark bounced: %w", err)
	}
	return nil
}

// MarkRepliedByProspect sets replied_at on all sent outbound messages for a prospect.
// This is an extra method on the concrete type, not part of the domain interface.
func (r *Repository) MarkRepliedByProspect(ctx context.Context, prospectID uuid.UUID) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE outbound_messages SET replied_at = COALESCE(replied_at, NOW()) WHERE prospect_id = $1 AND status = 'sent'`,
		prospectID)
	if err != nil {
		return fmt.Errorf("mark replied by prospect: %w", err)
	}
	return nil
}

func (r *Repository) GetOutboundMessage(ctx context.Context, id uuid.UUID) (*domain.OutboundMessage, error) {
	var m domain.OutboundMessage
	err := r.pool.QueryRow(ctx,
		`SELECT id, prospect_id, sequence_id, step_order, channel, body, status, scheduled_at, sent_at, created_at
		 FROM outbound_messages WHERE id = $1`, id).
		Scan(&m.ID, &m.ProspectID, &m.SequenceID, &m.StepOrder, &m.Channel, &m.Body, &m.Status, &m.ScheduledAt, &m.SentAt, &m.CreatedAt)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get outbound message: %w", err)
	}
	return &m, nil
}

func (r *Repository) SavePromptFeedback(ctx context.Context, userID uuid.UUID, original, edited, prospectContext, channel string) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO prompt_feedback (user_id, original_body, edited_body, prospect_context, channel)
		 VALUES ($1, $2, $3, $4, $5)`,
		userID, original, edited, prospectContext, channel)
	if err != nil {
		return fmt.Errorf("save prompt feedback: %w", err)
	}
	return nil
}

func (r *Repository) GetRecentFeedback(ctx context.Context, userID uuid.UUID, limit int) ([]domain.PromptFeedback, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, user_id, original_body, edited_body, prospect_context, channel, created_at
		 FROM prompt_feedback WHERE user_id = $1 ORDER BY created_at DESC LIMIT $2`, userID, limit)
	if err != nil {
		return nil, fmt.Errorf("get recent feedback: %w", err)
	}
	defer rows.Close()

	var feedback []domain.PromptFeedback
	for rows.Next() {
		var f domain.PromptFeedback
		if err := rows.Scan(&f.ID, &f.UserID, &f.OriginalBody, &f.EditedBody, &f.ProspectContext, &f.Channel, &f.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan prompt feedback: %w", err)
		}
		feedback = append(feedback, f)
	}
	return feedback, rows.Err()
}

func (r *Repository) GetConversationHistory(ctx context.Context, prospectID uuid.UUID) ([]domain.ConversationEntry, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT body, status, sent_at
		 FROM outbound_messages
		 WHERE prospect_id = $1 AND status IN ('sent', 'approved')
		 ORDER BY created_at ASC`, prospectID)
	if err != nil {
		return nil, fmt.Errorf("get conversation history: %w", err)
	}
	defer rows.Close()

	var entries []domain.ConversationEntry
	for rows.Next() {
		var e domain.ConversationEntry
		var sentAt *time.Time
		if err := rows.Scan(&e.Body, &e.Status, &sentAt); err != nil {
			return nil, fmt.Errorf("scan conversation entry: %w", err)
		}
		if sentAt != nil {
			e.SentAt = *sentAt
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

func (r *Repository) GetStats(ctx context.Context, userID uuid.UUID) (*domain.Stats, error) {
	var stats domain.Stats
	err := r.pool.QueryRow(ctx,
		`SELECT
			COALESCE(SUM(CASE WHEN om.status = 'draft' THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN om.status = 'approved' THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN om.status = 'sent' THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN om.opened_at IS NOT NULL THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN om.replied_at IS NOT NULL THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN om.status = 'bounced' THEN 1 ELSE 0 END), 0)
		 FROM outbound_messages om
		 JOIN prospects p ON p.id = om.prospect_id
		 WHERE p.user_id = $1`, userID).
		Scan(&stats.Draft, &stats.Approved, &stats.Sent, &stats.Opened, &stats.Replied, &stats.Bounced)
	if err != nil {
		return nil, fmt.Errorf("get stats: %w", err)
	}
	return &stats, nil
}
