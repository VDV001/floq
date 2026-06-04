package inbox

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/daniil/floq/internal/db"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// pendingReplyDedupIndex is the constraint name the partial-unique
// dedup index ships under in migration 031. The repository checks
// against this exact name when translating a 23505 unique_violation
// to the domain sentinel — any OTHER unique violation on this table
// must still propagate as a raw error so future indexes don't get
// silently flattened into dedup semantics.
const pendingReplyDedupIndex = "idx_pending_replies_dedup_pending"

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

const pendingReplyColumns = `id, user_id, lead_id, channel, kind, body, status, created_at, decided_at, decided_by, sent_at`

func (r *PendingReplyRepo) Save(ctx context.Context, pr *PendingReply) error {
	_, err := r.q(ctx).Exec(ctx,
		`INSERT INTO pending_replies (`+pendingReplyColumns+`)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`,
		pr.ID, pr.UserID, pr.LeadID, string(pr.Channel), string(pr.Kind), pr.Body,
		string(pr.Status), pr.CreatedAt, pr.DecidedAt, pr.DecidedBy, pr.SentAt)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == pendingReplyDedupIndex {
			return ErrPendingReplyDuplicatePending
		}
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
			&pr.CreatedAt, &pr.DecidedAt, &pr.DecidedBy, &pr.SentAt)
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

// FindPendingByContent locates the existing pending row whose
// (user_id, lead_id, kind, body) matches — used by the usecase after a
// Save collides with the partial-unique dedup index installed in
// migration 031. Returns nil without error when no such row exists.
// Body is trimmed to match the factory's storage normalisation so
// callers passing raw whitespace-padded content still hit the row.
func (r *PendingReplyRepo) FindPendingByContent(ctx context.Context, userID, leadID uuid.UUID, kind PendingReplyKind, body string) (*PendingReply, error) {
	trimmed := strings.TrimSpace(body)
	var pr PendingReply
	var channel, kindStr, status string
	err := r.q(ctx).QueryRow(ctx,
		`SELECT `+pendingReplyColumns+`
		 FROM pending_replies
		 WHERE user_id = $1 AND lead_id = $2 AND kind = $3 AND body = $4 AND status = 'pending'
		 LIMIT 1`,
		userID, leadID, string(kind), trimmed).
		Scan(&pr.ID, &pr.UserID, &pr.LeadID, &channel, &kindStr, &pr.Body, &status,
			&pr.CreatedAt, &pr.DecidedAt, &pr.DecidedBy, &pr.SentAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("find pending by content: %w", err)
	}
	pr.Channel = Channel(channel)
	pr.Kind = PendingReplyKind(kindStr)
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
			&pr.CreatedAt, &pr.DecidedAt, &pr.DecidedBy, &pr.SentAt); err != nil {
			return nil, fmt.Errorf("scan pending reply: %w", err)
		}
		pr.Channel = Channel(channel)
		pr.Kind = PendingReplyKind(kind)
		pr.Status = PendingReplyStatus(status)
		out = append(out, &pr)
	}
	return out, rows.Err()
}

// ListPendingByUser returns every status='pending' row for the user
// joined with the minimum lead context the operator queue needs to
// render — contact + company + channel + identifiers — in one query
// so the frontend avoids N+1 lookups. user_id is enforced on both
// sides of the join as defense in depth: even though leads.user_id
// and pending_replies.user_id should match for the same lead_id, an
// explicit join predicate makes the cross-tenant invariant local to
// the SQL rather than relying on FK semantics.
//
// Returns an empty (non-nil) slice when nothing is pending so callers
// can iterate without a nil-check.
func (r *PendingReplyRepo) ListPendingByUser(ctx context.Context, userID uuid.UUID) ([]*PendingReplyWithLead, error) {
	rows, err := r.q(ctx).Query(ctx,
		`SELECT pr.id, pr.user_id, pr.lead_id, pr.channel, pr.kind, pr.body, pr.status,
		        pr.created_at, pr.decided_at, pr.decided_by, pr.sent_at,
		        l.contact_name, l.company, l.channel, l.telegram_chat_id, l.email_address
		 FROM pending_replies pr
		 JOIN leads l ON l.id = pr.lead_id AND l.user_id = pr.user_id
		 WHERE pr.user_id = $1 AND pr.status = 'pending'
		 ORDER BY pr.created_at DESC`,
		userID)
	if err != nil {
		return nil, fmt.Errorf("list pending replies by user: %w", err)
	}
	defer rows.Close()
	out := make([]*PendingReplyWithLead, 0)
	for rows.Next() {
		var pr PendingReply
		var channel, kind, status string
		var leadChannel string
		var snippet LeadSnippet
		if err := rows.Scan(&pr.ID, &pr.UserID, &pr.LeadID, &channel, &kind, &pr.Body, &status,
			&pr.CreatedAt, &pr.DecidedAt, &pr.DecidedBy, &pr.SentAt,
			&snippet.ContactName, &snippet.Company, &leadChannel,
			&snippet.TelegramChatID, &snippet.EmailAddress); err != nil {
			return nil, fmt.Errorf("scan pending reply with lead: %w", err)
		}
		pr.Channel = Channel(channel)
		pr.Kind = PendingReplyKind(kind)
		pr.Status = PendingReplyStatus(status)
		snippet.Channel = Channel(leadChannel)
		out = append(out, &PendingReplyWithLead{Reply: &pr, Lead: snippet})
	}
	return out, rows.Err()
}

