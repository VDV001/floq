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
// deletes only the kinds that disappeared since the previous call (set
// difference) rather than Reset()-ing the whole vector — a global Reset
// opens a window in which a concurrent scrape sees an empty or partially
// repopulated gauge. Tracking the prior key set keeps every still-present
// series continuously valued across updates.
//
// prevKinds is guarded by mu; in practice only the single scanner
// goroutine calls this, but the lock keeps it safe if that ever changes.
func (m *Metrics) SetPendingReplyDepth(byKind map[string]int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for kind := range m.prevKinds {
		if _, still := byKind[kind]; !still {
			m.queueDepth.DeleteLabelValues(kind)
		}
	}
	next := make(map[string]struct{}, len(byKind))
	for kind, depth := range byKind {
		m.queueDepth.WithLabelValues(kind).Set(float64(depth))
		next[kind] = struct{}{}
	}
	m.prevKinds = next
}

// StartQueueScanner polls source every interval and republishes the
// pending-reply queue depth, until ctx is cancelled. Runs once on entry
// so a freshly started server reflects the backlog without waiting a
// full interval. A source error is logged and the last gauge value is
// left in place (better stale than zeroed-on-blip). Each scan is bounded
// by its own timeout so a hung query cannot stall the loop indefinitely.
// Blocking; run in a goroutine. A nil logger falls back to slog.Default.
func (m *Metrics) StartQueueScanner(ctx context.Context, source QueueDepthSource, interval time.Duration, logger *slog.Logger) {
	if logger == nil {
		logger = slog.Default()
	}
	scan := func() {
		scanCtx, cancel := context.WithTimeout(ctx, interval)
		defer cancel()
		depths, err := source.QueueDepths(scanCtx)
		if err != nil {
			logger.WarnContext(ctx, "metrics: queue-depth scan failed", "err", err)
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
