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

// fakeEnricher records the query and returns a canned result.
type fakeEnricher struct {
	query  enrichment.EnrichQuery
	result domain.LegalDetails
	found  bool
	err    error
	called bool
}

func (f *fakeEnricher) Enrich(_ context.Context, q enrichment.EnrichQuery) (domain.LegalDetails, bool, error) {
	f.called = true
	f.query = q
	return f.result, f.found, f.err
}

func newUCWithEnricher(store enrichment.Store, fetcher enrichment.PageFetcher, ext enrichment.Extractor, lim enrichment.RateLimiter, enr enrichment.Enricher) *enrichment.UseCase {
	return enrichment.NewUseCase(store, fetcher, ext, lim,
		enrichment.Config{TTLSeconds: 3600, MaxAttempts: 3, BatchLimit: 50}, nil,
		enrichment.WithEnricher(enr))
}

func TestProcessPending_RegistryMerge_INNFromPage(t *testing.T) {
	rec := duePending(t, "ivan@acme.ru")
	store := &fakeStore{due: []*domain.CompanyEnrichment{rec}}
	page := "<html>Acme ИНН 7707083893</html>"
	legal := domain.LegalDetails{INN: "7707083893", OGRN: "1027700132195", Address: "Москва"}
	enr := &fakeEnricher{result: legal, found: true}
	uc := newUCWithEnricher(store, fakeFetcher{page: page}, fakeExtractor{profile: domain.CompanyProfile{Title: "Acme LLC"}}, fakeLimiter{allow: true}, enr)

	n, err := uc.ProcessPending(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1, n)
	require.True(t, enr.called)
	// Query carries both signals: INN extracted from the page + the title.
	assert.Equal(t, "7707083893", enr.query.INN)
	assert.Equal(t, "Acme LLC", enr.query.CompanyName)
	// Registry details merged into the saved profile.
	require.Len(t, store.saved, 1)
	assert.Equal(t, legal, store.saved[0].Profile.Legal)
}

func TestProcessPending_RegistryMiss_StillEnriched(t *testing.T) {
	rec := duePending(t, "ivan@acme.ru")
	store := &fakeStore{due: []*domain.CompanyEnrichment{rec}}
	enr := &fakeEnricher{found: false}
	uc := newUCWithEnricher(store, fakeFetcher{page: "<html>"}, fakeExtractor{profile: domain.CompanyProfile{Title: "Acme"}}, fakeLimiter{allow: true}, enr)

	n, err := uc.ProcessPending(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1, n)
	require.Len(t, store.saved, 1)
	assert.True(t, store.saved[0].Profile.Legal.IsEmpty(), "miss leaves legal empty")
	assert.Equal(t, domain.StatusEnriched, store.saved[0].Status)
}

func TestProcessPending_RegistryError_GracefulDegrade(t *testing.T) {
	rec := duePending(t, "ivan@acme.ru")
	store := &fakeStore{due: []*domain.CompanyEnrichment{rec}}
	enr := &fakeEnricher{err: errors.New("dadata down")}
	uc := newUCWithEnricher(store, fakeFetcher{page: "<html>"}, fakeExtractor{profile: domain.CompanyProfile{Title: "Acme"}}, fakeLimiter{allow: true}, enr)

	n, err := uc.ProcessPending(context.Background())
	require.NoError(t, err, "registry failure must not fail the enrichment")
	assert.Equal(t, 1, n)
	require.Len(t, store.saved, 1)
	assert.True(t, store.saved[0].Profile.Legal.IsEmpty())
	assert.Equal(t, domain.StatusEnriched, store.saved[0].Status, "website profile still saved")
}
