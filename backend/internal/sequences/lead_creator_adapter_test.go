package sequences

import (
	"context"
	"errors"
	"testing"

	leadsdomain "github.com/daniil/floq/internal/leads/domain"
	"github.com/daniil/floq/internal/sequences/domain"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Mock leads repository ---

type mockLeadsRepo struct {
	createdLead *leadsdomain.Lead
	createErr   error
}

func (m *mockLeadsRepo) ListLeads(_ context.Context, _ uuid.UUID) ([]leadsdomain.Lead, error) {
	return nil, nil
}
func (m *mockLeadsRepo) GetLead(_ context.Context, _ uuid.UUID) (*leadsdomain.Lead, error) {
	return nil, nil
}
func (m *mockLeadsRepo) CreateLead(_ context.Context, lead *leadsdomain.Lead) error {
	m.createdLead = lead
	return m.createErr
}
func (m *mockLeadsRepo) UpdateFirstMessage(_ context.Context, _ uuid.UUID, _ string) error {
	return nil
}
func (m *mockLeadsRepo) UpdateLeadStatus(_ context.Context, _ uuid.UUID, _ leadsdomain.LeadStatus) error {
	return nil
}
func (m *mockLeadsRepo) GetLeadByTelegramChatID(_ context.Context, _ uuid.UUID, _ int64) (*leadsdomain.Lead, error) {
	return nil, nil
}
func (m *mockLeadsRepo) GetLeadByEmailAddress(_ context.Context, _ uuid.UUID, _ string) (*leadsdomain.Lead, error) {
	return nil, nil
}
func (m *mockLeadsRepo) StaleLeadsWithoutReminder(_ context.Context, _ int) ([]leadsdomain.Lead, error) {
	return nil, nil
}
func (m *mockLeadsRepo) ListMessages(_ context.Context, _ uuid.UUID) ([]leadsdomain.Message, error) {
	return nil, nil
}
func (m *mockLeadsRepo) CreateMessage(_ context.Context, _ *leadsdomain.Message) error { return nil }
func (m *mockLeadsRepo) GetQualification(_ context.Context, _ uuid.UUID) (*leadsdomain.Qualification, error) {
	return nil, nil
}
func (m *mockLeadsRepo) UpsertQualification(_ context.Context, _ *leadsdomain.Qualification) error {
	return nil
}
func (m *mockLeadsRepo) GetLatestDraft(_ context.Context, _ uuid.UUID) (*leadsdomain.Draft, error) {
	return nil, nil
}
func (m *mockLeadsRepo) CreateDraft(_ context.Context, _ *leadsdomain.Draft) error { return nil }
func (m *mockLeadsRepo) CreateReminder(_ context.Context, _ uuid.UUID, _ string) error {
	return nil
}
func (m *mockLeadsRepo) CountMonthLeads(_ context.Context, _ uuid.UUID) (int, error) { return 0, nil }
func (m *mockLeadsRepo) CountTotalLeads(_ context.Context, _ uuid.UUID) (int, error) { return 0, nil }

// --- Tests ---

func TestLeadCreatorAdapter_CreateLeadFromProspect(t *testing.T) {
	repo := &mockLeadsRepo{}
	adapter := NewLeadCreatorAdapter(repo)

	userID := uuid.New()
	sourceID := uuid.New()
	prospect := &domain.ProspectView{
		ID:       uuid.New(),
		UserID:   userID,
		Name:     "Alice",
		Company:  "Acme",
		SourceID: &sourceID,
	}

	leadID, err := adapter.CreateLeadFromProspect(context.Background(), prospect, userID)
	require.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, leadID)

	require.NotNil(t, repo.createdLead)
	assert.Equal(t, userID, repo.createdLead.UserID)
	assert.Equal(t, "Alice", repo.createdLead.ContactName)
	assert.Equal(t, "Acme", repo.createdLead.Company)
	assert.Equal(t, &sourceID, repo.createdLead.SourceID)
}

func TestLeadCreatorAdapter_CreateLeadFromProspect_Error(t *testing.T) {
	repo := &mockLeadsRepo{createErr: errors.New("db error")}
	adapter := NewLeadCreatorAdapter(repo)

	prospect := &domain.ProspectView{
		ID:     uuid.New(),
		UserID: uuid.New(),
		Name:   "Alice",
	}

	leadID, err := adapter.CreateLeadFromProspect(context.Background(), prospect, prospect.UserID)
	require.Error(t, err)
	assert.Equal(t, uuid.Nil, leadID)
}

func TestNewLeadCreatorAdapter(t *testing.T) {
	repo := &mockLeadsRepo{}
	adapter := NewLeadCreatorAdapter(repo)
	assert.NotNil(t, adapter)
}
