package prospects

import (
	"context"
	"fmt"
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

func (m *mockRepo) ListProspects(_ context.Context, userID uuid.UUID) ([]domain.ProspectWithSource, error) {
	var result []domain.ProspectWithSource
	for _, p := range m.prospects {
		if p.UserID == userID {
			result = append(result, domain.ProspectWithSource{Prospect: *p})
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

func (m *mockRepo) FindByEmail(_ context.Context, userID uuid.UUID, email string) (*domain.Prospect, error) {
	for _, p := range m.prospects {
		if p.UserID == userID && p.Email == email {
			return p, nil
		}
	}
	return nil, nil
}

func (m *mockRepo) GetProspectForUser(_ context.Context, userID, prospectID uuid.UUID) (*domain.Prospect, error) {
	p, ok := m.prospects[prospectID]
	if !ok || p.UserID != userID {
		return nil, nil
	}
	return p, nil
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

// --- Error-producing mock repo ---

type mockErrorRepo struct {
	mockRepo
	listErr   error
	getErr    error
	createErr error
	deleteErr error
	findErr   error
	batchErr  error
}

func (m *mockErrorRepo) ListProspects(_ context.Context, _ uuid.UUID) ([]domain.ProspectWithSource, error) {
	if m.listErr != nil {
		return nil, m.listErr
	}
	return m.mockRepo.ListProspects(context.Background(), uuid.Nil)
}

func (m *mockErrorRepo) GetProspect(_ context.Context, _ uuid.UUID) (*domain.Prospect, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	return nil, nil
}

func (m *mockErrorRepo) CreateProspect(_ context.Context, p *domain.Prospect) error {
	if m.createErr != nil {
		return m.createErr
	}
	return nil
}

func (m *mockErrorRepo) DeleteProspect(_ context.Context, _ uuid.UUID) error {
	if m.deleteErr != nil {
		return m.deleteErr
	}
	return nil
}

func (m *mockErrorRepo) FindByEmail(_ context.Context, _ uuid.UUID, _ string) (*domain.Prospect, error) {
	if m.findErr != nil {
		return nil, m.findErr
	}
	return nil, nil
}

func (m *mockErrorRepo) CreateProspectsBatch(_ context.Context, _ []domain.Prospect) error {
	if m.batchErr != nil {
		return m.batchErr
	}
	return nil
}

// --- Error-producing lead checker ---

type mockErrorLeadChecker struct {
	err error
}

func (m *mockErrorLeadChecker) LeadExistsByEmail(_ context.Context, _ uuid.UUID, _ string) (bool, error) {
	return false, m.err
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

	input := CreateProspectInput{UserID: uuid.New(), Name: "Alice", Company: "Acme", Title: "CEO", Email: "taken@example.com"}
	_, err := uc.CreateProspect(context.Background(), input)
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

func TestListProspects_Empty(t *testing.T) {
	repo := newMockRepo()
	uc := NewUseCase(repo)
	result, err := uc.ListProspects(context.Background(), uuid.New())
	require.NoError(t, err)
	assert.Empty(t, result)
}

func TestCreateProspect_HappyPath(t *testing.T) {
	repo := newMockRepo()
	uc := NewUseCase(repo, WithLeadChecker(&mockLeadChecker{}))

	input := CreateProspectInput{UserID: uuid.New(), Name: "Alice", Company: "Acme", Title: "CEO", Email: "alice@acme.com"}
	p, err := uc.CreateProspect(context.Background(), input)
	require.NoError(t, err)
	assert.Contains(t, repo.prospects, p.ID)
}

func TestCreateProspect_EmptyName(t *testing.T) {
	repo := newMockRepo()
	uc := NewUseCase(repo)

	input := CreateProspectInput{UserID: uuid.New(), Name: "", Company: "Acme", Title: "CEO", Email: "a@b.com"}
	_, err := uc.CreateProspect(context.Background(), input)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "prospect name is required")
}

func TestCreateProspect_DedupByEmail(t *testing.T) {
	repo := newMockRepo()
	userID := uuid.New()
	uc := NewUseCase(repo, WithLeadChecker(&mockLeadChecker{}))

	input1 := CreateProspectInput{UserID: userID, Name: "Alice", Company: "Acme", Title: "CEO", Email: "dup@acme.com"}
	_, err := uc.CreateProspect(context.Background(), input1)
	require.NoError(t, err)

	input2 := CreateProspectInput{UserID: userID, Name: "Bob", Company: "Beta", Title: "CTO", Email: "dup@acme.com"}
	_, err = uc.CreateProspect(context.Background(), input2)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "проспект с таким email уже существует")
}

func TestCreateProspect_NoEmailSkipsDedup(t *testing.T) {
	repo := newMockRepo()
	uc := NewUseCase(repo, WithLeadChecker(&mockLeadChecker{}))

	input := CreateProspectInput{UserID: uuid.New(), Name: "Alice", Company: "Acme", Title: "CEO", Email: ""}
	p, err := uc.CreateProspect(context.Background(), input)
	require.NoError(t, err)
	assert.Contains(t, repo.prospects, p.ID)
}

func TestCreateProspect_NoLeadChecker(t *testing.T) {
	repo := newMockRepo()
	uc := NewUseCase(repo) // no lead checker

	input := CreateProspectInput{UserID: uuid.New(), Name: "Alice", Company: "Acme", Title: "CEO", Email: "alice@acme.com"}
	_, err := uc.CreateProspect(context.Background(), input)
	require.NoError(t, err)
}

func TestGetProspect_Found(t *testing.T) {
	repo := newMockRepo()
	id := uuid.New()
	repo.prospects[id] = &domain.Prospect{ID: id, Name: "Alice"}

	uc := NewUseCase(repo)
	p, err := uc.GetProspect(context.Background(), id)
	require.NoError(t, err)
	require.NotNil(t, p)
	assert.Equal(t, "Alice", p.Name)
}

func TestGetProspect_NotFound(t *testing.T) {
	repo := newMockRepo()
	uc := NewUseCase(repo)
	p, err := uc.GetProspect(context.Background(), uuid.New())
	require.NoError(t, err)
	assert.Nil(t, p)
}

func TestDeleteProspect(t *testing.T) {
	repo := newMockRepo()
	id := uuid.New()
	repo.prospects[id] = &domain.Prospect{ID: id, Name: "Alice"}

	uc := NewUseCase(repo)
	err := uc.DeleteProspect(context.Background(), id)
	require.NoError(t, err)
	assert.NotContains(t, repo.prospects, id)
}

func TestFindByEmail_Found(t *testing.T) {
	repo := newMockRepo()
	userID := uuid.New()
	id := uuid.New()
	repo.prospects[id] = &domain.Prospect{ID: id, UserID: userID, Email: "alice@acme.com"}

	uc := NewUseCase(repo)
	p, err := uc.FindByEmail(context.Background(), userID, "alice@acme.com")
	require.NoError(t, err)
	require.NotNil(t, p)
	assert.Equal(t, "alice@acme.com", p.Email)
}

func TestFindByEmail_NotFound(t *testing.T) {
	repo := newMockRepo()
	uc := NewUseCase(repo)
	p, err := uc.FindByEmail(context.Background(), uuid.New(), "nope@nope.com")
	require.NoError(t, err)
	assert.Nil(t, p)
}

func TestCreateProspect_FindByEmailError(t *testing.T) {
	repo := &mockErrorRepo{findErr: fmt.Errorf("db down")}
	uc := NewUseCase(repo)
	input := CreateProspectInput{UserID: uuid.New(), Name: "Alice", Company: "Acme", Title: "CEO", Email: "a@b.com"}
	_, err := uc.CreateProspect(context.Background(), input)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "prospect dedup")
}

func TestCreateProspect_LeadCheckerError(t *testing.T) {
	repo := newMockRepo()
	lc := &mockErrorLeadChecker{err: fmt.Errorf("lead svc down")}
	uc := NewUseCase(repo, WithLeadChecker(lc))
	input := CreateProspectInput{UserID: uuid.New(), Name: "Alice", Company: "Acme", Title: "CEO", Email: "a@b.com"}
	_, err := uc.CreateProspect(context.Background(), input)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "lead check")
}

func TestImportCSV_DedupSkipsExistingProspect(t *testing.T) {
	repo := newMockRepo()
	userID := uuid.New()
	// Pre-seed prospect with same email
	existing, _ := domain.NewProspect(userID, "Existing", "Co", "CTO", "dup@test.com", "manual")
	repo.prospects[existing.ID] = existing

	uc := NewUseCase(repo, WithLeadChecker(&mockLeadChecker{}))
	csv := []byte("name,company,title,email\nAlice,Acme,CEO,dup@test.com\nBob,Beta,CTO,fresh@test.com\n")
	count, err := uc.ImportCSV(context.Background(), userID, csv)
	require.NoError(t, err)
	assert.Equal(t, 1, count) // only Bob imported
}

func TestImportCSV_DedupSkipsExistingLead(t *testing.T) {
	repo := newMockRepo()
	lc := &mockLeadChecker{existingEmails: map[string]bool{"lead@test.com": true}}
	uc := NewUseCase(repo, WithLeadChecker(lc))
	userID := uuid.New()

	csv := []byte("name,company,title,email\nAlice,Acme,CEO,lead@test.com\nBob,Beta,CTO,fresh@test.com\n")
	count, err := uc.ImportCSV(context.Background(), userID, csv)
	require.NoError(t, err)
	assert.Equal(t, 1, count) // only Bob
}

func TestImportCSV_FindByEmailError(t *testing.T) {
	repo := &mockErrorRepo{findErr: fmt.Errorf("db error")}
	uc := NewUseCase(repo)
	csv := []byte("name,company,title,email\nAlice,Acme,CEO,a@b.com\n")
	_, err := uc.ImportCSV(context.Background(), uuid.New(), csv)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "dedup prospect check")
}

func TestImportCSV_LeadCheckerError(t *testing.T) {
	repo := newMockRepo()
	lc := &mockErrorLeadChecker{err: fmt.Errorf("lead svc down")}
	uc := NewUseCase(repo, WithLeadChecker(lc))
	csv := []byte("name,company,title,email\nAlice,Acme,CEO,a@b.com\n")
	_, err := uc.ImportCSV(context.Background(), uuid.New(), csv)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "dedup lead check")
}

