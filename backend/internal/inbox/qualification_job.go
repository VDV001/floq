package inbox

import (
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
)

// JobStatus is the lifecycle state of one auto-qualification job.
type JobStatus string

const (
	// JobPending is queued or mid-retry: the worker will (re)attempt it.
	JobPending JobStatus = "pending"
	// JobDone qualified successfully; terminal.
	JobDone JobStatus = "done"
	// JobFailed exhausted its retries; terminal (dead-letter).
	JobFailed JobStatus = "failed"
)

// Enqueue-time invariants. Domain errors (not bare errors.New in a function) so
// callers can errors.Is them.
var (
	// ErrEmptyQualifyText guards against enqueuing a job with nothing to qualify.
	ErrEmptyQualifyText = errors.New("inbox: qualification job text is empty")
	// ErrEmptyJobLead guards against a job not tied to a lead.
	ErrEmptyJobLead = errors.New("inbox: qualification job lead is empty")
	// ErrEmptyJobOwner guards against a job not tied to a user.
	ErrEmptyJobOwner = errors.New("inbox: qualification job owner is empty")
)

// QualRetryBaseBackoff is the first retry delay for a failed qualification; it
// doubles per attempt. Worker-timescale (AI calls are slow and rate-limited), so
// a minute base rather than the webhook delivery's 30s. Exported so the
// repository's due-ness predicate stays in sync with NextRetryAfter.
const QualRetryBaseBackoff = time.Minute

// QualificationJob is the queue record for one auto-qualification. It captures
// the AI qualifier inputs at enqueue time (so a retry re-runs the exact
// qualification, including ephemeral attachment text) and carries the
// pending/done/failed lifecycle the worker drives. It is the unit of work that
// replaces the old fire-and-forget qualification goroutine (#206 Part C).
type QualificationJob struct {
	ID          uuid.UUID
	LeadID      uuid.UUID
	UserID      uuid.UUID
	ContactName string
	Channel     Channel
	QualifyText string
	Status      JobStatus
	Attempts    int
	LastError   string
	// NextRetryAt is when this job is next eligible (nil = due now, i.e. never
	// attempted or terminal). The worker claims rows where it is null or in the
	// past; this is the domain-authoritative backoff schedule.
	NextRetryAt *time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// NewQualificationJob builds a pending job, validating the enqueue invariants.
func NewQualificationJob(leadID, userID uuid.UUID, contactName string, channel Channel, qualifyText string) (*QualificationJob, error) {
	if leadID == uuid.Nil {
		return nil, ErrEmptyJobLead
	}
	if userID == uuid.Nil {
		return nil, ErrEmptyJobOwner
	}
	if strings.TrimSpace(qualifyText) == "" {
		return nil, ErrEmptyQualifyText
	}
	return &QualificationJob{
		ID:          uuid.New(),
		LeadID:      leadID,
		UserID:      userID,
		ContactName: contactName,
		Channel:     channel,
		QualifyText: qualifyText,
		Status:      JobPending,
		Attempts:    0,
	}, nil
}

// MarkDone records a successful qualification (terminal). now is passed in (not
// time.Now) so the transition is deterministic and testable.
func (j *QualificationJob) MarkDone(now time.Time) {
	j.Attempts++
	j.Status = JobDone
	j.LastError = ""
	j.NextRetryAt = nil
}

// MarkFailed records a failed attempt. The job stays pending (retryable) until
// attempts reach maxAttempts, at which point it becomes terminally failed
// (dead-letter). While retryable, NextRetryAt is set to the exponential-backoff
// schedule off now (domain-authoritative); on terminal failure it is cleared.
func (j *QualificationJob) MarkFailed(reason string, maxAttempts int, now time.Time) {
	j.Attempts++
	j.LastError = reason
	if j.Attempts >= maxAttempts {
		j.Status = JobFailed
		j.NextRetryAt = nil
	} else {
		j.Status = JobPending
		next := j.NextRetryAfter(now)
		j.NextRetryAt = &next
	}
}

// NextRetryAfter returns when this job is next eligible, an exponential backoff
// from base by the current attempt count. base is passed in (not time.Now) so it
// is deterministic and testable.
func (j *QualificationJob) NextRetryAfter(base time.Time) time.Time {
	backoff := QualRetryBaseBackoff
	for i := 1; i < j.Attempts; i++ {
		backoff *= 2
	}
	return base.Add(backoff)
}
