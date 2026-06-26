package enrichment

import (
	"context"
	"time"

	"github.com/daniil/floq/internal/enrichment/domain"
	"github.com/google/uuid"
)

// Store persists company enrichment records and doubles as the work queue.
// Declared here in the consumer (usecase) per DIP; the pgx implementation is
// *Repository in this package, injected from the composition root.
type Store interface {
	// UpsertPending inserts a pending enrichment for (user, domain). It is a
	// no-op if a row already exists (dedup by the unique key) — the worker
	// refreshes existing rows via their TTL.
	UpsertPending(ctx context.Context, e *domain.CompanyEnrichment) error
	// ClaimDue returns up to limit records that are due for processing:
	// pending, or enriched-and-expired, with attempts below maxAttempts.
	ClaimDue(ctx context.Context, limit, maxAttempts int) ([]*domain.CompanyEnrichment, error)
	// Save persists the result of processing a record (status/profile/error/
	// attempts/timestamps).
	Save(ctx context.Context, e *domain.CompanyEnrichment) error
	// Get returns the enrichment for (user, domain), tenant-scoped.
	Get(ctx context.Context, userID uuid.UUID, domainName string) (*domain.CompanyEnrichment, bool, error)
}

// PageFetcher fetches the raw page content for a company domain. The website
// adapter implements it over the shared proxy-aware HTTP client.
type PageFetcher interface {
	Fetch(ctx context.Context, domainName string) (page string, err error)
}

// Extractor turns raw page content into a CompanyProfile. Phase 1 ships the
// pure HTMLExtractor; a Phase-2 LLMExtractor can slot in behind this seam.
type Extractor interface {
	Extract(ctx context.Context, page string) (domain.CompanyProfile, error)
}

var _ Extractor = (*HTMLExtractor)(nil)

// RateLimiter throttles outbound scrapes per domain. Declared locally (DIP) so
// the usecase does not import the ratelimit package; ratelimit.Limiter
// satisfies it structurally and is injected from the composition root.
type RateLimiter interface {
	Allow(ctx context.Context, key string) (allowed bool, retryAfter time.Duration, err error)
}
