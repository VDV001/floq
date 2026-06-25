package analytics

import (
	"context"
	"log/slog"
	"time"
)

// refresher is the slice of Repository the refresh cron drives — an
// interface so the loop is unit-testable with a fake.
type refresher interface {
	RefreshMatviews(ctx context.Context) error
}

// RefreshCron periodically rebuilds the funnel materialized views so the
// analytics read-path serves fresh aggregates without running the heavy
// GROUP BYs on the OLTP query path. Mirrors audit.RetentionCron: a ticker
// loop that runs once on startup and stops when its context is cancelled,
// so it shuts down gracefully with the server.
type RefreshCron struct {
	repo     refresher
	interval time.Duration
	logger   *slog.Logger
}

// NewRefreshCron builds the cron. A nil logger falls back to default.
func NewRefreshCron(repo refresher, interval time.Duration, logger *slog.Logger) *RefreshCron {
	if logger == nil {
		logger = slog.Default()
	}
	return &RefreshCron{repo: repo, interval: interval, logger: logger}
}

// Start runs the refresh loop until ctx is cancelled. It refreshes once
// immediately so a freshly started server serves current aggregates, then
// on each interval tick.
func (c *RefreshCron) Start(ctx context.Context) {
	c.logger.Info("analytics: matview refresh cron started", "interval", c.interval)
	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()

	c.runOnce(ctx)
	for {
		select {
		case <-ctx.Done():
			c.logger.Info("analytics: matview refresh cron shutting down")
			return
		case <-ticker.C:
			c.runOnce(ctx)
		}
	}
}

func (c *RefreshCron) runOnce(ctx context.Context) {
	if err := c.repo.RefreshMatviews(ctx); err != nil {
		c.logger.Warn("analytics: matview refresh failed", "err", err)
	}
}
