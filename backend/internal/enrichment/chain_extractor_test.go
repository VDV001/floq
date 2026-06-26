package enrichment_test

import (
	"context"
	"errors"
	"testing"

	"github.com/daniil/floq/internal/enrichment"
	"github.com/daniil/floq/internal/enrichment/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stubExtractor is a stub Extractor returning a canned profile/error and
// recording whether it was called.
type stubExtractor struct {
	profile domain.CompanyProfile
	err     error
	called  bool
}

func (f *stubExtractor) Extract(ctx context.Context, page string) (domain.CompanyProfile, error) {
	f.called = true
	return f.profile, f.err
}

func TestChainExtractor_MergesLLMFieldsOntoBase(t *testing.T) {
	base := &stubExtractor{profile: domain.CompanyProfile{
		Title:  "Acme LLC",
		Emails: []string{"info@acme.ru"},
	}}
	llm := &stubExtractor{profile: domain.CompanyProfile{
		Industry:    "fintech",
		CompanySize: domain.CompanySizeMedium,
	}}
	chain := enrichment.NewChainExtractor(base, llm, nil)

	got, err := chain.Extract(context.Background(), "<html>")
	require.NoError(t, err)
	// Base contacts preserved...
	assert.Equal(t, "Acme LLC", got.Title)
	assert.Equal(t, []string{"info@acme.ru"}, got.Emails)
	// ...and LLM fields merged in.
	assert.Equal(t, "fintech", got.Industry)
	assert.Equal(t, domain.CompanySizeMedium, got.CompanySize)
}

func TestChainExtractor_DegradesGracefullyOnLLMError(t *testing.T) {
	base := &stubExtractor{profile: domain.CompanyProfile{Title: "Acme LLC"}}
	llm := &stubExtractor{err: errors.New("llm timeout")}
	chain := enrichment.NewChainExtractor(base, llm, nil)

	got, err := chain.Extract(context.Background(), "<html>")
	require.NoError(t, err, "LLM failure must not fail the extraction")
	assert.Equal(t, "Acme LLC", got.Title, "the cheap HTML profile still comes through")
	assert.Empty(t, got.Industry)
	assert.Equal(t, domain.CompanySizeUnknown, got.CompanySize)
}

func TestChainExtractor_NilLLMReturnsBaseUnchanged(t *testing.T) {
	base := &stubExtractor{profile: domain.CompanyProfile{Title: "Acme LLC"}}
	chain := enrichment.NewChainExtractor(base, nil, nil)

	got, err := chain.Extract(context.Background(), "<html>")
	require.NoError(t, err)
	assert.Equal(t, "Acme LLC", got.Title)
	assert.Empty(t, got.Industry)
}

func TestChainExtractor_BaseErrorPropagatesAndSkipsLLM(t *testing.T) {
	sentinel := errors.New("fetch parse failed")
	base := &stubExtractor{err: sentinel}
	llm := &stubExtractor{}
	chain := enrichment.NewChainExtractor(base, llm, nil)

	_, err := chain.Extract(context.Background(), "<html>")
	assert.ErrorIs(t, err, sentinel)
	assert.False(t, llm.called, "LLM must not run when the base extractor fails")
}

func TestChainExtractor_SatisfiesExtractorPort(t *testing.T) {
	var _ enrichment.Extractor = enrichment.NewChainExtractor(&stubExtractor{}, nil, nil)
}
