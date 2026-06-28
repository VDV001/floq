package inbox

import (
	"context"
	"log/slog"
	"time"
)

// qualProcessor is the slice of QualificationWorker the cron drives — an
// interface so the loop is unit-testable with a fake.
type qualProcessor interface {
	ProcessPending(ctx context.Context) (int, error)
}

// QualificationCron periodically drains the lead_qualification_jobs queue (#206
// Part C). It mirrors webhooks.DeliveryCron: a ticker loop that runs once on
// startup (to drain any backlog accrued while the server was down) and stops
// when its context is cancelled, shutting down gracefully with the server.
type QualificationCron struct {
	uc       qualProcessor
	interval time.Duration
	logger   *slog.Logger
}

// NewQualificationCron builds the cron. A nil logger falls back to default.
func NewQualificationCron(uc qualProcessor, interval time.Duration, logger *slog.Logger) *QualificationCron {
	if logger == nil {
		logger = slog.Default()
	}
	return &QualificationCron{uc: uc, interval: interval, logger: logger}
}

// Start runs the qualification loop until ctx is cancelled.
func (c *QualificationCron) Start(ctx context.Context) {
	c.logger.Info("inbox: qualification cron started", "interval", c.interval)
	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()

	c.runOnce(ctx)
	for {
		select {
		case <-ctx.Done():
			c.logger.Info("inbox: qualification cron shutting down")
			return
		case <-ticker.C:
			c.runOnce(ctx)
		}
	}
}

func (c *QualificationCron) runOnce(ctx context.Context) {
	qualified, err := c.uc.ProcessPending(ctx)
	if err != nil {
		c.logger.Warn("inbox: qualification pass failed", "err", err)
		return
	}
	if qualified > 0 {
		c.logger.Info("inbox: qualification pass complete", "qualified", qualified)
	}
}
