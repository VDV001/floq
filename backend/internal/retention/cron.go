// Package retention provides a queue-agnostic background sweep that deletes
// terminal rows from a table-as-queue once they age past a retention window.
// The qualification-jobs and webhook-deliveries queues both accumulate terminal
// rows forever (#206 / #181); this shared loop drives each queue's own Purger so
// the ticker plumbing is written once (#212). It deliberately does NOT own the
// retention policy — each queue's Purger computes its own cut-off, so windows can
// differ per queue.
package retention

import (
	"context"
	"log/slog"
	"time"
)

// Purger runs one retention sweep and returns how many rows it deleted. Kept to
// a single method so each queue's use case satisfies it directly (DIP: declared
// here, the consumer).
type Purger interface {
	Purge(ctx context.Context) (int, error)
}

// Cron periodically drives a Purger until its context is cancelled. It mirrors
// audit.RetentionCron / webhooks.DeliveryCron: run once on startup so a freshly
// started server clears any backlog that accrued while it was down, then on each
// interval tick. name labels the queue in log lines.
type Cron struct {
	name     string
	purger   Purger
	interval time.Duration
	logger   *slog.Logger
}

// NewCron builds the cron. A nil logger falls back to the default.
func NewCron(name string, p Purger, interval time.Duration, logger *slog.Logger) *Cron {
	if logger == nil {
		logger = slog.Default()
	}
	return &Cron{name: name, purger: p, interval: interval, logger: logger}
}

// Start runs the retention loop until ctx is cancelled. Blocking; run in a
// goroutine.
func (c *Cron) Start(ctx context.Context) {
	c.logger.Info("retention cron started", "queue", c.name, "interval", c.interval)
	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()

	c.runOnce(ctx)
	for {
		select {
		case <-ctx.Done():
			c.logger.Info("retention cron shutting down", "queue", c.name)
			return
		case <-ticker.C:
			c.runOnce(ctx)
		}
	}
}

func (c *Cron) runOnce(ctx context.Context) {
	purged, err := c.purger.Purge(ctx)
	if err != nil {
		c.logger.Warn("retention pass failed", "queue", c.name, "err", err)
		return
	}
	if purged > 0 {
		c.logger.Info("retention pass complete", "queue", c.name, "purged", purged)
	}
}
