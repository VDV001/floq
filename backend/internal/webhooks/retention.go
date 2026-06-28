package webhooks

import (
	"context"
	"fmt"
	"time"
)

// terminalDeliveryPurger is the repository slice DeliveryRetention needs:
// deleting terminal deliveries older than a cut-off. Declared here (the
// consumer) per DIP; Repository satisfies it.
type terminalDeliveryPurger interface {
	PurgeTerminalDeliveriesOlderThan(ctx context.Context, threshold time.Time) (int, error)
}

// DeliveryRetention deletes terminal (succeeded/failed) webhook deliveries once
// they age past retentionDays. It owns the clock so the cron stays time-free —
// the retention boundary is a policy decision, not a transport concern (#212).
// Satisfies retention.Purger.
type DeliveryRetention struct {
	repo terminalDeliveryPurger
	days int
	now  func() time.Time
}

// NewDeliveryRetention builds the use case. retentionDays is how long a terminal
// delivery is kept after it settles before being swept.
func NewDeliveryRetention(repo terminalDeliveryPurger, retentionDays int) *DeliveryRetention {
	return &DeliveryRetention{repo: repo, days: retentionDays, now: time.Now}
}

// Purge deletes every terminal delivery whose terminal transition (updated_at)
// predates now-retentionDays, returning how many rows were removed.
func (r *DeliveryRetention) Purge(ctx context.Context) (int, error) {
	threshold := r.now().AddDate(0, 0, -r.days)
	n, err := r.repo.PurgeTerminalDeliveriesOlderThan(ctx, threshold)
	if err != nil {
		return 0, fmt.Errorf("webhook delivery retention purge: %w", err)
	}
	return n, nil
}
