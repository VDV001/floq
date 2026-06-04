package metrics

import (
	"context"
	"log/slog"
	"time"
)

// QueueDepthSource is the port the queue scanner polls for the current
// pending-reply backlog, keyed by reply kind. Defined here (the
// consumer) per dependency-inversion; the inbox repository satisfies it
// via an adapter at the composition root so this package never imports
// inbox.
type QueueDepthSource interface {
	QueueDepths(ctx context.Context) (map[string]int, error)
}

// SetPendingReplyDepth publishes the current per-kind queue depth. It
// Resets the gauge first so a kind that drained to zero stops being
// reported at its stale last value (the source only returns kinds with
// rows, so a disappeared kind would otherwise linger).
func (m *Metrics) SetPendingReplyDepth(byKind map[string]int) {
	m.queueDepth.Reset()
	for kind, depth := range byKind {
		m.queueDepth.WithLabelValues(kind).Set(float64(depth))
	}
}

// StartQueueScanner polls source every interval and republishes the
// pending-reply queue depth, until ctx is cancelled. Runs once on entry
// so a freshly started server reflects the backlog without waiting a
// full interval. A source error is logged and the last gauge value is
// left in place (better stale than zeroed-on-blip). Blocking; run in a
// goroutine. A nil logger falls back to slog.Default.
func (m *Metrics) StartQueueScanner(ctx context.Context, source QueueDepthSource, interval time.Duration, logger ...*slog.Logger) {
	log := slog.Default()
	if len(logger) > 0 && logger[0] != nil {
		log = logger[0]
	}
	scan := func() {
		depths, err := source.QueueDepths(ctx)
		if err != nil {
			log.WarnContext(ctx, "metrics: queue-depth scan failed", "err", err)
			return
		}
		m.SetPendingReplyDepth(depths)
	}

	scan()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			scan()
		}
	}
}
