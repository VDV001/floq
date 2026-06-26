package enrichment_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/daniil/floq/internal/enrichment"
	"github.com/daniil/floq/internal/enrichment/domain"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- fakes ---

type fakeStore struct {
	upserted  []*domain.CompanyEnrichment
	due       []*domain.CompanyEnrichment
	saved     []*domain.CompanyEnrichment
	upsertErr error
	getResult *domain.CompanyEnrichment // returned by Get (found = non-nil)
}

func (f *fakeStore) UpsertPending(_ context.Context, e *domain.CompanyEnrichment) error {
	if f.upsertErr != nil {
		return f.upsertErr
	}
	f.upserted = append(f.upserted, e)
	return nil
}
func (f *fakeStore) ClaimDue(_ context.Context, _, _ int) ([]*domain.CompanyEnrichment, error) {
	return f.due, nil
}
func (f *fakeStore) Save(_ context.Context, e *domain.CompanyEnrichment) error {
	f.saved = append(f.saved, e)
	return nil
}
func (f *fakeStore) Get(context.Context, uuid.UUID, string) (*domain.CompanyEnrichment, bool, error) {
	return f.getResult, f.getResult != nil, nil
}

type fakeFetcher struct {
	page string
	err  error
}

func (f fakeFetcher) Fetch(context.Context, string) (string, error) { return f.page, f.err }

type fakeExtractor struct {
	profile domain.CompanyProfile
	err     error
}

func (f fakeExtractor) Extract(context.Context, string) (domain.CompanyProfile, error) {
	return f.profile, f.err
}

type fakeLimiter struct{ allow bool }

func (f fakeLimiter) Allow(context.Context, string) (bool, time.Duration, error) {
	return f.allow, 0, nil
}

func newUC(store enrichment.Store, fetcher enrichment.PageFetcher, ext enrichment.Extractor, lim enrichment.RateLimiter) *enrichment.UseCase {
	return enrichment.NewUseCase(store, fetcher, ext, lim,
		enrichment.Config{TTLSeconds: 3600, MaxAttempts: 3, BatchLimit: 50}, nil)
}

func duePending(t *testing.T, email string) *domain.CompanyEnrichment {
	t.Helper()
	d, err := domain.NewDomain(email)
	require.NoError(t, err)
	e, err := domain.NewPendingEnrichment(uuid.New(), d)
	require.NoError(t, err)
	return e
}

// --- Enqueue ---

func TestEnqueue_CorporateEmail_UpsertsPending(t *testing.T) {
	store := &fakeStore{}
	uc := newUC(store, fakeFetcher{}, fakeExtractor{}, fakeLimiter{allow: true})

	require.NoError(t, uc.Enqueue(context.Background(), uuid.New(), "ivan@acme.ru"))
	require.Len(t, store.upserted, 1)
	assert.Equal(t, "acme.ru", store.upserted[0].Domain.String())
	assert.Equal(t, domain.StatusPending, store.upserted[0].Status)
}

func TestEnqueue_FreeEmail_SilentlySkips(t *testing.T) {
	store := &fakeStore{}
	uc := newUC(store, fakeFetcher{}, fakeExtractor{}, fakeLimiter{allow: true})

	require.NoError(t, uc.Enqueue(context.Background(), uuid.New(), "ivan@gmail.com"))
	assert.Empty(t, store.upserted, "free-provider email is not enqueued")
}

func TestEnqueue_InvalidEmail_SilentlySkips(t *testing.T) {
	store := &fakeStore{}
	uc := newUC(store, fakeFetcher{}, fakeExtractor{}, fakeLimiter{allow: true})

	require.NoError(t, uc.Enqueue(context.Background(), uuid.New(), "garbage"))
	assert.Empty(t, store.upserted)
}

func TestEnqueue_DBError_IsReturned(t *testing.T) {
	store := &fakeStore{upsertErr: errors.New("db down")}
	uc := newUC(store, fakeFetcher{}, fakeExtractor{}, fakeLimiter{allow: true})

	err := uc.Enqueue(context.Background(), uuid.New(), "ivan@acme.ru")
	assert.Error(t, err, "real persistence errors surface to the caller (who logs best-effort)")
}

// --- ProcessPending ---

func TestProcessPending_Success_MarksEnriched(t *testing.T) {
	store := &fakeStore{due: []*domain.CompanyEnrichment{duePending(t, "ivan@acme.ru")}}
	profile := domain.CompanyProfile{Title: "Acme", Description: "Widgets"}
	uc := newUC(store, fakeFetcher{page: "<html>"}, fakeExtractor{profile: profile}, fakeLimiter{allow: true})

	n, err := uc.ProcessPending(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1, n)
	require.Len(t, store.saved, 1)
	assert.Equal(t, domain.StatusEnriched, store.saved[0].Status)
	assert.Equal(t, profile, store.saved[0].Profile)
}

func TestProcessPending_FetchError_MarksFailed(t *testing.T) {
	store := &fakeStore{due: []*domain.CompanyEnrichment{duePending(t, "ivan@acme.ru")}}
	uc := newUC(store, fakeFetcher{err: errors.New("timeout")}, fakeExtractor{}, fakeLimiter{allow: true})

	_, err := uc.ProcessPending(context.Background())
	require.NoError(t, err)
	require.Len(t, store.saved, 1)
	assert.Equal(t, domain.StatusFailed, store.saved[0].Status)
	assert.Equal(t, 1, store.saved[0].Attempts)
	assert.Contains(t, store.saved[0].Error, "timeout")
}

func TestProcessPending_ExtractError_MarksFailed(t *testing.T) {
	store := &fakeStore{due: []*domain.CompanyEnrichment{duePending(t, "ivan@acme.ru")}}
	uc := newUC(store, fakeFetcher{page: "<html>"}, fakeExtractor{err: errors.New("parse")}, fakeLimiter{allow: true})

	_, err := uc.ProcessPending(context.Background())
	require.NoError(t, err)
	require.Len(t, store.saved, 1)
	assert.Equal(t, domain.StatusFailed, store.saved[0].Status)
}

func TestProcessPending_RateLimited_SkipsRow(t *testing.T) {
	store := &fakeStore{due: []*domain.CompanyEnrichment{duePending(t, "ivan@acme.ru")}}
	uc := newUC(store, fakeFetcher{page: "<html>"}, fakeExtractor{}, fakeLimiter{allow: false})

	n, err := uc.ProcessPending(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 0, n, "rate-limited row is left for the next tick")
	assert.Empty(t, store.saved, "no scrape, no save when rate-limited")
}

func TestProcessPending_ContinuesAfterOneFailure(t *testing.T) {
	store := &fakeStore{due: []*domain.CompanyEnrichment{
		duePending(t, "ivan@first.ru"),
		duePending(t, "ivan@second.ru"),
	}}
	// fetcher fails for all; loop must still process both rows.
	uc := newUC(store, fakeFetcher{err: errors.New("boom")}, fakeExtractor{}, fakeLimiter{allow: true})

	_, err := uc.ProcessPending(context.Background())
	require.NoError(t, err)
	assert.Len(t, store.saved, 2, "one bad row does not abort the batch")
}
