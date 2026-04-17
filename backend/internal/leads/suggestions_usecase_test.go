package leads

import (
	"context"
	"errors"
	"testing"

	"github.com/daniil/floq/internal/leads/domain"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockSuggestionFinder is a test double for domain.ProspectSuggestionFinder.
type mockSuggestionFinder struct {
	findResult    []domain.ProspectSuggestion
	findErr       error
	findCalls     int
	findUserID    uuid.UUID
	findLeadID    uuid.UUID
	counts        map[uuid.UUID]int
	countsErr     error
	linkedUserID  uuid.UUID
	linkedLead    uuid.UUID
	linkedPros    uuid.UUID
	linkErr       error
	dismissedU    uuid.UUID
	dismissedL    uuid.UUID
	dismissedP    uuid.UUID
	dismissErr    error
	dismissCalls  int
}

func (m *mockSuggestionFinder) FindForLead(_ context.Context, userID, leadID uuid.UUID) ([]domain.ProspectSuggestion, error) {
	m.findCalls++
	m.findUserID = userID
	m.findLeadID = leadID
	return m.findResult, m.findErr
}

func (m *mockSuggestionFinder) CountsForUser(_ context.Context, _ uuid.UUID) (map[uuid.UUID]int, error) {
	return m.counts, m.countsErr
}

func (m *mockSuggestionFinder) LinkProspect(_ context.Context, userID, leadID, prospectID uuid.UUID) error {
	m.linkedUserID = userID
	m.linkedLead = leadID
	m.linkedPros = prospectID
	return m.linkErr
}

func (m *mockSuggestionFinder) DismissSuggestion(_ context.Context, userID, leadID, prospectID uuid.UUID) error {
	m.dismissCalls++
	m.dismissedU = userID
	m.dismissedL = leadID
	m.dismissedP = prospectID
	return m.dismissErr
}

func TestGetProspectSuggestions_NoFinder_ReturnsEmpty(t *testing.T) {
	uc := NewUseCase(newMockRepo(), nil, nil)
	suggestions, err := uc.GetProspectSuggestions(context.Background(), uuid.New(), uuid.New())
	require.NoError(t, err)
	assert.Empty(t, suggestions)
}

func TestGetProspectSuggestions_DelegatesWithUserAndLeadIDs(t *testing.T) {
	finder := &mockSuggestionFinder{
		findResult: []domain.ProspectSuggestion{
			{ProspectID: uuid.New(), Name: "Test", Confidence: domain.ConfidenceHigh},
		},
	}
	uc := NewUseCase(newMockRepo(), nil, nil, WithSuggestionFinder(finder))

	userID := uuid.New()
	leadID := uuid.New()
	got, err := uc.GetProspectSuggestions(context.Background(), userID, leadID)
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, domain.ConfidenceHigh, got[0].Confidence)
	assert.Equal(t, userID, finder.findUserID)
	assert.Equal(t, leadID, finder.findLeadID)
	assert.Equal(t, 1, finder.findCalls)
}

func TestGetProspectSuggestions_ErrorPropagates(t *testing.T) {
	finder := &mockSuggestionFinder{findErr: domain.ErrLeadNotFound}
	uc := NewUseCase(newMockRepo(), nil, nil, WithSuggestionFinder(finder))

	_, err := uc.GetProspectSuggestions(context.Background(), uuid.New(), uuid.New())
	require.ErrorIs(t, err, domain.ErrLeadNotFound)
}

func TestLinkProspectToLead_DelegatesWithUserID(t *testing.T) {
	finder := &mockSuggestionFinder{}
	uc := NewUseCase(newMockRepo(), nil, nil, WithSuggestionFinder(finder))

	userID, leadID, prospectID := uuid.New(), uuid.New(), uuid.New()
	require.NoError(t, uc.LinkProspectToLead(context.Background(), userID, leadID, prospectID))
	assert.Equal(t, userID, finder.linkedUserID)
	assert.Equal(t, leadID, finder.linkedLead)
	assert.Equal(t, prospectID, finder.linkedPros)
}

func TestLinkProspectToLead_ErrorPropagates(t *testing.T) {
	finder := &mockSuggestionFinder{linkErr: errors.New("link failed")}
	uc := NewUseCase(newMockRepo(), nil, nil, WithSuggestionFinder(finder))

	require.Error(t, uc.LinkProspectToLead(context.Background(), uuid.New(), uuid.New(), uuid.New()))
}

func TestLinkProspectToLead_NoFinder_Errors(t *testing.T) {
	uc := NewUseCase(newMockRepo(), nil, nil)
	require.Error(t, uc.LinkProspectToLead(context.Background(), uuid.New(), uuid.New(), uuid.New()))
}

func TestDismissProspectSuggestion_DelegatesWithUserID(t *testing.T) {
	finder := &mockSuggestionFinder{}
	uc := NewUseCase(newMockRepo(), nil, nil, WithSuggestionFinder(finder))

	userID, leadID, prospectID := uuid.New(), uuid.New(), uuid.New()
	require.NoError(t, uc.DismissProspectSuggestion(context.Background(), userID, leadID, prospectID))
	assert.Equal(t, userID, finder.dismissedU)
	assert.Equal(t, leadID, finder.dismissedL)
	assert.Equal(t, prospectID, finder.dismissedP)
}

func TestDismissProspectSuggestion_NoFinder_Errors(t *testing.T) {
	uc := NewUseCase(newMockRepo(), nil, nil)
	require.Error(t, uc.DismissProspectSuggestion(context.Background(), uuid.New(), uuid.New(), uuid.New()))
}

func TestSuggestionCounts_NoFinder_ReturnsEmptyMap(t *testing.T) {
	uc := NewUseCase(newMockRepo(), nil, nil)
	counts, err := uc.SuggestionCounts(context.Background(), uuid.New())
	require.NoError(t, err)
	assert.Empty(t, counts)
}

func TestSuggestionCounts_DelegatesToFinder(t *testing.T) {
	leadID := uuid.New()
	finder := &mockSuggestionFinder{counts: map[uuid.UUID]int{leadID: 3}}
	uc := NewUseCase(newMockRepo(), nil, nil, WithSuggestionFinder(finder))

	counts, err := uc.SuggestionCounts(context.Background(), uuid.New())
	require.NoError(t, err)
	assert.Equal(t, 3, counts[leadID])
}
