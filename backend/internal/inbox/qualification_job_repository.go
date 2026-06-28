package inbox

import (
	"context"
	"fmt"

	"github.com/daniil/floq/internal/db"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Compile-time check that *QualificationJobRepo satisfies the worker store.
var _ QualificationJobStore = (*QualificationJobRepo)(nil)

// QualificationJobRepo is the pgx-backed lead_qualification_jobs queue.
type QualificationJobRepo struct {
	pool *pgxpool.Pool
}

// NewQualificationJobRepository wires the SQL-backed implementation.
func NewQualificationJobRepository(pool *pgxpool.Pool) *QualificationJobRepo {
	return &QualificationJobRepo{pool: pool}
}

// q returns the Querier bound to the current context (a pgx.Tx when the caller
// wrapped the call in db.TxManager.WithTx, otherwise the pool) — so an
// EnqueueQualificationJob inside the lead-intake transaction commits atomically
// with the lead, and the worker's SaveQualificationJob joins its commit tx.
func (r *QualificationJobRepo) q(ctx context.Context) db.Querier {
	return db.ConnFromCtx(ctx, r.pool)
}

const qualificationJobColumns = `id, lead_id, user_id, contact_name, channel, qualify_text, status, attempts, last_error, next_retry_at`

// EnqueueQualificationJob inserts a pending job. created_at/updated_at are
// DB-managed (DEFAULT now()).
func (r *QualificationJobRepo) EnqueueQualificationJob(ctx context.Context, j *QualificationJob) error {
	_, err := r.q(ctx).Exec(ctx,
		`INSERT INTO lead_qualification_jobs (`+qualificationJobColumns+`)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
		j.ID, j.LeadID, j.UserID, j.ContactName, string(j.Channel), j.QualifyText,
		string(j.Status), j.Attempts, j.LastError, j.NextRetryAt)
	if err != nil {
		return fmt.Errorf("inbox: enqueue qualification job: %w", err)
	}
	return nil
}

// ClaimDueQualificationJobs returns up to limit due pending jobs, earliest-due
// first, with attempts below maxAttempts. The effective due-time is
// COALESCE(next_retry_at, created_at) (the backoff schedule, or — when never
// attempted — the enqueue time); both the WHERE and ORDER BY key on it so the
// query rides idx_lead_qualification_jobs_due (migration 053): a forward index
// scan that stops at the first not-due row. The id tiebreak makes the order
// total and stable. A single worker runs this, so no cross-instance lease yet.
func (r *QualificationJobRepo) ClaimDueQualificationJobs(ctx context.Context, limit, maxAttempts int) ([]*QualificationJob, error) {
	rows, err := r.q(ctx).Query(ctx, `
		SELECT `+qualificationJobColumns+`
		FROM lead_qualification_jobs
		WHERE status = 'pending' AND attempts < $2
		  AND COALESCE(next_retry_at, created_at) <= now()
		ORDER BY COALESCE(next_retry_at, created_at), id
		LIMIT $1`, limit, maxAttempts)
	if err != nil {
		return nil, fmt.Errorf("inbox: claim due qualification jobs: %w", err)
	}
	defer rows.Close()

	var out []*QualificationJob
	for rows.Next() {
		var (
			j       QualificationJob
			channel string
			status  string
		)
		if err := rows.Scan(&j.ID, &j.LeadID, &j.UserID, &j.ContactName, &channel,
			&j.QualifyText, &status, &j.Attempts, &j.LastError, &j.NextRetryAt); err != nil {
			return nil, fmt.Errorf("inbox: scan qualification job: %w", err)
		}
		j.Channel = Channel(channel)
		j.Status = JobStatus(status)
		out = append(out, &j)
	}
	return out, rows.Err()
}

// SaveQualificationJob persists the outcome of a processing attempt.
func (r *QualificationJobRepo) SaveQualificationJob(ctx context.Context, j *QualificationJob) error {
	_, err := r.q(ctx).Exec(ctx, `
		UPDATE lead_qualification_jobs
		SET status = $2, attempts = $3, last_error = $4, next_retry_at = $5, updated_at = now()
		WHERE id = $1`,
		j.ID, string(j.Status), j.Attempts, j.LastError, j.NextRetryAt)
	if err != nil {
		return fmt.Errorf("inbox: save qualification job: %w", err)
	}
	return nil
}
