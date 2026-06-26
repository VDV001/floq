package enrichment

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/daniil/floq/internal/enrichment/domain"
	"github.com/google/uuid"
)

// Config tunes the enrichment usecase.
type Config struct {
	TTLSeconds  int // how long an enriched record stays fresh before refresh
	MaxAttempts int // give up after this many failed attempts
	BatchLimit  int // max rows claimed per ProcessPending tick
}

// UseCase orchestrates enqueueing and processing of company enrichments.
type UseCase struct {
	store     Store
	fetcher   PageFetcher
	extractor Extractor
	enricher  Enricher // optional Phase-3 registry step; nil = disabled
	limiter   RateLimiter
	cfg       Config
	logger    *slog.Logger
}

// Option customizes the usecase at construction.
type Option func(*UseCase)

// WithEnricher enables the optional Phase-3 (#188) registry step: after the
// page is extracted, the company's legal details are looked up and merged.
// A nil enricher leaves the step disabled (ship-dark default).
func WithEnricher(e Enricher) Option {
	return func(uc *UseCase) { uc.enricher = e }
}

// NewUseCase builds the enrichment usecase. A nil logger falls back to the
// default slog logger.
func NewUseCase(store Store, fetcher PageFetcher, extractor Extractor, limiter RateLimiter, cfg Config, logger *slog.Logger, opts ...Option) *UseCase {
	if logger == nil {
		logger = slog.Default()
	}
	uc := &UseCase{store: store, fetcher: fetcher, extractor: extractor, limiter: limiter, cfg: cfg, logger: logger}
	for _, opt := range opts {
		opt(uc)
	}
	return uc
}

// Enqueue derives the company domain from email and enqueues a pending
// enrichment. Emails with no company domain (free providers, malformed) are a
// silent no-op — not an error. Real persistence errors are returned so the
// caller can log them; callers treat enqueue as best-effort and never fail the
// lead/prospect create on it.
func (uc *UseCase) Enqueue(ctx context.Context, userID uuid.UUID, email string) error {
	d, err := domain.NewDomain(email)
	if errors.Is(err, domain.ErrFreeEmailProvider) || errors.Is(err, domain.ErrInvalidDomain) {
		return nil // expected: nothing to enrich
	}
	if err != nil {
		return err
	}
	e, err := domain.NewPendingEnrichment(userID, d)
	if err != nil {
		return err
	}
	return uc.store.UpsertPending(ctx, e)
}

// ProcessPending claims a batch of due records and (re)scrapes each under the
// per-domain rate limit. It returns the number successfully enriched. A single
// row's failure (fetch/extract error, or a panic) never aborts the batch.
func (uc *UseCase) ProcessPending(ctx context.Context) (int, error) {
	due, err := uc.store.ClaimDue(ctx, uc.cfg.BatchLimit, uc.cfg.MaxAttempts)
	if err != nil {
		return 0, fmt.Errorf("enrichment: claim due: %w", err)
	}
	enriched := 0
	for _, e := range due {
		if uc.processOne(ctx, e) {
			enriched++
		}
	}
	return enriched, nil
}

// processOne scrapes a single record. It returns true only on a successful
// enrich. Panics are recovered and recorded as a failure so one poison row
// cannot crash the worker loop.
func (uc *UseCase) processOne(ctx context.Context, e *domain.CompanyEnrichment) (ok bool) {
	dom := e.Domain.String()
	defer func() {
		if r := recover(); r != nil {
			uc.logger.ErrorContext(ctx, "enrichment: panic processing domain", "domain", dom, "panic", r)
			e.MarkFailed(fmt.Sprintf("panic: %v", r))
			_ = uc.store.Save(ctx, e)
			ok = false
		}
	}()

	allowed, _, err := uc.limiter.Allow(ctx, "enrichment:"+dom)
	if err != nil {
		uc.logger.WarnContext(ctx, "enrichment: rate-limiter error, skipping tick", "domain", dom, "err", err)
		return false
	}
	if !allowed {
		// Leave the row due; the next tick retries within budget.
		return false
	}

	page, err := uc.fetcher.Fetch(ctx, dom)
	if err != nil {
		e.MarkFailed(fmt.Sprintf("fetch: %v", err))
		uc.save(ctx, e)
		return false
	}
	// Attribute the extraction to the record's user so a Completer-backed
	// extractor (the Phase-2 LLM path) can cost-attribute its provider call in
	// the audit log. Harmless for the deterministic HTML extractor, which
	// ignores it.
	ctx = WithSubjectUser(ctx, e.UserID)
	profile, err := uc.extractor.Extract(ctx, page)
	if err != nil {
		e.MarkFailed(fmt.Sprintf("extract: %v", err))
		uc.save(ctx, e)
		return false
	}
	uc.enrichRegistry(ctx, page, &profile)
	e.MarkEnriched(profile, uc.cfg.TTLSeconds)
	uc.save(ctx, e)
	return true
}

// enrichRegistry runs the optional Phase-3 registry lookup and merges any legal
// details into profile. Best-effort: a nil enricher, a clean miss, or an error
// all leave the website profile intact (graceful degrade) — registry data must
// never cost us the cheaper scraped profile. The match strategy (precise INN
// vs fuzzy name, skip-on-ambiguity) lives in the Enricher adapter.
func (uc *UseCase) enrichRegistry(ctx context.Context, page string, profile *domain.CompanyProfile) {
	if uc.enricher == nil {
		return
	}
	q := EnrichQuery{INN: ExtractINN(page), CompanyName: profile.Title}
	legal, found, err := uc.enricher.Enrich(ctx, q)
	if err != nil {
		uc.logger.WarnContext(ctx, "enrichment: registry lookup failed; keeping website profile", "err", err)
		return
	}
	if found {
		profile.Legal = legal
	}
}

func (uc *UseCase) save(ctx context.Context, e *domain.CompanyEnrichment) {
	if err := uc.store.Save(ctx, e); err != nil {
		uc.logger.ErrorContext(ctx, "enrichment: save failed", "domain", e.Domain.String(), "err", err)
	}
}

// Get returns the enrichment for a user's company domain (tenant-scoped).
func (uc *UseCase) Get(ctx context.Context, userID uuid.UUID, domainName string) (*domain.CompanyEnrichment, bool, error) {
	return uc.store.Get(ctx, userID, domainName)
}
