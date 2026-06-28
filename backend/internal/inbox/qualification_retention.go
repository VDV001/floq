package inbox

import (
	"context"
	"fmt"
	"time"
)

// terminalJobPurger is the repository slice QualificationRetention needs:
// deleting terminal qualification jobs older than a cut-off. Declared here (the
// consumer) per DIP; QualificationJobRepo satisfies it.
type terminalJobPurger interface {
	PurgeTerminalJobsOlderThan(ctx context.Context, threshold time.Time) (int, error)
}

// QualificationRetention deletes terminal (done/failed) qualification jobs once
// they age past retentionDays. It owns the clock so the cron stays time-free —
// the retention boundary is a policy decision, not a transport concern (#212).
// Satisfies retention.Purger.
type QualificationRetention struct {
	repo terminalJobPurger
	days int
	now  func() time.Time
}

// NewQualificationRetention builds the use case. retentionDays is how long a
// terminal job is kept after it finishes before being swept.
func NewQualificationRetention(repo terminalJobPurger, retentionDays int) *QualificationRetention {
	return &QualificationRetention{repo: repo, days: retentionDays, now: time.Now}
}

// Purge deletes every terminal job whose terminal transition (updated_at)
// predates now-retentionDays, returning how many rows were removed.
func (r *QualificationRetention) Purge(ctx context.Context) (int, error) {
	threshold := r.now().AddDate(0, 0, -r.days)
	n, err := r.repo.PurgeTerminalJobsOlderThan(ctx, threshold)
	if err != nil {
		return 0, fmt.Errorf("qualification retention purge: %w", err)
	}
	return n, nil
}
