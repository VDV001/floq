package onec

import (
	"context"
	"errors"
	"log/slog"

	"github.com/daniil/floq/internal/integrations/onec/domain"
	"github.com/google/uuid"
)

// InboundProcessor applies one inbound 1C event idempotently. The inbound
// UseCase satisfies it; reconciliation reuses that exact path so a re-fed event
// dedups (already seen) or applies (webhook was missed) with no extra logic.
type InboundProcessor interface {
	ProcessInboundEvent(ctx context.Context, userID uuid.UUID, in RawInboundEvent) (ProcessResult, error)
}

// ReconcileStore lists tenants to reconcile and resolves their read credentials.
// The postgres Repository satisfies it.
type ReconcileStore interface {
	ActiveOnecUserIDs(ctx context.Context) ([]uuid.UUID, error)
	GetOutboundCredentials(ctx context.Context, userID uuid.UUID) (*domain.OutboundCredentials, error)
}

// ReconcileStats summarises one reconciliation pass for a user.
type ReconcileStats struct {
	Fetched int // events read back from 1C
	Applied int // events that were missing locally and got applied (recovered)
	Deduped int // events already in the ledger — no-op
	Failed  int // events that errored during processing (skipped, batch continues)
}

// ReconcileUseCase is the safety net for lost webhooks (#109): it periodically
// reads recent events back from 1C and re-feeds them through the inbound path,
// which idempotently applies anything missed and ignores anything already seen.
type ReconcileUseCase struct {
	store     ReconcileStore
	reader    OneCReader
	processor InboundProcessor
	logger    *slog.Logger
}

// NewReconcileUseCase wires the reconciliation use case. A nil logger falls back
// to the default.
func NewReconcileUseCase(store ReconcileStore, reader OneCReader, processor InboundProcessor, logger *slog.Logger) *ReconcileUseCase {
	if logger == nil {
		logger = slog.Default()
	}
	return &ReconcileUseCase{store: store, reader: reader, processor: processor, logger: logger}
}

// ReconcileAll reconciles every tenant with a usable 1C connection. A failure
// for one tenant is logged and does not stop the others.
func (u *ReconcileUseCase) ReconcileAll(ctx context.Context) error {
	ids, err := u.store.ActiveOnecUserIDs(ctx)
	if err != nil {
		return err
	}
	for _, id := range ids {
		stats, err := u.ReconcileUser(ctx, id)
		if err != nil {
			u.logger.Warn("onec: reconcile failed for user", "user_id", id, "err", err)
			continue
		}
		if stats.Applied > 0 || stats.Failed > 0 {
			u.logger.Info("onec: reconciled user",
				"user_id", id, "fetched", stats.Fetched, "applied", stats.Applied,
				"deduped", stats.Deduped, "failed", stats.Failed)
		}
	}
	return nil
}

// ReconcileUser reads a tenant's recent 1C events and re-feeds each through the
// inbound path. An unconfigured tenant is a silent no-op. A per-event failure is
// counted and skipped so one bad document never blocks the rest of the batch.
func (u *ReconcileUseCase) ReconcileUser(ctx context.Context, userID uuid.UUID) (ReconcileStats, error) {
	creds, err := u.store.GetOutboundCredentials(ctx, userID)
	if errors.Is(err, ErrOutboundNotConfigured) {
		return ReconcileStats{}, nil
	}
	if err != nil {
		return ReconcileStats{}, err
	}

	events, err := u.reader.ListEvents(ctx, creds)
	if err != nil {
		return ReconcileStats{}, err
	}

	stats := ReconcileStats{Fetched: len(events)}
	for _, ev := range events {
		// Stop promptly on shutdown rather than hammering a cancelled context and
		// inflating Failed with context.Canceled for every remaining event.
		if ctx.Err() != nil {
			return stats, ctx.Err()
		}
		res, err := u.processor.ProcessInboundEvent(ctx, userID, ev)
		switch {
		case err != nil:
			stats.Failed++
			u.logger.Warn("onec: reconcile event failed",
				"user_id", userID, "external_id", ev.ExternalID, "external_type", ev.ExternalType, "err", err)
		case res.Applied:
			stats.Applied++ // missed/unprocessed event recovered
		default:
			stats.Deduped++ // already processed (or no actionable rule) — no-op
		}
	}
	return stats, nil
}
