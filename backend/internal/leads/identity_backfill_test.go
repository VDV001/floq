package leads

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/daniil/floq/internal/leads/domain"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stubBackfillSource struct {
	leads     []LeadIdentifierRow
	prospects []ProspectIdentifierRow
	leadsErr  error
	prospErr  error
}

func (s *stubBackfillSource) LeadsForBackfill(_ context.Context) ([]LeadIdentifierRow, error) {
	return s.leads, s.leadsErr
}

func (s *stubBackfillSource) ProspectsForBackfill(_ context.Context) ([]ProspectIdentifierRow, error) {
	return s.prospects, s.prospErr
}

func TestIdentityBackfill_Run_LinksLeadsAndProspects_ToSingleIdentity(t *testing.T) {
	// Four rows (2 leads + 2 prospects) all describing the same person via
	// the email channel — after Run, the backing repo must contain exactly
	// one Identity row and four link rows (idempotent on re-run).
	userID := uuid.New()
	leadA, leadB := uuid.New(), uuid.New()
	prospA, prospB := uuid.New(), uuid.New()

	source := &stubBackfillSource{
		leads: []LeadIdentifierRow{
			{LeadID: leadA, UserID: userID, Email: "alice@acme.com"},
			{LeadID: leadB, UserID: userID, Email: "alice@acme.com"},
		},
		prospects: []ProspectIdentifierRow{
			{ProspectID: prospA, UserID: userID, Email: "alice@acme.com"},
			{ProspectID: prospB, UserID: userID, Email: "alice@acme.com", Phone: "+79991234567"},
		},
	}

	repo := newInMemoryIdentityRepo()
	resolver := NewIdentityResolver(repo)

	backfill := NewIdentityBackfill(source, resolver, repo)
	require.NoError(t, backfill.Run(context.Background()))

	require.Len(t, repo.saved, 1, "all four rows must resolve to a single Identity")
	require.Len(t, repo.leadLinks, 2)
	require.Len(t, repo.prospectLinks, 2)
}

func TestIdentityBackfill_Run_SkipsRowsWithNoIdentifiers(t *testing.T) {
	userID := uuid.New()
	source := &stubBackfillSource{
		leads: []LeadIdentifierRow{
			{LeadID: uuid.New(), UserID: userID, Email: ""},
		},
		prospects: []ProspectIdentifierRow{
			{ProspectID: uuid.New(), UserID: userID},
		},
	}

	repo := newInMemoryIdentityRepo()
	resolver := NewIdentityResolver(repo)

	backfill := NewIdentityBackfill(source, resolver, repo)
	require.NoError(t, backfill.Run(context.Background()))

	assert.Empty(t, repo.saved, "rows with all-empty identifiers must not create identities")
	assert.Empty(t, repo.leadLinks)
	assert.Empty(t, repo.prospectLinks)
}

func TestIdentityBackfill_Run_HonoursContextCancellation(t *testing.T) {
	userID := uuid.New()
	source := &stubBackfillSource{
		leads: make([]LeadIdentifierRow, 100),
	}
	for i := range source.leads {
		source.leads[i] = LeadIdentifierRow{LeadID: uuid.New(), UserID: userID, Email: "alice@acme.com"}
	}

	repo := newInMemoryIdentityRepo()
	resolver := NewIdentityResolver(repo)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	backfill := NewIdentityBackfill(source, resolver, repo)
	err := backfill.Run(ctx)
	require.ErrorIs(t, err, context.Canceled, "Run must abort when parent ctx is cancelled")
}

func TestIdentityBackfill_Run_IsIdempotent(t *testing.T) {
	userID := uuid.New()
	leadID := uuid.New()
	source := &stubBackfillSource{
		leads: []LeadIdentifierRow{
			{LeadID: leadID, UserID: userID, Email: "alice@acme.com"},
		},
	}

	repo := newInMemoryIdentityRepo()
	resolver := NewIdentityResolver(repo)
	backfill := NewIdentityBackfill(source, resolver, repo)

	require.NoError(t, backfill.Run(context.Background()))
	require.NoError(t, backfill.Run(context.Background()))

	assert.Len(t, repo.saved, 1, "second Run must not create a duplicate Identity")
	assert.Len(t, repo.leadLinks, 1, "second Run must not duplicate the lead link")
}

func TestIdentityBackfill_Run_SourceError_BubblesUp(t *testing.T) {
	source := &stubBackfillSource{leadsErr: errors.New("db unavailable")}
	repo := newInMemoryIdentityRepo()
	resolver := NewIdentityResolver(repo)
	backfill := NewIdentityBackfill(source, resolver, repo)

	err := backfill.Run(context.Background())
	require.Error(t, err, "fetch-source failures must surface so operators can retry")
}

// stubFailingResolver is a domain.IdentityResolver that always errors.
type stubFailingResolver struct {
	mu       sync.Mutex
	attempts int
}

func (s *stubFailingResolver) Resolve(_ context.Context, _ uuid.UUID, _, _, _ string) (*domain.Identity, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.attempts++
	return nil, errors.New("resolver down")
}

func TestIdentityBackfill_Run_ResolverError_LoggedNotPropagated(t *testing.T) {
	userID := uuid.New()
	source := &stubBackfillSource{
		leads: []LeadIdentifierRow{
			{LeadID: uuid.New(), UserID: userID, Email: "a@b.com"},
			{LeadID: uuid.New(), UserID: userID, Email: "c@d.com"},
		},
	}
	resolver := &stubFailingResolver{}
	repo := newInMemoryIdentityRepo()

	backfill := NewIdentityBackfill(source, resolver, repo)
	err := backfill.Run(context.Background())
	require.NoError(t, err, "per-row resolver failures must be swallowed so backfill keeps walking")

	resolver.mu.Lock()
	defer resolver.mu.Unlock()
	assert.Equal(t, 2, resolver.attempts, "both rows must be tried even after the first fails")
}
