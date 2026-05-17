package inbox

import (
	"context"
	"errors"
	"fmt"

	"github.com/daniil/floq/internal/db"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Compile-time check that *PendingReplyRepo satisfies the port.
var _ PendingReplyRepository = (*PendingReplyRepo)(nil)

// PendingReplyRepo is the pgx-backed implementation of
// PendingReplyRepository. All queries are scoped by user_id so the
// repository itself cannot leak cross-tenant rows even if the caller
// forgets to check.
type PendingReplyRepo struct {
	pool *pgxpool.Pool
}

// NewPendingReplyRepository wires the SQL-backed implementation.
func NewPendingReplyRepository(pool *pgxpool.Pool) *PendingReplyRepo {
	return &PendingReplyRepo{pool: pool}
}

// q returns the Querier bound to the current context (a pgx.Tx when
// the caller wrapped the call in db.TxManager.WithTx, otherwise the
// pool) — mirrors the pattern used by leads.Repository and
// leads.IdentityRepository.
func (r *PendingReplyRepo) q(ctx context.Context) db.Querier {
	return db.ConnFromCtx(ctx, r.pool)
}

const pendingReplyColumns = `id, user_id, lead_id, channel, kind, body, status, created_at, decided_at, sent_at`

func (r *PendingReplyRepo) Save(ctx context.Context, pr *PendingReply) error {
	_, err := r.q(ctx).Exec(ctx,
		`INSERT INTO pending_replies (`+pendingReplyColumns+`)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
		pr.ID, pr.UserID, pr.LeadID, string(pr.Channel), string(pr.Kind), pr.Body,
		string(pr.Status), pr.CreatedAt, pr.DecidedAt, pr.SentAt)
	if err != nil {
		return fmt.Errorf("save pending reply: %w", err)
	}
	return nil
}

func (r *PendingReplyRepo) GetByID(ctx context.Context, userID, id uuid.UUID) (*PendingReply, error) {
	var pr PendingReply
	var channel, kind, status string
	err := r.q(ctx).QueryRow(ctx,
		`SELECT `+pendingReplyColumns+`
		 FROM pending_replies
		 WHERE id = $1 AND user_id = $2`,
		id, userID).
		Scan(&pr.ID, &pr.UserID, &pr.LeadID, &channel, &kind, &pr.Body, &status,
			&pr.CreatedAt, &pr.DecidedAt, &pr.SentAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get pending reply by id: %w", err)
	}
	pr.Channel = Channel(channel)
	pr.Kind = PendingReplyKind(kind)
	pr.Status = PendingReplyStatus(status)
	return &pr, nil
}

func (r *PendingReplyRepo) ListByLead(ctx context.Context, userID, leadID uuid.UUID) ([]*PendingReply, error) {
	rows, err := r.q(ctx).Query(ctx,
		`SELECT `+pendingReplyColumns+`
		 FROM pending_replies
		 WHERE user_id = $1 AND lead_id = $2
		 ORDER BY created_at DESC`,
		userID, leadID)
	if err != nil {
		return nil, fmt.Errorf("list pending replies by lead: %w", err)
	}
	defer rows.Close()
	out := make([]*PendingReply, 0)
	for rows.Next() {
		var pr PendingReply
		var channel, kind, status string
		if err := rows.Scan(&pr.ID, &pr.UserID, &pr.LeadID, &channel, &kind, &pr.Body, &status,
			&pr.CreatedAt, &pr.DecidedAt, &pr.SentAt); err != nil {
			return nil, fmt.Errorf("scan pending reply: %w", err)
		}
		pr.Channel = Channel(channel)
		pr.Kind = PendingReplyKind(kind)
		pr.Status = PendingReplyStatus(status)
		out = append(out, &pr)
	}
	return out, rows.Err()
}

// Update persists status, decided_at and sent_at for an existing row.
// Scoped by user_id and id to prevent a malformed entity from updating
// somebody else's row even if the caller forgets to re-check ownership.
// Body, channel, kind and created_at are immutable after Save by
// contract — the entity exposes no setters for them.
func (r *PendingReplyRepo) Update(ctx context.Context, pr *PendingReply, expectedStatus PendingReplyStatus) error {
	tag, err := r.q(ctx).Exec(ctx,
		`UPDATE pending_replies
		 SET status = $1, decided_at = $2, sent_at = $3
		 WHERE id = $4 AND user_id = $5 AND status = $6`,
		string(pr.Status), pr.DecidedAt, pr.SentAt, pr.ID, pr.UserID, string(expectedStatus))
	if err != nil {
		return fmt.Errorf("update pending reply: %w", err)
	}
	if tag.RowsAffected() == 0 {
		// Either the row never existed, was deleted, or belongs to
		// another tenant — all three are indistinguishable on
		// purpose so callers cannot leak existence cross-tenant.
		return ErrPendingReplyNotFound
	}
	return nil
}
