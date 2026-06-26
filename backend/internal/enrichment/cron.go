package enrichment

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

// EnrichmentCron periodically scrapes due company domains (#182). It mirrors
// audit.RetentionCron / onec.ReconcileCron: a ticker loop that runs once on
// startup (to drain any backlog accrued while the server was down) and stops
// when its context is cancelled, shutting down gracefully with the server.
type EnrichmentCron struct {
	uc       processor
	interval time.Duration
	logger   *slog.Logger
}

// NewEnrichmentCron builds the cron. A nil logger falls back to default.
func NewEnrichmentCron(uc processor, interval time.Duration, logger *slog.Logger) *EnrichmentCron {
	if logger == nil {
		logger = slog.Default()
	}
	return &EnrichmentCron{uc: uc, interval: interval, logger: logger}
}

// Start runs the enrichment loop until ctx is cancelled.
func (c *EnrichmentCron) Start(ctx context.Context) {
	c.logger.Info("enrichment: cron started", "interval", c.interval)
	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()

	c.runOnce(ctx)
	for {
		select {
		case <-ctx.Done():
			c.logger.Info("enrichment: cron shutting down")
			return
		case <-ticker.C:
			c.runOnce(ctx)
		}
	}
}

func (c *EnrichmentCron) runOnce(ctx context.Context) {
	enriched, err := c.uc.ProcessPending(ctx)
	if err != nil {
		c.logger.Warn("enrichment: pass failed", "err", err)
		return
	}
	if enriched > 0 {
		c.logger.Info("enrichment: pass complete", "enriched", enriched)
	}
}
