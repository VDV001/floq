package prospects

import (
	"context"
	"testing"
	"time"

	"github.com/daniil/floq/internal/prospects/domain"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Mock Repository ---

type mockRepo struct {
	prospects map[uuid.UUID]*domain.Prospect
	batched   []domain.Prospect
}

func newMockRepo() *mockRepo {
	return &mockRepo{
		prospects: make(map[uuid.UUID]*domain.Prospect),
	}
}

func (m *mockRepo) ListProspects(_ context.Context, userID uuid.UUID) ([]domain.Prospect, error) {
	var result []domain.Prospect
	for _, p := range m.prospects {
		if p.UserID == userID {
			result = append(result, *p)
		}
	}
	return result, nil
}

func (m *mockRepo) GetProspect(_ context.Context, id uuid.UUID) (*domain.Prospect, error) {
	p, ok := m.prospects[id]
	if !ok {
		return nil, nil
	}
	return p, nil
}

func (m *mockRepo) FindByEmail(_ context.Context, _ uuid.UUID, _ string) (*domain.Prospect, error) {
	return nil, nil
}

func (m *mockRepo) FindByTelegramUsername(_ context.Context, _ uuid.UUID, _ string) (*domain.Prospect, error) {
	return nil, nil
}

func (m *mockRepo) CreateProspect(_ context.Context, p *domain.Prospect) error {
	m.prospects[p.ID] = p
	return nil
}

func (m *mockRepo) CreateProspectsBatch(_ context.Context, prospects []domain.Prospect) error {
	m.batched = append(m.batched, prospects...)
	for i := range prospects {
		m.prospects[prospects[i].ID] = &prospects[i]
	}
	return nil
}

func (m *mockRepo) DeleteProspect(_ context.Context, id uuid.UUID) error {
	delete(m.prospects, id)
	return nil
}

func (m *mockRepo) UpdateStatus(_ context.Context, id uuid.UUID, status domain.ProspectStatus) error {
	if p, ok := m.prospects[id]; ok {
		p.Status = status
	}
	return nil
}

func (m *mockRepo) ConvertToLead(_ context.Context, _, _ uuid.UUID) error {
	return nil
}

func (m *mockRepo) UpdateVerification(_ context.Context, _ uuid.UUID, _ domain.VerifyStatus, _ int, _ string, _ time.Time) error {
	return nil
}

// --- Mock LeadChecker ---

type mockLeadChecker struct {
	existingEmails map[string]bool
}

func (m *mockLeadChecker) LeadExistsByEmail(_ context.Context, _ uuid.UUID, email string) (bool, error) {
	if m.existingEmails == nil {
		return false, nil
	}
	return m.existingEmails[email], nil
}

// --- Tests ---

func TestImportCSV_HappyPath(t *testing.T) {
	repo := newMockRepo()
	uc := NewUseCase(repo, WithLeadChecker(&mockLeadChecker{}))
	userID := uuid.New()

	csv := []byte("name,company,title,email\nAlice,Acme,CEO,alice@acme.com\nBob,Beta,CTO,bob@beta.com\n")

	count, err := uc.ImportCSV(context.Background(), userID, csv)
	require.NoError(t, err)
	assert.Equal(t, 2, count)
	assert.Len(t, repo.batched, 2)

	assert.Equal(t, "Alice", repo.batched[0].Name)
	assert.Equal(t, "Acme", repo.batched[0].Company)
	assert.Equal(t, "CEO", repo.batched[0].Title)
	assert.Equal(t, "alice@acme.com", repo.batched[0].Email)
	assert.Equal(t, domain.ProspectStatusNew, repo.batched[0].Status)
	assert.Equal(t, "csv", repo.batched[0].Source)
	assert.Equal(t, userID, repo.batched[0].UserID)

	assert.Equal(t, "Bob", repo.batched[1].Name)
}

func TestImportCSV_InvalidHeader(t *testing.T) {
	repo := newMockRepo()
	uc := NewUseCase(repo, WithLeadChecker(&mockLeadChecker{}))

	csv := []byte("first_name,company,title,email\nAlice,Acme,CEO,alice@acme.com\n")

	count, err := uc.ImportCSV(context.Background(), uuid.New(), csv)
	assert.Error(t, err)
	assert.Equal(t, 0, count)
	assert.Contains(t, err.Error(), "invalid csv header")
}

func TestImportCSV_WithOptionalColumns(t *testing.T) {
	repo := newMockRepo()
	uc := NewUseCase(repo, WithLeadChecker(&mockLeadChecker{}))
	userID := uuid.New()

	csv := []byte("name,company,title,email,phone,industry\nAlice,Acme,CEO,alice@acme.com,+7999,SaaS\n")

	count, err := uc.ImportCSV(context.Background(), userID, csv)
	require.NoError(t, err)
	assert.Equal(t, 1, count)
	assert.Equal(t, "+7999", repo.batched[0].Phone)
	assert.Equal(t, "SaaS", repo.batched[0].Industry)
}

func TestCreateProspect_RejectsExistingLead(t *testing.T) {
	repo := newMockRepo()
	lc := &mockLeadChecker{existingEmails: map[string]bool{"taken@example.com": true}}
	uc := NewUseCase(repo, WithLeadChecker(lc))

	p := domain.NewProspect(uuid.New(), "Alice", "Acme", "CEO", "taken@example.com", "manual")
	err := uc.CreateProspect(context.Background(), p)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "лид с таким email уже существует")
}

func TestExportCSV(t *testing.T) {
	repo := newMockRepo()
	userID := uuid.New()
	id := uuid.New()
	repo.prospects[id] = &domain.Prospect{
		ID:           id,
		UserID:       userID,
		Name:         "Alice",
		Company:      "Acme",
		Title:        "CEO",
		Email:        "alice@acme.com",
		Source:       "manual",
		Status:       domain.ProspectStatusNew,
		VerifyStatus: domain.VerifyStatusNotChecked,
	}

	uc := NewUseCase(repo)
	data, err := uc.ExportCSV(context.Background(), userID)
	require.NoError(t, err)

	csv := string(data)
	assert.Contains(t, csv, "name,company,title,email")
	assert.Contains(t, csv, "Alice,Acme,CEO,alice@acme.com")
}

func TestExportCSV_Empty(t *testing.T) {
	repo := newMockRepo()
	uc := NewUseCase(repo)

	data, err := uc.ExportCSV(context.Background(), uuid.New())
	require.NoError(t, err)
	assert.Contains(t, string(data), "name,company,title,email")
}

func TestListProspects(t *testing.T) {
	repo := newMockRepo()
	userID := uuid.New()
	id := uuid.New()
	repo.prospects[id] = &domain.Prospect{ID: id, UserID: userID, Name: "Test"}

	uc := NewUseCase(repo)
	result, err := uc.ListProspects(context.Background(), userID)
	require.NoError(t, err)
	require.Len(t, result, 1)
	assert.Equal(t, "Test", result[0].Name)
}
