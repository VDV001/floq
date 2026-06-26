package main

import (
	"context"

	"github.com/daniil/floq/internal/ai"
	"github.com/daniil/floq/internal/ai/security"
	auditdomain "github.com/daniil/floq/internal/audit/domain"
	"github.com/daniil/floq/internal/enrichment"
)

// enrichmentLLMAdapter bridges the audit-recording AI provider into the
// enrichment context's Completer port. It is composition-root glue: the
// enrichment context must not import internal/ai or internal/audit, so all of
// the cross-cutting concerns live here —
//
//   - cost: the untrusted page is capped (CostBreaker.CapInput) before it
//     reaches the model, the cheapest model mode is forced, and the output is
//     tightly token-capped (industry+size is a few tokens);
//   - audit: the call is tagged request_type=enrichment so the (already
//     audit-recording) provider attributes its cost to enrichment.
//
// Per-call flood control is left to the per-domain scrape rate limiter that
// already gates how often ProcessPending reaches the extractor.
type enrichmentLLMAdapter struct {
	provider  ai.Provider
	breaker   *security.CostBreaker
	maxTokens int
}

var _ enrichment.Completer = (*enrichmentLLMAdapter)(nil)

func newEnrichmentLLMAdapter(provider ai.Provider, maxInputRunes, maxTokens int) *enrichmentLLMAdapter {
	return &enrichmentLLMAdapter{
		provider: provider,
		// Input cap only (maxCallsPerKey=0 disables the call budget); the
		// per-domain scrape rate limit already bounds call frequency.
		breaker:   security.NewCostBreaker(maxInputRunes, 0, 0),
		maxTokens: maxTokens,
	}
}

func (a *enrichmentLLMAdapter) Complete(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	capped, _ := a.breaker.CapInput(userPrompt)
	// The enrichment worker runs off a root ctx with no audit CallMeta, so
	// re-tagging (WithRequestType) would be a no-op and the RecordingProvider
	// would DROP the row. Build a fresh CallMeta from the subject user the
	// usecase attached, so the call is cost-attributed under 'enrichment'. With
	// no subject user we attach nothing (a Nil-user meta is rejected by
	// NewEntry); the call still runs, just un-attributed.
	if uid, ok := enrichment.SubjectUserFromContext(ctx); ok {
		ctx = auditdomain.ContextWithCallMeta(ctx, auditdomain.CallMeta{
			UserID:      uid,
			RequestType: auditdomain.RequestTypeEnrichment,
		})
	}
	res, err := a.provider.Complete(ctx, ai.CompletionRequest{
		Messages: []ai.Message{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: capped},
		},
		MaxTokens: a.maxTokens,
		Mode:      ai.ModelModeBudget,
	})
	if err != nil {
		return "", err
	}
	return res.Text, nil
}
