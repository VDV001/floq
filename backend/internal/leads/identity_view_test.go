package leads

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/daniil/floq/internal/leads/domain"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stubIdentityReader struct {
	mu              sync.Mutex
	byLead          map[uuid.UUID]*domain.Identity
	linkedByIdentity map[uuid.UUID][]uuid.UUID
	getErr          error
	linkedErr       error
}

func newStubIdentityReader() *stubIdentityReader {
	return &stubIdentityReader{
		byLead:           make(map[uuid.UUID]*domain.Identity),
		linkedByIdentity: make(map[uuid.UUID][]uuid.UUID),
	}
}

func (s *stubIdentityReader) GetByLeadID(_ context.Context, leadID uuid.UUID) (*domain.Identity, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.getErr != nil {
		return nil, s.getErr
	}
	return s.byLead[leadID], nil
}

func (s *stubIdentityReader) LinkedLeadIDs(_ context.Context, identityID uuid.UUID) ([]uuid.UUID, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.linkedErr != nil {
		return nil, s.linkedErr
	}
	return s.linkedByIdentity[identityID], nil
}

func TestUseCase_GetLeadView_NoIdentity_ReturnsLeadOnly(t *testing.T) {
	leadID := uuid.New()
	lead := &domain.Lead{ID: leadID, UserID: uuid.New(), Channel: domain.ChannelEmail, ContactName: "Alice", Status: domain.StatusNew}

	repo := newMockUCRepo()
	repo.byID[leadID] = lead
	identities := newStubIdentityReader()

	uc := NewUseCase(repo, nil, nil, WithIdentityReader(identities))
	view, err := uc.GetLeadView(context.Background(), lead.UserID, leadID)
	require.NoError(t, err)
	require.NotNil(t, view)
	assert.Equal(t, lead, view.Lead)
	assert.Nil(t, view.Identity, "no identity linked → Identity field must be nil")
	assert.Empty(t, view.LinkedLeadIDs)
}

func TestUseCase_GetLeadView_WithIdentity_ReturnsAggregate(t *testing.T) {
	userID := uuid.New()
	leadID := uuid.New()
	otherLead := uuid.New()
	identityID := uuid.New()

	lead := &domain.Lead{ID: leadID, UserID: userID, Channel: domain.ChannelEmail, ContactName: "Alice", Status: domain.StatusNew}
	identity := &domain.Identity{ID: identityID, UserID: userID, Email: "alice@acme.com", TelegramUsername: "alice_bot"}

	repo := newMockUCRepo()
	repo.byID[leadID] = lead
	repo.byID[otherLead] = &domain.Lead{ID: otherLead, UserID: userID, Channel: domain.ChannelTelegram, ContactName: "Alice", Status: domain.StatusNew}
	identities := newStubIdentityReader()
	identities.byLead[leadID] = identity
	identities.linkedByIdentity[identityID] = []uuid.UUID{leadID, otherLead}

	uc := NewUseCase(repo, nil, nil, WithIdentityReader(identities))
	view, err := uc.GetLeadView(context.Background(), lead.UserID, leadID)
	require.NoError(t, err)
	require.NotNil(t, view.Identity)
	assert.Equal(t, identityID, view.Identity.ID)
	assert.Equal(t, "alice@acme.com", view.Identity.Email)
	assert.ElementsMatch(t, []uuid.UUID{leadID, otherLead}, view.LinkedLeadIDs)
}

func TestUseCase_GetLeadView_NoReader_GracefullyDegrades(t *testing.T) {
	leadID := uuid.New()
	lead := &domain.Lead{ID: leadID, UserID: uuid.New(), Channel: domain.ChannelEmail, ContactName: "Alice", Status: domain.StatusNew}
	repo := newMockUCRepo()
	repo.byID[leadID] = lead

	uc := NewUseCase(repo, nil, nil) // no WithIdentityReader

	view, err := uc.GetLeadView(context.Background(), lead.UserID, leadID)
	require.NoError(t, err)
	require.NotNil(t, view)
	assert.Equal(t, lead, view.Lead)
	assert.Nil(t, view.Identity)
}

func TestUseCase_GetLeadView_LeadNotFound(t *testing.T) {
	repo := newMockUCRepo()
	uc := NewUseCase(repo, nil, nil, WithIdentityReader(newStubIdentityReader()))

	view, err := uc.GetLeadView(context.Background(), uuid.New(), uuid.New())
	require.NoError(t, err, "missing lead is a (nil, nil) contract, not an error")
	assert.Nil(t, view)
}

