package domain

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// AuditRepository persists Entry rows in the audit_log table. The
// caller hands over already-validated aggregates (constructed via
// NewEntry); the implementation is a thin INSERT layer and does no
// further checking — every column-level constraint duplicates an
// invariant already enforced by the factory.
//
// Save accepts a slice so the async recorder can flush in batches via
// pgx.CopyFrom. An empty slice is a no-op.
type AuditRepository interface {
	Save(ctx context.Context, entries []*Entry) error
	// CostSummary aggregates audit_log rows for one user over a closed
	// time range [from, to). Returns zero-valued breakdown slices (not
	// nil) when nothing matches so JSON serialisation produces [] rather
	// than null.
	CostSummary(ctx context.Context, userID uuid.UUID, from, to time.Time) (*CostSummary, error)
}

// Recorder is the port the RecordingProvider decorator uses to hand
// finished call records off to background storage. Implementations are
// expected to be non-blocking: a stuck or saturated recorder must NOT
// stall the AI hot path. The recorder owns the policy for what to do
// when the buffer is full (drop with metric, log, etc.).
type Recorder interface {
	Record(ctx context.Context, entry *Entry)
}
