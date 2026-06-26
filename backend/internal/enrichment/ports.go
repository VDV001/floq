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

// Completer is the LLM seam for the Phase-2 LLMExtractor. Declared locally
// (DIP) so the enrichment context never imports internal/ai or internal/audit
// — the composition root injects an adapter over the audit-recording provider
// that tags the request type and bounds cost. systemPrompt frames the
// extraction; userPrompt carries the (untrusted, possibly capped) page.
type Completer interface {
	Complete(ctx context.Context, systemPrompt, userPrompt string) (string, error)
}

// EnrichQuery carries the identity signals the registry Enricher matches on:
// an INN extracted from the page (precise) and/or the scraped company name
// (fuzzy). Both may be set; the adapter prefers the INN when present.
type EnrichQuery struct {
	INN         string
	CompanyName string
}

// Enricher looks up a company's legal/registry details (ЕГРЮЛ via an official
// source) by identity. It is orthogonal to Extractor: Extractor works on the
// already-fetched page, Enricher itself reaches out to the registry. Declared
// locally (DIP) so the enrichment context stays free of the DaData/http
// adapter; the composition root injects it. found=false is a clean miss (no
// confident match) — never a guess.
type Enricher interface {
	Enrich(ctx context.Context, q EnrichQuery) (domain.LegalDetails, bool, error)
}

// RateLimiter throttles outbound scrapes per domain. Declared locally (DIP) so
// the usecase does not import the ratelimit package; ratelimit.Limiter
// satisfies it structurally and is injected from the composition root.
type RateLimiter interface {
	Allow(ctx context.Context, key string) (allowed bool, retryAfter time.Duration, err error)
}