// CountPendingByUser returns the per-lead count of rows still awaiting
// operator decision (status='pending') for the given user. Used by
// the leads-context inbox-list badge.
func (r *PendingReplyRepo) CountPendingByUser(ctx context.Context, userID uuid.UUID) (map[uuid.UUID]int, error) {
	rows, err := r.q(ctx).Query(ctx,
		`SELECT lead_id, COUNT(*)
		 FROM pending_replies
		 WHERE user_id = $1 AND status = 'pending'
		 GROUP BY lead_id`,
		userID)
	if err != nil {
		return nil, fmt.Errorf("count pending replies by user: %w", err)
	}
	defer rows.Close()
	out := make(map[uuid.UUID]int)
	for rows.Next() {
		var leadID uuid.UUID
		var count int
		if err := rows.Scan(&leadID, &count); err != nil {
			return nil, fmt.Errorf("scan pending count: %w", err)
		}
		out[leadID] = count
	}
	return out, rows.Err()
}

// CountPendingByKind returns the total number of rows still awaiting an
// operator decision (status='pending'), grouped by reply kind ACROSS all
// users. It is deliberately tenant-aggregate: the only consumer is the
// public queue-depth metric, which must not carry a per-user label.
func (r *PendingReplyRepo) CountPendingByKind(ctx context.Context) (map[string]int, error) {
	rows, err := r.q(ctx).Query(ctx,
		`SELECT kind, COUNT(*)
		 FROM pending_replies
		 WHERE status = 'pending'
		 GROUP BY kind`)
	if err != nil {
		return nil, fmt.Errorf("count pending replies by kind: %w", err)
	}
	defer rows.Close()
	out := make(map[string]int)
	for rows.Next() {
		var kind string
		var count int
		if err := rows.Scan(&kind, &count); err != nil {
			return nil, fmt.Errorf("scan pending kind count: %w", err)
		}
		out[kind] = count
	}
	return out, rows.Err()
}

// Update persists status, decided_at and sent_at for an existing row.
// Scoped by user_id and id to prevent a malformed entity from updating
// somebody else's row even if the caller forgets to re-check ownership.
// Body, channel, kind and created_at are immutable after Save by
// contract — the entity exposes no setters for them.
// UpdateBody writes the body column only, with the same WHERE
// (id, user_id, status) optimistic-lock as Update. status is always
// PendingReplyStatusPending at the call site — Approved / Sent /
// Rejected rows are immutable per the domain invariant. Integration
// tests in pending_reply_repository_test.go pin the SQL contract.
func (r *PendingReplyRepo) UpdateBody(ctx context.Context, pr *PendingReply, expectedStatus PendingReplyStatus) error {
	tag, err := r.q(ctx).Exec(ctx,
		`UPDATE pending_replies
		 SET body = $1
		 WHERE id = $2 AND user_id = $3 AND status = $4`,
		pr.Body, pr.ID, pr.UserID, string(expectedStatus))
	if err != nil {
		return fmt.Errorf("update pending reply body: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrPendingReplyNotFound
	}
	return nil
}

func (r *PendingReplyRepo) Update(ctx context.Context, pr *PendingReply, expectedStatus PendingReplyStatus) error {
	tag, err := r.q(ctx).Exec(ctx,
		`UPDATE pending_replies
		 SET status = $1, decided_at = $2, decided_by = $3, sent_at = $4
		 WHERE id = $5 AND user_id = $6 AND status = $7`,
		string(pr.Status), pr.DecidedAt, pr.DecidedBy, pr.SentAt, pr.ID, pr.UserID, string(expectedStatus))
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
