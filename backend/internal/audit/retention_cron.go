package audit

import (
	"context"
	"log/slog"
	"time"
)

// purger is the slice of RetentionUseCase the cron drives. An interface
// so the loop is unit-testable with a fake.
type purger interface {
	Purge(ctx context.Context) (int, error)
}

// RetentionCron periodically rolls audit_log rows older than the
// retention window into the daily aggregate and deletes them (#101). It
// mirrors onec.ReconcileCron / reminders.Cron: a ticker loop that runs
// once on startup and stops when its context is cancelled, so it shuts
// down gracefully with the server.
type RetentionCron struct {
	uc       purger
	interval time.Duration
	logger   *slog.Logger
}

// NewRetentionCron builds the cron. A nil logger falls back to default.
func NewRetentionCron(uc purger, interval time.Duration, logger *slog.Logger) *RetentionCron {
	if logger == nil {
		logger = slog.Default()
	}
	return &RetentionCron{uc: uc, interval: interval, logger: logger}
}

// Start runs the retention loop until ctx is cancelled. It runs once
// immediately so a freshly started server reclaims any backlog that
// accrued while it was down, then on each interval tick.
func (c *RetentionCron) Start(ctx context.Context) {
	c.logger.Info("audit: retention cron started", "interval", c.interval)
	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()

	c.runOnce(ctx)
	for {
		select {
		case <-ctx.Done():
			c.logger.Info("audit: retention cron shutting down")
			return
		case <-ticker.C:
			c.runOnce(ctx)
		}
	}
}

func (c *RetentionCron) runOnce(ctx context.Context) {
	purged, err := c.uc.Purge(ctx)
	if err != nil {
		c.logger.Warn("audit: retention pass failed", "err", err)
		return
	}
	if purged > 0 {
		c.logger.Info("audit: retention pass complete", "purged", purged)
	}
}