func TestUseCase_GetLeadView_IdentityFetchFails_FallsBackToLeadOnly(t *testing.T) {
	leadID := uuid.New()
	lead := &domain.Lead{ID: leadID, UserID: uuid.New(), Channel: domain.ChannelEmail, ContactName: "Alice", Status: domain.StatusNew}
	repo := newMockUCRepo()
	repo.byID[leadID] = lead

	identities := newStubIdentityReader()
	identities.getErr = errors.New("identity db down")

	uc := NewUseCase(repo, nil, nil, WithIdentityReader(identities))
	view, err := uc.GetLeadView(context.Background(), lead.UserID, leadID)
	require.NoError(t, err, "identity fetch failure must not block lead detail rendering")
	require.NotNil(t, view)
	assert.Equal(t, lead, view.Lead)
	assert.Nil(t, view.Identity, "fallback view exposes the lead without identity")
}

func TestUseCase_GetAggregatedMessages_NoReader_ReturnsLeadOnly(t *testing.T) {
	userID := uuid.New()
	leadID := uuid.New()
	repo := newMockUCRepo()
	repo.byID[leadID] = &domain.Lead{ID: leadID, UserID: userID}
	repo.messagesByLead = map[uuid.UUID][]domain.Message{
		leadID: {{ID: uuid.New(), LeadID: leadID, Body: "solo"}},
	}
	uc := NewUseCase(repo, nil, nil)

	msgs, err := uc.GetAggregatedMessages(context.Background(), userID, leadID)
	require.NoError(t, err)
	require.Len(t, msgs, 1)
	assert.Equal(t, "solo", msgs[0].Body)
}

func TestUseCase_GetAggregatedMessages_NoIdentity_FallsBackToLeadOnly(t *testing.T) {
	userID := uuid.New()
	leadID := uuid.New()
	repo := newMockUCRepo()
	repo.byID[leadID] = &domain.Lead{ID: leadID, UserID: userID}
	repo.messagesByLead = map[uuid.UUID][]domain.Message{
		leadID: {{ID: uuid.New(), LeadID: leadID, Body: "solo"}},
	}
	uc := NewUseCase(repo, nil, nil, WithIdentityReader(newStubIdentityReader()))

	msgs, err := uc.GetAggregatedMessages(context.Background(), userID, leadID)
	require.NoError(t, err)
	require.Len(t, msgs, 1)
}

func TestUseCase_GetAggregatedMessages_RejectsForeignTenant(t *testing.T) {
	ownerID := uuid.New()
	attackerID := uuid.New()
	leadID := uuid.New()
	repo := newMockUCRepo()
	repo.byID[leadID] = &domain.Lead{ID: leadID, UserID: ownerID}
	repo.messagesByLead = map[uuid.UUID][]domain.Message{
		leadID: {{ID: uuid.New(), LeadID: leadID, Body: "secret"}},
	}
	uc := NewUseCase(repo, nil, nil, WithIdentityReader(newStubIdentityReader()))

	msgs, err := uc.GetAggregatedMessages(context.Background(), attackerID, leadID)
	require.NoError(t, err)
	assert.Nil(t, msgs, "foreign tenant must get (nil, nil); handler maps to 404")
}

func TestUseCase_GetAggregatedMessages_MergesLeadsChronologically(t *testing.T) {
	userID := uuid.New()
	leadA, leadB := uuid.New(), uuid.New()
	identityID := uuid.New()
	t0 := time.Date(2026, 5, 16, 10, 0, 0, 0, time.UTC)

	msgA1 := domain.Message{ID: uuid.New(), LeadID: leadA, Body: "A-first", SentAt: t0}
	msgB1 := domain.Message{ID: uuid.New(), LeadID: leadB, Body: "B-second", SentAt: t0.Add(1 * time.Minute)}
	msgA2 := domain.Message{ID: uuid.New(), LeadID: leadA, Body: "A-third", SentAt: t0.Add(2 * time.Minute)}

	repo := newMockUCRepo()
	repo.byID[leadA] = &domain.Lead{ID: leadA, UserID: userID}
	repo.byID[leadB] = &domain.Lead{ID: leadB, UserID: userID}
	repo.messagesByLead = map[uuid.UUID][]domain.Message{
		leadA: {msgA1, msgA2}, // already-sorted within lead
		leadB: {msgB1},
	}
	identities := newStubIdentityReader()
	identities.byLead[leadA] = &domain.Identity{ID: identityID, UserID: userID, Email: "alice@acme.com"}
	identities.linkedByIdentity[identityID] = []uuid.UUID{leadA, leadB}

	uc := NewUseCase(repo, nil, nil, WithIdentityReader(identities))
	msgs, err := uc.GetAggregatedMessages(context.Background(), userID, leadA)
	require.NoError(t, err)
	require.Len(t, msgs, 3)
	assert.Equal(t, "A-first", msgs[0].Body)
	assert.Equal(t, "B-second", msgs[1].Body)
	assert.Equal(t, "A-third", msgs[2].Body)
}

