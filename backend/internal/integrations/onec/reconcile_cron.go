package onec

import (
	"context"
	"log/slog"
	"time"
)

// reconciler is the slice of ReconcileUseCase the cron drives. An interface so
// the cron's loop is unit-testable with a fake.
type reconciler interface {
	ReconcileAll(ctx context.Context) error
}

// ReconcileCron periodically runs the 1C reconciliation safety net (#109). It
// mirrors reminders.Cron: a ticker loop that runs once on startup and stops when
// its context is cancelled, so it shuts down gracefully with the server.
type ReconcileCron struct {
	uc       reconciler
	interval time.Duration
	logger   *slog.Logger
}

// NewReconcileCron builds the cron. A nil logger falls back to the default.
func NewReconcileCron(uc reconciler, interval time.Duration, logger *slog.Logger) *ReconcileCron {
	if logger == nil {
		logger = slog.Default()
	}
	return &ReconcileCron{uc: uc, interval: interval, logger: logger}
}

// Start runs the reconciliation loop until ctx is cancelled. It runs once
// immediately so a freshly started server closes any gap that accrued while it
// was down, then on each interval tick.
func (c *ReconcileCron) Start(ctx context.Context) {
	c.logger.Info("onec: reconcile cron started", "interval", c.interval)
	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()

	c.runOnce(ctx)
	for {
		select {
		case <-ctx.Done():
			c.logger.Info("onec: reconcile cron shutting down")
			return
		case <-ticker.C:
			c.runOnce(ctx)
		}
	}
}

func (c *ReconcileCron) runOnce(ctx context.Context) {
	if err := c.uc.ReconcileAll(ctx); err != nil {
		c.logger.Warn("onec: reconcile pass failed", "err", err)
	}
}
