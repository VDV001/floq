package inbox

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	auditdomain "github.com/daniil/floq/internal/audit/domain"
	"github.com/google/uuid"
)

// QualificationWorkerConfig tunes the worker's per-tick claim budget, dead-letter
// cap, and claim lease.
type QualificationWorkerConfig struct {
	BatchLimit  int
	MaxAttempts int
	// Lease is how long a claimed job is reserved for this worker. It must exceed
	// a single job's processing time (the AI Qualify call) so the row is not
	// reclaimed mid-flight; a crashed worker's lease expires after it, making the
	// job reclaimable (#212). Independent of BatchLimit — jobs are claimed and
	// processed one at a time.
	Lease time.Duration
}

// QualificationWorker drains the lead_qualification_jobs queue (#206 Part C). It
// replaces the old fire-and-forget qualification goroutine: claim due jobs, run
// the AI qualifier, and commit the qualification + lead status + lead.qualified
// webhook in one transaction (fail-closed). A failed AI call or a failed commit
// reschedules the job with exponential backoff until the attempt cap, then
// dead-letters it. Qualification runs whether or not webhooks are enabled — the
// emitter is the only webhooks-gated part.
type QualificationWorker struct {
	store   QualificationJobStore
	ai      AIQualifier
	writer  QualificationWriter
	emitter LeadQualifiedEmitter
	tx      TxManager
	cfg     QualificationWorkerConfig
	logger  *slog.Logger
}

// NewQualificationWorker wires the worker. The emitter and tx manager are set
// separately (SetLeadQualifiedEmitter / SetTxManager) so the composition root can
// leave the emitter off when webhooks are disabled.
func NewQualificationWorker(store QualificationJobStore, ai AIQualifier, writer QualificationWriter, cfg QualificationWorkerConfig) *QualificationWorker {
	if cfg.BatchLimit <= 0 {
		cfg.BatchLimit = 50
	}
	if cfg.MaxAttempts <= 0 {
		cfg.MaxAttempts = 5
	}
	if cfg.Lease <= 0 {
		// A zero lease would set locked_until = now() and void multi-worker
		// protection; floor it like the other knobs (#212).
		cfg.Lease = 5 * time.Minute
	}
	return &QualificationWorker{store: store, ai: ai, writer: writer, cfg: cfg, logger: slog.Default()}
}

// SetLeadQualifiedEmitter wires the transactional lead.qualified emitter. Nil
// (webhooks off) → qualification still runs, just without the event.
func (w *QualificationWorker) SetLeadQualifiedEmitter(e LeadQualifiedEmitter) { w.emitter = e }

// SetTxManager wires the transaction manager that makes the commit atomic.
func (w *QualificationWorker) SetTxManager(tx TxManager) { w.tx = tx }

// SetLogger overrides the default logger.
func (w *QualificationWorker) SetLogger(l *slog.Logger) {
	if l != nil {
		w.logger = l
	}
}

// ProcessPending drains up to BatchLimit due jobs this tick, claiming and
// processing them one at a time. Each claim leases exactly the row it hands back
// and processing follows immediately, so the lease only has to outlast a single
// job and two instances never process the same row (#212). Returns the number
// successfully qualified; one job's failure never aborts the tick.
func (w *QualificationWorker) ProcessPending(ctx context.Context) (int, error) {
	leaseSecs := int(w.cfg.Lease.Seconds())
	qualified := 0
	for i := 0; i < w.cfg.BatchLimit; i++ {
		j, err := w.store.ClaimDueQualificationJob(ctx, w.cfg.MaxAttempts, leaseSecs)
		if err != nil {
			return qualified, fmt.Errorf("inbox: claim qualification job: %w", err)
		}
		if j == nil {
			break // no more claimable rows this tick
		}
		if w.processOne(ctx, j) {
			qualified++
		}
	}
	return qualified, nil
}