func TestUseCase_GetAggregatedMessages_DropsForeignLinkedLead(t *testing.T) {
	// Defense-in-depth: even if a corrupted identity record links a
	// foreign tenant's lead, the merge filters it out — the attacker's
	// own lead drives the timeline, the foreign messages stay invisible.
	userID := uuid.New()
	foreignUser := uuid.New()
	leadA, foreignLead := uuid.New(), uuid.New()
	identityID := uuid.New()
	t0 := time.Date(2026, 5, 16, 10, 0, 0, 0, time.UTC)

	repo := newMockUCRepo()
	repo.byID[leadA] = &domain.Lead{ID: leadA, UserID: userID}
	repo.byID[foreignLead] = &domain.Lead{ID: foreignLead, UserID: foreignUser}
	repo.messagesByLead = map[uuid.UUID][]domain.Message{
		leadA:       {{ID: uuid.New(), LeadID: leadA, Body: "mine", SentAt: t0}},
		foreignLead: {{ID: uuid.New(), LeadID: foreignLead, Body: "leaked-secret", SentAt: t0}},
	}
	identities := newStubIdentityReader()
	identities.byLead[leadA] = &domain.Identity{ID: identityID, UserID: userID, Email: "alice@acme.com"}
	identities.linkedByIdentity[identityID] = []uuid.UUID{leadA, foreignLead}

	uc := NewUseCase(repo, nil, nil, WithIdentityReader(identities))
	msgs, err := uc.GetAggregatedMessages(context.Background(), userID, leadA)
	require.NoError(t, err)
	require.Len(t, msgs, 1)
	assert.Equal(t, "mine", msgs[0].Body)
}

func TestUseCase_GetAggregatedMessages_PartialLeadFailure_PreservesOthers(t *testing.T) {
	userID := uuid.New()
	leadA, leadB := uuid.New(), uuid.New()
	identityID := uuid.New()
	t0 := time.Date(2026, 5, 16, 10, 0, 0, 0, time.UTC)

	repo := newMockUCRepo()
	repo.byID[leadA] = &domain.Lead{ID: leadA, UserID: userID}
	repo.byID[leadB] = &domain.Lead{ID: leadB, UserID: userID}
	repo.messagesByLead = map[uuid.UUID][]domain.Message{
		leadA: {{ID: uuid.New(), LeadID: leadA, Body: "A-ok", SentAt: t0}},
	}
	repo.listMessagesErr = map[uuid.UUID]error{
		leadB: errors.New("transient pg blip"),
	}

	identities := newStubIdentityReader()
	identities.byLead[leadA] = &domain.Identity{ID: identityID, UserID: userID, Email: "alice@acme.com"}
	identities.linkedByIdentity[identityID] = []uuid.UUID{leadA, leadB}

	uc := NewUseCase(repo, nil, nil, WithIdentityReader(identities))
	msgs, err := uc.GetAggregatedMessages(context.Background(), userID, leadA)
	require.NoError(t, err, "one bad lead must not abort the entire timeline")
	require.Len(t, msgs, 1)
	assert.Equal(t, "A-ok", msgs[0].Body)
}

