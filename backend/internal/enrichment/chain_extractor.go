package enrichment

import (
	"context"
	"log/slog"

	"github.com/daniil/floq/internal/enrichment/domain"
)

// ChainExtractor composes a deterministic base Extractor (the HTML regex
// extractor) with an optional, additive LLM Extractor, implementing the same
// Extractor port (open/closed: the usecase is untouched). The base always
// runs and owns the cheap contact fields; the LLM only overlays the Phase-2
// fields (industry/company size).
//
// The LLM is best-effort: its failure is logged and swallowed so a model
// outage never costs us the cheap, deterministic HTML profile. A nil llm
// (e.g. ENRICHMENT_LLM_ENABLED=false) makes ChainExtractor behave exactly like
// the bare base extractor.
type ChainExtractor struct {
	base   Extractor
	llm    Extractor
	logger *slog.Logger
}

// NewChainExtractor builds the chain. llm may be nil to disable LLM
// enrichment. A nil logger falls back to the default slog logger.
func NewChainExtractor(base, llm Extractor, logger *slog.Logger) *ChainExtractor {
	if logger == nil {
		logger = slog.Default()
	}
	return &ChainExtractor{base: base, llm: llm, logger: logger}
}

var _ Extractor = (*ChainExtractor)(nil)

// Extract runs the base extractor, then overlays the LLM's Phase-2 fields.
func (c *ChainExtractor) Extract(ctx context.Context, page string) (domain.CompanyProfile, error) {
	profile, err := c.base.Extract(ctx, page)
	if err != nil {
		return profile, err // base failure is a real extraction failure
	}
	if c.llm == nil {
		return profile, nil
	}
	enriched, err := c.llm.Extract(ctx, page)
	if err != nil {
		// Degrade gracefully: keep the deterministic HTML profile.
		c.logger.WarnContext(ctx, "enrichment: llm extractor failed; using html profile only", "err", err)
		return profile, nil
	}
	profile.Industry = enriched.Industry
	profile.CompanySize = enriched.CompanySize
	return profile, nil
}
