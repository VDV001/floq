package prospects

import (
	"context"
	"testing"

	leadsdomain "github.com/daniil/floq/internal/leads/domain"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockLeadsRepo is a minimal mock for leads domain.Repository
// used to test LeadCheckerAdapter.
type mockLeadsRepo struct {
	leads map[string]*leadsdomain.Lead // key = email
}

func (m *mockLeadsRepo) GetLeadByEmailAddress(_ context.Context, _ uuid.UUID, email string) (*leadsdomain.Lead, error) {
	if l, ok := m.leads[email]; ok {
		return l, nil
	}
	return nil, nil
}

// Stubs for the rest of the interface — not exercised by adapter tests.

func (m *mockLeadsRepo) ListLeads(context.Context, uuid.UUID) ([]leadsdomain.Lead, error) {
	return nil, nil
}
func (m *mockLeadsRepo) GetLead(context.Context, uuid.UUID) (*leadsdomain.Lead, error) {
	return nil, nil
}
func (m *mockLeadsRepo) CreateLead(context.Context, *leadsdomain.Lead) error { return nil }
func (m *mockLeadsRepo) UpdateFirstMessage(context.Context, uuid.UUID, string) error {
	return nil
}
func (m *mockLeadsRepo) UpdateLeadStatus(context.Context, uuid.UUID, leadsdomain.LeadStatus) error {
	return nil
}
func (m *mockLeadsRepo) GetLeadByTelegramChatID(context.Context, uuid.UUID, int64) (*leadsdomain.Lead, error) {
	return nil, nil
}
func (m *mockLeadsRepo) StaleLeadsWithoutReminder(context.Context, int) ([]leadsdomain.Lead, error) {
	return nil, nil
}
func (m *mockLeadsRepo) ListMessages(context.Context, uuid.UUID) ([]leadsdomain.Message, error) {
	return nil, nil
}
func (m *mockLeadsRepo) CreateMessage(context.Context, *leadsdomain.Message) error { return nil }
func (m *mockLeadsRepo) GetQualification(context.Context, uuid.UUID) (*leadsdomain.Qualification, error) {
	return nil, nil
}
func (m *mockLeadsRepo) UpsertQualification(context.Context, *leadsdomain.Qualification) error {
	return nil
}
func (m *mockLeadsRepo) GetLatestDraft(context.Context, uuid.UUID) (*leadsdomain.Draft, error) {
	return nil, nil
}
func (m *mockLeadsRepo) CreateDraft(context.Context, *leadsdomain.Draft) error { return nil }
func (m *mockLeadsRepo) CreateReminder(context.Context, uuid.UUID, string) error {
	return nil
}
func (m *mockLeadsRepo) CountMonthLeads(context.Context, uuid.UUID) (int, error) { return 0, nil }
func (m *mockLeadsRepo) CountTotalLeads(context.Context, uuid.UUID) (int, error) { return 0, nil }

// --- Tests ---

func TestLeadCheckerAdapter_Exists(t *testing.T) {
	repo := &mockLeadsRepo{leads: map[string]*leadsdomain.Lead{
		"alice@acme.com": {ID: uuid.New()},
	}}
	adapter := NewLeadCheckerAdapter(repo)

	exists, err := adapter.LeadExistsByEmail(context.Background(), uuid.New(), "alice@acme.com")
	require.NoError(t, err)
	assert.True(t, exists)
}

func TestLeadCheckerAdapter_NotExists(t *testing.T) {
	repo := &mockLeadsRepo{leads: map[string]*leadsdomain.Lead{}}
	adapter := NewLeadCheckerAdapter(repo)

	exists, err := adapter.LeadExistsByEmail(context.Background(), uuid.New(), "nobody@acme.com")
	require.NoError(t, err)
	assert.False(t, exists)
}

// mockErrorLeadsRepo always returns an error from GetLeadByEmailAddress.
type mockErrorLeadsRepo struct {
	mockLeadsRepo
}

func (m *mockErrorLeadsRepo) GetLeadByEmailAddress(_ context.Context, _ uuid.UUID, _ string) (*leadsdomain.Lead, error) {
	return nil, assert.AnError
}

func TestLeadCheckerAdapter_RepoError(t *testing.T) {
	repo := &mockErrorLeadsRepo{}
	adapter := NewLeadCheckerAdapter(repo)

	exists, err := adapter.LeadExistsByEmail(context.Background(), uuid.New(), "a@b.com")
	assert.Error(t, err)
	assert.False(t, exists)
}

func TestNewLeadCheckerAdapter(t *testing.T) {
	repo := &mockLeadsRepo{}
	adapter := NewLeadCheckerAdapter(repo)
	require.NotNil(t, adapter)
	assert.Equal(t, leadsdomain.Repository(repo), adapter.repo)
}
