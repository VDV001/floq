package leads

import (
	"context"
	"log/slog"

	"github.com/daniil/floq/internal/leads/domain"
	"github.com/google/uuid"
)

// PendingReplyCounter is the narrow port the inbox-list view needs from
// the HITL/inbox context: how many drafts are still in the 'pending'
// status awaiting operator action per lead, scoped to a user. Kept
// outside domain.Repository so the leads context does not depend on the
// inbox package — the composition root supplies an adapter.
type PendingReplyCounter interface {
	CountPendingByUser(ctx context.Context, userID uuid.UUID) (map[uuid.UUID]int, error)
}

// LeadWithPendingCount is the read-projection returned by
// ListLeadsWithPendingCounts: the lead list row plus its pending-
// reply badge count. PendingCount is zero when no rows match, when
// the counter port is not wired, or when the counter returned an
// error — the inbox list itself is too critical to 500 over a missing
// badge.
type LeadWithPendingCount struct {
	LeadWithSource domain.LeadWithSource
	PendingCount   int
}

// WithPendingReplyCounter wires the optional counter. Omit (or pass
// nil) to keep ListLeadsWithPendingCounts in zero-badge mode.
func WithPendingReplyCounter(c PendingReplyCounter) Option {
	return func(uc *UseCase) { uc.pendingCounter = c }
}

// ListLeadsWithPendingCounts returns the same lead list as ListLeads,
// each row augmented with the count of pending HITL replies tied to
// the lead. A nil counter or a counter error degrades to zero —
// callers see no badges but the list itself stays usable.
func (uc *UseCase) ListLeadsWithPendingCounts(ctx context.Context, userID uuid.UUID) ([]LeadWithPendingCount, error) {
	leads, err := uc.repo.ListLeads(ctx, userID)
	if err != nil {
		return nil, err
	}
	out := make([]LeadWithPendingCount, len(leads))
	if uc.pendingCounter == nil {
		for i := range leads {
			out[i] = LeadWithPendingCount{LeadWithSource: leads[i]}
		}
		return out, nil
	}
	counts, cerr := uc.pendingCounter.CountPendingByUser(ctx, userID)
	if cerr != nil {
		// Degrade — badges hidden, list still works.
		uc.logger.WarnContext(ctx, "pending-reply counter failed; rendering inbox without badges",
			slog.Any("err", cerr))
		for i := range leads {
			out[i] = LeadWithPendingCount{LeadWithSource: leads[i]}
		}
		return out, nil
	}
	for i := range leads {
		out[i] = LeadWithPendingCount{
			LeadWithSource: leads[i],
			PendingCount:   counts[leads[i].ID],
		}
	}
	return out, nil
}
