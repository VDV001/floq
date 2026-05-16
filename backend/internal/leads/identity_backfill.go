package leads

import (
	"context"
	"log/slog"

	"github.com/daniil/floq/internal/leads/domain"
	"github.com/google/uuid"
)

// LeadIdentifierRow is the minimal projection of a leads row that the
// backfill needs: lead identity + the channel-native identifier the
// row currently carries.
type LeadIdentifierRow struct {
	LeadID uuid.UUID
	UserID uuid.UUID
	Email  string
}

// ProspectIdentifierRow is the prospect-side analogue. Prospects can
// carry up to three identifiers, so all three are passed through.
type ProspectIdentifierRow struct {
	ProspectID       uuid.UUID
	UserID           uuid.UUID
	Email            string
	Phone            string
	TelegramUsername string
}

// BackfillSource is the port the backfill uses to enumerate the legacy
// leads + prospects rows that need an Identity attached. The composition
// root provides a SQL implementation; tests stub it directly.
type BackfillSource interface {
	LeadsForBackfill(ctx context.Context) ([]LeadIdentifierRow, error)
	ProspectsForBackfill(ctx context.Context) ([]ProspectIdentifierRow, error)
}

// IdentityBackfill walks pre-existing leads + prospects, resolves each
// row to a unified Identity, and inserts the corresponding link table
// entry. It is safe to re-run — LinkLead / LinkProspect use ON CONFLICT
// DO NOTHING, and Resolve returns the same Identity for matching
// identifiers.
//
// Designed for one-shot invocation on server startup inside a goroutine
// whose ctx is the long-lived server context. ctx cancellation aborts
// the walk immediately.
type IdentityBackfill struct {
	source   BackfillSource
	resolver domain.IdentityResolver
	repo     domain.IdentityRepository
	logger   *slog.Logger
}

// NewIdentityBackfill wires the runner. The logger is optional; the
// default is slog.Default(), so callers that don't care about
// structured-log routing can pass nil through the option.
func NewIdentityBackfill(source BackfillSource, resolver domain.IdentityResolver, repo domain.IdentityRepository, opts ...IdentityBackfillOption) *IdentityBackfill {
	b := &IdentityBackfill{
		source:   source,
		resolver: resolver,
		repo:     repo,
		logger:   slog.Default(),
	}
	for _, opt := range opts {
		opt(b)
	}
	return b
}

// IdentityBackfillOption customises a *IdentityBackfill at construction.
type IdentityBackfillOption func(*IdentityBackfill)

// WithBackfillLogger overrides the default slog.Logger so the runner
// emits to the same handler as the rest of the server.
func WithBackfillLogger(l *slog.Logger) IdentityBackfillOption {
	return func(b *IdentityBackfill) {
		if l != nil {
			b.logger = l
		}
	}
}

// Run walks the source, resolves, and links. Fetch-source errors are
// returned (operators retry). Per-row resolve/link errors are logged
// and swallowed so a single bad row does not stop the walk.
//
// Returns ctx.Err() promptly on cancellation; partial progress is left
// committed (each row is its own micro-transaction at the repo level).
func (b *IdentityBackfill) Run(ctx context.Context) error {
	leads, err := b.source.LeadsForBackfill(ctx)
	if err != nil {
		return err
	}
	for _, l := range leads {
		if err := ctx.Err(); err != nil {
			return err
		}
		if l.Email == "" {
			continue
		}
		id, err := b.resolver.Resolve(ctx, l.UserID, l.Email, "", "")
		if err != nil {
			b.logger.WarnContext(ctx, "identity backfill: resolve lead failed",
				"lead", l.LeadID, "err", err)
			continue
		}
		if err := b.repo.LinkLead(ctx, l.LeadID, id.ID); err != nil {
			b.logger.WarnContext(ctx, "identity backfill: link lead failed",
				"lead", l.LeadID, "err", err)
		}
	}

	prospects, err := b.source.ProspectsForBackfill(ctx)
	if err != nil {
		return err
	}
	for _, p := range prospects {
		if err := ctx.Err(); err != nil {
			return err
		}
		if p.Email == "" && p.Phone == "" && p.TelegramUsername == "" {
			continue
		}
		id, err := b.resolver.Resolve(ctx, p.UserID, p.Email, p.Phone, p.TelegramUsername)
		if err != nil {
			b.logger.WarnContext(ctx, "identity backfill: resolve prospect failed",
				"prospect", p.ProspectID, "err", err)
			continue
		}
		if err := b.repo.LinkProspect(ctx, p.ProspectID, id.ID); err != nil {
			b.logger.WarnContext(ctx, "identity backfill: link prospect failed",
				"prospect", p.ProspectID, "err", err)
		}
	}
	return nil
}