func TestImportCSV_BatchError(t *testing.T) {
	repo := &mockErrorRepo{batchErr: fmt.Errorf("batch fail")}
	uc := NewUseCase(repo)
	csv := []byte("name,company,title,email\nAlice,Acme,CEO,\n") // empty email to skip dedup
	_, err := uc.ImportCSV(context.Background(), uuid.New(), csv)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "batch fail")
}

func TestImportCSV_EmptyCSV(t *testing.T) {
	repo := newMockRepo()
	uc := NewUseCase(repo)
	_, err := uc.ImportCSV(context.Background(), uuid.New(), []byte(""))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "read csv header")
}

func TestImportCSV_MalformedRecord(t *testing.T) {
	repo := newMockRepo()
	uc := NewUseCase(repo)
	// Quoted field not closed → csv parse error
	csvData := []byte("name,company,title,email\n\"unclosed,Acme,CEO,a@b.com\n")
	_, err := uc.ImportCSV(context.Background(), uuid.New(), csvData)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "read csv record")
}

func TestExportCSV_ListError(t *testing.T) {
	repo := &mockErrorRepo{listErr: fmt.Errorf("db down")}
	uc := NewUseCase(repo)
	_, err := uc.ExportCSV(context.Background(), uuid.New())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "list prospects")
}
