package webhooks

import (
	"context"
	"log/slog"
	"time"
)

// processor is the slice of UseCase the cron drives — an interface so the loop
// is unit-testable with a fake.
type processor interface {
	ProcessPending(ctx context.Context) (int, error)
}

// DeliveryCron periodically drains the webhook delivery outbox (#181). It
// mirrors EnrichmentCron: a ticker loop that runs once on startup (to drain any
// backlog accrued while the server was down) and stops when its context is
// cancelled, shutting down gracefully with the server.
type DeliveryCron struct {
	uc       processor
	interval time.Duration
	logger   *slog.Logger
}

// NewDeliveryCron builds the cron. A nil logger falls back to default.
func NewDeliveryCron(uc processor, interval time.Duration, logger *slog.Logger) *DeliveryCron {
	if logger == nil {
		logger = slog.Default()
	}
	return &DeliveryCron{uc: uc, interval: interval, logger: logger}
}

// Start runs the delivery loop until ctx is cancelled.
func (c *DeliveryCron) Start(ctx context.Context) {
	c.logger.Info("webhooks: delivery cron started", "interval", c.interval)
	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()

	c.runOnce(ctx)
	for {
		select {
		case <-ctx.Done():
			c.logger.Info("webhooks: delivery cron shutting down")
			return
		case <-ticker.C:
			c.runOnce(ctx)
		}
	}
}

func (c *DeliveryCron) runOnce(ctx context.Context) {
	delivered, err := c.uc.ProcessPending(ctx)
	if err != nil {
		c.logger.Warn("webhooks: delivery pass failed", "err", err)
		return
	}
	if delivered > 0 {
		c.logger.Info("webhooks: delivery pass complete", "delivered", delivered)
	}
}
