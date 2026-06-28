package inbox

import (
	"context"
	"fmt"
	"time"

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

// PurgeTerminalJobsOlderThan deletes terminal (done/failed) jobs whose terminal
// transition (updated_at) predates threshold, returning the row count. Pending
// jobs are never touched — the status filter spares any in-flight or retrying
// row regardless of age. Runs on the pool; GC needs no transaction (#212). The
// terminal statuses are passed as a parameter built from the domain enum so the
// query never drifts from JobDone/JobFailed.
func (r *QualificationJobRepo) PurgeTerminalJobsOlderThan(ctx context.Context, threshold time.Time) (int, error) {
	tag, err := r.q(ctx).Exec(ctx, `
		DELETE FROM lead_qualification_jobs
		WHERE status = ANY($1) AND updated_at < $2`,
		[]string{string(JobDone), string(JobFailed)}, threshold)
	if err != nil {
		return 0, err
	}
	return int(tag.RowsAffected()), nil
}

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

// ClaimDueQualificationJob atomically claims and leases the single earliest-due
// pending job (attempts below maxAttempts), returning nil when none is
// claimable. The effective due-time is COALESCE(next_retry_at, created_at) (the
// backoff schedule, or — when never attempted — the enqueue time); both the
// inner WHERE and ORDER BY key on it so the subselect rides
// idx_lead_qualification_jobs_due (migration 053). The id tiebreak makes the
// order total and stable.
//
// Multi-worker safety (#212): the inner SELECT takes the row under FOR UPDATE
// SKIP LOCKED, and the UPDATE marks it locked_until = now()+leaseSeconds. The
// claim filter skips rows whose lease is still in the future, so a second worker
// moves to the next row instead of double-processing. Claiming one row at a time
// and processing it immediately means the lease only has to outlast a single
// item, independent of batch size; a crashed worker's lease expires and the row
// becomes reclaimable with no separate recovery sweep.
func (r *QualificationJobRepo) ClaimDueQualificationJob(ctx context.Context, maxAttempts, leaseSeconds int) (*QualificationJob, error) {
	rows, err := r.q(ctx).Query(ctx, `
		WITH due AS (
			SELECT id FROM lead_qualification_jobs
			WHERE status = 'pending' AND attempts < $1
			  AND COALESCE(next_retry_at, created_at) <= now()
			  AND (locked_until IS NULL OR locked_until <= now())
			ORDER BY COALESCE(next_retry_at, created_at), id
			LIMIT 1
			FOR UPDATE SKIP LOCKED
		)
		UPDATE lead_qualification_jobs t
		SET locked_until = now() + make_interval(secs => $2)
		FROM due
		WHERE t.id = due.id
		RETURNING t.id, t.lead_id, t.user_id, t.contact_name, t.channel,
		          t.qualify_text, t.status, t.attempts, t.last_error, t.next_retry_at`,
		maxAttempts, leaseSeconds)
	if err != nil {
		return nil, fmt.Errorf("inbox: claim due qualification job: %w", err)
	}
	defer rows.Close()

	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return nil, fmt.Errorf("inbox: claim due qualification job: %w", err)
		}
		return nil, nil
	}
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
	return &j, rows.Err()
}

// SaveQualificationJob persists the outcome of a processing attempt.
func (r *QualificationJobRepo) SaveQualificationJob(ctx context.Context, j *QualificationJob) error {
	// locked_until is cleared: the processing attempt that held the lease is over.
	// For a terminal outcome the lease is moot; for a retry, clearing it lets
	// next_retry_at alone govern re-claim instead of stalling until the lease
	// would have expired (#212).
	_, err := r.q(ctx).Exec(ctx, `
		UPDATE lead_qualification_jobs
		SET status = $2, attempts = $3, last_error = $4, next_retry_at = $5,
		    locked_until = NULL, updated_at = now()
		WHERE id = $1`,
		j.ID, string(j.Status), j.Attempts, j.LastError, j.NextRetryAt)
	if err != nil {
		return fmt.Errorf("inbox: save qualification job: %w", err)
	}
	return nil
}