func TestUseCase_GetAggregatedMessages_IdentityErrorDegradesToLeadOnly(t *testing.T) {
	userID := uuid.New()
	leadID := uuid.New()
	repo := newMockUCRepo()
	repo.byID[leadID] = &domain.Lead{ID: leadID, UserID: userID}
	repo.messagesByLead = map[uuid.UUID][]domain.Message{
		leadID: {{ID: uuid.New(), LeadID: leadID, Body: "solo"}},
	}
	identities := newStubIdentityReader()
	identities.getErr = errors.New("identity db down")

	uc := NewUseCase(repo, nil, nil, WithIdentityReader(identities))
	msgs, err := uc.GetAggregatedMessages(context.Background(), userID, leadID)
	require.NoError(t, err)
	require.Len(t, msgs, 1)
}

// --- minimal UseCase repo mock for these tests (the existing one in
// usecase_test.go is a different package-internal type; redeclaring
// here keeps the file self-contained without forcing renames).

type mockUCRepo struct {
	mu              sync.Mutex
	byID            map[uuid.UUID]*domain.Lead
	messagesByLead  map[uuid.UUID][]domain.Message
	listMessagesErr map[uuid.UUID]error
}

func newMockUCRepo() *mockUCRepo {
	return &mockUCRepo{
		byID:           make(map[uuid.UUID]*domain.Lead),
		messagesByLead: make(map[uuid.UUID][]domain.Message),
	}
}

func (m *mockUCRepo) GetLead(_ context.Context, id uuid.UUID) (*domain.Lead, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.byID[id], nil
}

// The remaining domain.Repository methods are stubbed out — tests in
// this file only exercise GetLeadView, which uses GetLead. Each stub
// panics so accidental dependency creep surfaces immediately.

func (m *mockUCRepo) ListLeads(context.Context, uuid.UUID) ([]domain.LeadWithSource, error) {
	panic("not used")
}
func (m *mockUCRepo) ListAllLeads(context.Context, uuid.UUID) ([]domain.LeadWithSource, error) {
	panic("not used")
}
func (m *mockUCRepo) GetLeadForUser(_ context.Context, userID, leadID uuid.UUID) (*domain.Lead, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	l, ok := m.byID[leadID]
	if !ok || l.UserID != userID {
		return nil, nil
	}
	return l, nil
}
func (m *mockUCRepo) CreateLead(context.Context, *domain.Lead) error { panic("not used") }
func (m *mockUCRepo) UpdateFirstMessage(context.Context, uuid.UUID, string) error {
	panic("not used")
}
func (m *mockUCRepo) UpdateLeadStatus(context.Context, uuid.UUID, domain.LeadStatus) error {
	panic("not used")
}
func (m *mockUCRepo) SetLeadArchived(context.Context, uuid.UUID, *time.Time) error {
	panic("not used")
}
func (m *mockUCRepo) UpdateSourceID(context.Context, uuid.UUID, *uuid.UUID) error {
	panic("not used")
}
func (m *mockUCRepo) GetLeadByTelegramChatID(context.Context, uuid.UUID, int64) (*domain.Lead, error) {
	panic("not used")
}
func (m *mockUCRepo) GetLeadByEmailAddress(context.Context, uuid.UUID, string) (*domain.Lead, error) {
	panic("not used")
}
func (m *mockUCRepo) StaleLeadsWithoutReminder(context.Context, int) ([]domain.Lead, error) {
	panic("not used")
}
func (m *mockUCRepo) ListMessages(_ context.Context, leadID uuid.UUID) ([]domain.Message, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err, ok := m.listMessagesErr[leadID]; ok {
		return nil, err
	}
	return m.messagesByLead[leadID], nil
}
func (m *mockUCRepo) CreateMessage(context.Context, *domain.Message) error { panic("not used") }
func (m *mockUCRepo) GetQualification(context.Context, uuid.UUID) (*domain.Qualification, error) {
	panic("not used")
}
func (m *mockUCRepo) UpsertQualification(context.Context, *domain.Qualification) error {
	panic("not used")
}
func (m *mockUCRepo) GetLatestDraft(context.Context, uuid.UUID) (*domain.Draft, error) {
	panic("not used")
}
func (m *mockUCRepo) CreateDraft(context.Context, *domain.Draft) error { panic("not used") }
func (m *mockUCRepo) CreateReminder(context.Context, uuid.UUID, string) error {
	panic("not used")
}
func (m *mockUCRepo) CountMonthLeads(context.Context, uuid.UUID) (int, error) {
	panic("not used")
}
func (m *mockUCRepo) CountTotalLeads(context.Context, uuid.UUID) (int, error) {
	panic("not used")
}