// processOne runs one job: qualify (outside the tx — it is a slow external call),
// then commit the writes + emit + job-done in one transaction. Any failure
// reschedules the job (or dead-letters it at the cap). A panic is contained so it
// never takes down the worker loop.
func (w *QualificationWorker) processOne(ctx context.Context, j *QualificationJob) (ok bool) {
	now := time.Now().UTC()
	// Snapshot the attempt count at claim. commit() may run j.MarkDone (which
	// increments Attempts) inside the transaction before failing; the error-path
	// MarkFailed below would then double-count. Restoring to this snapshot makes
	// every failed pass count as exactly one attempt.
	attemptsAtClaim := j.Attempts
	defer func() {
		if r := recover(); r != nil {
			w.logger.ErrorContext(ctx, "inbox: panic qualifying lead", "lead", j.LeadID, "panic", r)
			w.fail(ctx, j, attemptsAtClaim, fmt.Sprintf("panic: %v", r), now)
			ok = false
		}
	}()

	// Fresh CallMeta so the AI call is attributed and cost-audited (#182): a
	// background worker must BUILD the meta, not inherit it.
	qCtx := auditdomain.ContextWithCallMeta(ctx, auditdomain.CallMeta{
		UserID:      j.UserID,
		LeadID:      &j.LeadID,
		RequestType: auditdomain.RequestTypeQualification,
	})
	result, err := w.ai.Qualify(qCtx, j.ContactName, string(j.Channel), j.QualifyText)
	if err != nil {
		w.logger.WarnContext(ctx, "inbox: qualification failed", "lead", j.LeadID, "err", err)
		w.fail(ctx, j, attemptsAtClaim, err.Error(), now)
		return false
	}

	q := &InboxQualification{
		ID:                uuid.New(),
		LeadID:            j.LeadID,
		IdentifiedNeed:    result.IdentifiedNeed,
		EstimatedBudget:   result.EstimatedBudget,
		Deadline:          result.Deadline,
		Score:             result.Score,
		ScoreReason:       result.ScoreReason,
		RecommendedAction: result.RecommendedAction,
		ProviderUsed:      w.ai.ProviderName(),
		GeneratedAt:       now,
	}

	if err := w.commit(ctx, j, q, now); err != nil {
		w.logger.WarnContext(ctx, "inbox: qualification commit failed", "lead", j.LeadID, "err", err)
		w.fail(ctx, j, attemptsAtClaim, err.Error(), now)
		return false
	}
	return true
}

// fail records a failed processing attempt: it restores the attempt count to its
// claim-time value (commit() may have run MarkDone++ before failing) so the
// MarkFailed++ counts the pass exactly once, then schedules the retry (or
// dead-letters at the cap) and persists outside any rolled-back transaction.
func (w *QualificationWorker) fail(ctx context.Context, j *QualificationJob, attemptsAtClaim int, reason string, now time.Time) {
	j.Attempts = attemptsAtClaim
	j.MarkFailed(reason, w.cfg.MaxAttempts, now)
	w.save(ctx, j)
}

// commit persists the qualification, flips the lead to qualified, emits
// lead.qualified (when an emitter is wired), and marks the job done — all in one
// transaction so a failed emit rolls back the whole qualification (fail-closed)
// and the job stays pending for retry.
func (w *QualificationWorker) commit(ctx context.Context, j *QualificationJob, q *InboxQualification, now time.Time) error {
	fn := func(txCtx context.Context) error {
		if err := w.writer.UpsertQualification(txCtx, q); err != nil {
			return err
		}
		if err := w.writer.UpdateLeadStatus(txCtx, j.LeadID, StatusQualified); err != nil {
			return err
		}
		if w.emitter != nil {
			lead, err := w.writer.GetLead(txCtx, j.LeadID)
			if err != nil {
				return err
			}
			if lead == nil {
				return fmt.Errorf("inbox: lead %s not found for qualified emit", j.LeadID)
			}
			lead.Status = StatusQualified
			if err := w.emitter.EmitLeadQualified(txCtx, lead); err != nil {
				return err
			}
		}
		j.MarkDone(now)
		return w.store.SaveQualificationJob(txCtx, j)
	}
	if w.tx != nil {
		return w.tx.WithTx(ctx, fn)
	}
	return fn(ctx)
}

func (w *QualificationWorker) save(ctx context.Context, j *QualificationJob) {
	if err := w.store.SaveQualificationJob(ctx, j); err != nil {
		w.logger.ErrorContext(ctx, "inbox: save qualification job", "job", j.ID, "err", err)
	}
}
