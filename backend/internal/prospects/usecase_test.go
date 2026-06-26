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
	prospects          map[uuid.UUID]*domain.Prospect
	batched            []domain.Prospect
	updateConsentCalls int
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

func (m *mockRepo) FindByTelegramUsername(_ context.Context, userID uuid.UUID, username string) (*domain.Prospect, error) {
	for _, p := range m.prospects {
		if p.UserID == userID && p.TelegramUsername == username {
			return p, nil
		}
	}
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

func (m *mockRepo) UpdateConsent(_ context.Context, prospectID uuid.UUID, c domain.Consent) error {
	m.updateConsentCalls++
	if p, ok := m.prospects[prospectID]; ok {
		p.Consent = c
	}
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
	listErr    error
	getErr     error
	createErr  error
	deleteErr  error
	findErr    error
	findByTGErr error
	batchErr   error
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

func (m *mockErrorRepo) FindByTelegramUsername(_ context.Context, _ uuid.UUID, _ string) (*domain.Prospect, error) {
	if m.findByTGErr != nil {
		return nil, m.findByTGErr
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

	report, err := uc.ImportCSV(context.Background(), userID, csv)
	require.NoError(t, err)
	require.NotNil(t, report)
	assert.Equal(t, 2, report.Imported)
	assert.Empty(t, report.Skipped)
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

	report, err := uc.ImportCSV(context.Background(), uuid.New(), csv)
	assert.Error(t, err)
	assert.Nil(t, report)
	assert.Contains(t, err.Error(), "invalid csv header")
}

func TestImportCSV_WithOptionalColumns(t *testing.T) {
	repo := newMockRepo()
	uc := NewUseCase(repo, WithLeadChecker(&mockLeadChecker{}))
	userID := uuid.New()

	csv := []byte("name,company,title,email,phone,industry\nAlice,Acme,CEO,alice@acme.com,+7999,SaaS\n")

	report, err := uc.ImportCSV(context.Background(), userID, csv)
	require.NoError(t, err)
	assert.Equal(t, 1, report.Imported)
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
	report, err := uc.ImportCSV(context.Background(), userID, csv)
	require.NoError(t, err)
	assert.Equal(t, 1, report.Imported) // only Bob imported
}

func TestImportCSV_DedupSkipsExistingLead(t *testing.T) {
	repo := newMockRepo()
	lc := &mockLeadChecker{existingEmails: map[string]bool{"lead@test.com": true}}
	uc := NewUseCase(repo, WithLeadChecker(lc))
	userID := uuid.New()

	csv := []byte("name,company,title,email\nAlice,Acme,CEO,lead@test.com\nBob,Beta,CTO,fresh@test.com\n")
	report, err := uc.ImportCSV(context.Background(), userID, csv)
	require.NoError(t, err)
	assert.Equal(t, 1, report.Imported) // only Bob
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

func TestImportCSV_StripsBOM(t *testing.T) {
	repo := newMockRepo()
	uc := NewUseCase(repo, WithLeadChecker(&mockLeadChecker{}))
	userID := uuid.New()

	bom := []byte{0xEF, 0xBB, 0xBF}
	csvData := append(bom, []byte("name,company,title,email\nAlice,Acme,CEO,alice@acme.com\n")...)

	report, err := uc.ImportCSV(context.Background(), userID, csvData)
	require.NoError(t, err)
	assert.Equal(t, 1, report.Imported)
	assert.Equal(t, "Alice", repo.batched[0].Name)
}

func TestImportCSV_SemicolonDelimiter(t *testing.T) {
	repo := newMockRepo()
	uc := NewUseCase(repo, WithLeadChecker(&mockLeadChecker{}))
	userID := uuid.New()

	csvData := []byte("name;company;title;email\nAlice;Acme;CEO;alice@acme.com\n")

	report, err := uc.ImportCSV(context.Background(), userID, csvData)
	require.NoError(t, err)
	assert.Equal(t, 1, report.Imported)
	assert.Equal(t, "Alice", repo.batched[0].Name)
	assert.Equal(t, "Acme", repo.batched[0].Company)
}

func TestImportCSV_FlexibleColumnOrder(t *testing.T) {
	repo := newMockRepo()
	uc := NewUseCase(repo, WithLeadChecker(&mockLeadChecker{}))
	userID := uuid.New()

	csvData := []byte("email,name,company,title\nalice@acme.com,Alice,Acme,CEO\n")

	report, err := uc.ImportCSV(context.Background(), userID, csvData)
	require.NoError(t, err)
	assert.Equal(t, 1, report.Imported)
	assert.Equal(t, "Alice", repo.batched[0].Name)
	assert.Equal(t, "alice@acme.com", repo.batched[0].Email)
	assert.Equal(t, "Acme", repo.batched[0].Company)
	assert.Equal(t, "CEO", repo.batched[0].Title)
}

func TestImportCSV_RussianColumnAliases(t *testing.T) {
	repo := newMockRepo()
	uc := NewUseCase(repo, WithLeadChecker(&mockLeadChecker{}))
	userID := uuid.New()

	csvData := []byte("имя;компания;должность;почта;телефон;telegram\nАлиса;ООО Рога;CEO;alice@acme.com;+79991234567;alice_tg\n")

	report, err := uc.ImportCSV(context.Background(), userID, csvData)
	require.NoError(t, err)
	assert.Equal(t, 1, report.Imported)
	assert.Equal(t, "Алиса", repo.batched[0].Name)
	assert.Equal(t, "ООО Рога", repo.batched[0].Company)
	assert.Equal(t, "CEO", repo.batched[0].Title)
	assert.Equal(t, "alice@acme.com", repo.batched[0].Email)
	assert.Equal(t, "+79991234567", repo.batched[0].Phone)
	assert.Equal(t, "alice_tg", repo.batched[0].TelegramUsername)
}

func TestImportCSV_TGContactsAlias(t *testing.T) {
	repo := newMockRepo()
	uc := NewUseCase(repo, WithLeadChecker(&mockLeadChecker{}))
	userID := uuid.New()

	csvData := []byte("имя в tg;tg-контакты;телефон;комментарий\nДарья;@crmlab_assistant;;Интегратор\n")

	report, err := uc.ImportCSV(context.Background(), userID, csvData)
	require.NoError(t, err)
	assert.Equal(t, 1, report.Imported)
	assert.Equal(t, "Дарья", repo.batched[0].Name)
	assert.Equal(t, "crmlab_assistant", repo.batched[0].TelegramUsername)
	assert.Equal(t, "Интегратор", repo.batched[0].Context)
}

func TestImportCSV_OnlyNameRequired(t *testing.T) {
	repo := newMockRepo()
	uc := NewUseCase(repo, WithLeadChecker(&mockLeadChecker{}))
	userID := uuid.New()

	csvData := []byte("name\nAlice\nBob\n")

	report, err := uc.ImportCSV(context.Background(), userID, csvData)
	require.NoError(t, err)
	assert.Equal(t, 2, report.Imported)
	assert.Equal(t, "Alice", repo.batched[0].Name)
	assert.Equal(t, "Bob", repo.batched[1].Name)
}

func TestImportCSV_NormalizeTGUsername(t *testing.T) {
	repo := newMockRepo()
	uc := NewUseCase(repo, WithLeadChecker(&mockLeadChecker{}))
	userID := uuid.New()

	csvData := []byte("name,telegram_username\nAlice,@alice_bot\nBob,bob_user\n")

	report, err := uc.ImportCSV(context.Background(), userID, csvData)
	require.NoError(t, err)
	assert.Equal(t, 2, report.Imported)
	assert.Equal(t, "alice_bot", repo.batched[0].TelegramUsername)
	assert.Equal(t, "bob_user", repo.batched[1].TelegramUsername)
}

func TestImportCSV_DedupByTelegramUsername(t *testing.T) {
	repo := newMockRepo()
	userID := uuid.New()
	existing, _ := domain.NewProspect(userID, "Existing", "Co", "CTO", "", "manual")
	existing.TelegramUsername = "alice_tg"
	repo.prospects[existing.ID] = existing

	uc := NewUseCase(repo, WithLeadChecker(&mockLeadChecker{}))
	csvData := []byte("name,telegram_username\nAlice,@alice_tg\nBob,bob_tg\n")
	report, err := uc.ImportCSV(context.Background(), userID, csvData)
	require.NoError(t, err)
	assert.Equal(t, 1, report.Imported) // only Bob
	assert.Equal(t, "Bob", repo.batched[0].Name)
}

func TestImportCSV_FindByTGUsernameError(t *testing.T) {
	repo := &mockErrorRepo{findByTGErr: fmt.Errorf("db connection refused")}
	uc := NewUseCase(repo)
	csvData := []byte("name,telegram_username\nAlice,alice_tg\n")
	_, err := uc.ImportCSV(context.Background(), uuid.New(), csvData)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "dedup prospect tg check")
}

func TestImportCSV_CaseInsensitiveHeaders(t *testing.T) {
	repo := newMockRepo()
	uc := NewUseCase(repo, WithLeadChecker(&mockLeadChecker{}))
	userID := uuid.New()

	csvData := []byte("Name,Company,Title,Email\nAlice,Acme,CEO,alice@acme.com\n")

	report, err := uc.ImportCSV(context.Background(), userID, csvData)
	require.NoError(t, err)
	assert.Equal(t, 1, report.Imported)
	assert.Equal(t, "Alice", repo.batched[0].Name)
}

func TestImportCSV_NoNameColumn(t *testing.T) {
	repo := newMockRepo()
	uc := NewUseCase(repo)

	csvData := []byte("company,title,email\nAcme,CEO,a@b.com\n")

	_, err := uc.ImportCSV(context.Background(), uuid.New(), csvData)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "name")
}

func TestTemplateCSV(t *testing.T) {
	uc := NewUseCase(newMockRepo())
	data := uc.TemplateCSV()

	csv := string(data)
	assert.True(t, data[0] == 0xEF && data[1] == 0xBB && data[2] == 0xBF, "should have BOM")
	assert.Contains(t, csv, "name,company,title,email")
	assert.Contains(t, csv, "telegram_username")
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

func TestImportCSV_FallbackNameToCompany(t *testing.T) {
	repo := newMockRepo()
	uc := NewUseCase(repo, WithLeadChecker(&mockLeadChecker{}))
	userID := uuid.New()

	csvData := []byte("name,company,email\n,Acme,info@acme.com\n")
	report, err := uc.ImportCSV(context.Background(), userID, csvData)
	require.NoError(t, err)
	require.NotNil(t, report)
	assert.Equal(t, 1, report.Imported)
	assert.Empty(t, report.Skipped)
	require.Len(t, repo.batched, 1)
	assert.Equal(t, "Acme", repo.batched[0].Name, "company should fill in for missing contact name")
	assert.Equal(t, "Acme", repo.batched[0].Company)
	assert.Equal(t, "info@acme.com", repo.batched[0].Email)
}

func TestImportCSV_FallbackNameToEmail(t *testing.T) {
	repo := newMockRepo()
	uc := NewUseCase(repo, WithLeadChecker(&mockLeadChecker{}))
	userID := uuid.New()

	csvData := []byte("name,company,email\n,,info@acme.com\n")
	report, err := uc.ImportCSV(context.Background(), userID, csvData)
	require.NoError(t, err)
	require.NotNil(t, report)
	assert.Equal(t, 1, report.Imported)
	assert.Empty(t, report.Skipped)
	require.Len(t, repo.batched, 1)
	assert.Equal(t, "info@acme.com", repo.batched[0].Name, "email should fill in for missing name and company")
	assert.Equal(t, "info@acme.com", repo.batched[0].Email)
}

func TestImportCSV_NoIdentifierSkippedWithReason(t *testing.T) {
	repo := newMockRepo()
	uc := NewUseCase(repo, WithLeadChecker(&mockLeadChecker{}))
	userID := uuid.New()

	// Row 2 (data line 1): valid; row 3 (data line 2): blank everything.
	csvData := []byte("name,company,email\nAlice,Acme,alice@acme.com\n,,\n")
	report, err := uc.ImportCSV(context.Background(), userID, csvData)
	require.NoError(t, err)
	require.NotNil(t, report)
	assert.Equal(t, 1, report.Imported)
	require.Len(t, report.Skipped, 1)
	assert.Equal(t, 3, report.Skipped[0].Line, "skipped row must reference its CSV line (1-indexed, including header)")
	assert.Contains(t, report.Skipped[0].Reason, "identifier")
}

func TestImportCSV_FallbackName_NormalizesEmailBeforeUsingAsName(t *testing.T) {
	// When email is the fallback identifier used as Name, the persisted Name
	// must already be normalized — otherwise dedup by name elsewhere diverges
	// from dedup by email.
	repo := newMockRepo()
	uc := NewUseCase(repo, WithLeadChecker(&mockLeadChecker{}))
	userID := uuid.New()

	csvData := []byte("name,email\n,  INFO@ACME.COM  \n")
	report, err := uc.ImportCSV(context.Background(), userID, csvData)
	require.NoError(t, err)
	assert.Equal(t, 1, report.Imported)
	require.Len(t, repo.batched, 1)
	assert.Equal(t, "info@acme.com", repo.batched[0].Email)
	assert.Equal(t, "info@acme.com", repo.batched[0].Name)
}

func TestImportCSV_NormalizesPhone(t *testing.T) {
	repo := newMockRepo()
	uc := NewUseCase(repo, WithLeadChecker(&mockLeadChecker{}))
	userID := uuid.New()

	csvData := []byte("name,phone\nAlice,+7 999 (123) 45-67\n")
	report, err := uc.ImportCSV(context.Background(), userID, csvData)
	require.NoError(t, err)
	assert.Equal(t, 1, report.Imported)
	assert.Equal(t, "+79991234567", repo.batched[0].Phone)
}

func TestExportCSV_ListError(t *testing.T) {
	repo := &mockErrorRepo{listErr: fmt.Errorf("db down")}
	uc := NewUseCase(repo)
	_, err := uc.ExportCSV(context.Background(), uuid.New())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "list prospects")
}

// TestImportCSV_ConsentColumn verifies the optional consent column: a truthy
// value grants obtained consent (source "import"); empty/falsy leaves the
// prospect at the cold default 'none'. Table-driven over the accepted forms.
func TestImportCSV_ConsentColumn(t *testing.T) {
	truthy := []string{"yes", "true", "1", "obtained", "да", "Y"}
	for _, v := range truthy {
		t.Run("truthy="+v, func(t *testing.T) {
			repo := newMockRepo()
			uc := NewUseCase(repo, WithLeadChecker(&mockLeadChecker{}))
			csv := []byte("name,email,consent\nAlice,alice@acme.com," + v + "\n")
			_, err := uc.ImportCSV(context.Background(), uuid.New(), csv)
			require.NoError(t, err)
			require.Len(t, repo.batched, 1)
			assert.Equal(t, domain.ConsentStatusObtained, repo.batched[0].Consent.Status)
			assert.Equal(t, "import", repo.batched[0].Consent.Source)
			assert.False(t, repo.batched[0].Consent.Timestamp.IsZero())
		})
	}

	falsy := []string{"", "no", "0", "false"}
	for _, v := range falsy {
		t.Run("falsy="+v, func(t *testing.T) {
			repo := newMockRepo()
			uc := NewUseCase(repo, WithLeadChecker(&mockLeadChecker{}))
			csv := []byte("name,email,consent\nBob,bob@beta.com," + v + "\n")
			_, err := uc.ImportCSV(context.Background(), uuid.New(), csv)
			require.NoError(t, err)
			require.Len(t, repo.batched, 1)
			assert.Equal(t, domain.ConsentStatusNone, repo.batched[0].Consent.Status)
		})
	}
}

// TestExportCSV_IncludesConsent verifies exported rows carry consent columns.
func TestExportCSV_IncludesConsent(t *testing.T) {
	repo := newMockRepo()
	uc := NewUseCase(repo)
	userID := uuid.New()
	p, err := domain.NewProspect(userID, "Carol", "Acme", "CEO", "carol@acme.com", "manual")
	require.NoError(t, err)
	require.NoError(t, p.GrantConsent("inbound_reply", time.Now().UTC()))
	require.NoError(t, repo.CreateProspect(context.Background(), p))

	out, err := uc.ExportCSV(context.Background(), userID)
	require.NoError(t, err)
	s := string(out)
	assert.Contains(t, s, "consent_status")
	assert.Contains(t, s, "consent_source")
	assert.Contains(t, s, "obtained")
	assert.Contains(t, s, "inbound_reply")
}

// TestSetConsent verifies the manual operator toggle: grant and withdraw
// transition the prospect and persist (source "manual"); unsupported statuses
// and unknown/foreign prospects are rejected.
func TestSetConsent(t *testing.T) {
	newOwned := func(repo *mockRepo, userID uuid.UUID) uuid.UUID {
		p, err := domain.NewProspect(userID, "Bob", "Acme", "CEO", "bob@acme.com", "manual")
		require.NoError(t, err)
		repo.prospects[p.ID] = p
		return p.ID
	}

	t.Run("grant", func(t *testing.T) {
		repo := newMockRepo()
		uc := NewUseCase(repo)
		userID := uuid.New()
		id := newOwned(repo, userID)
		require.NoError(t, uc.SetConsent(context.Background(), userID, id, domain.ConsentStatusObtained))
		assert.Equal(t, domain.ConsentStatusObtained, repo.prospects[id].Consent.Status)
		assert.Equal(t, "manual", repo.prospects[id].Consent.Source)
	})

	t.Run("withdraw", func(t *testing.T) {
		repo := newMockRepo()
		uc := NewUseCase(repo)
		userID := uuid.New()
		id := newOwned(repo, userID)
		require.NoError(t, uc.SetConsent(context.Background(), userID, id, domain.ConsentStatusWithdrawn))
		assert.Equal(t, domain.ConsentStatusWithdrawn, repo.prospects[id].Consent.Status)
	})

	// The Consent.Status assertions above pass even if SetConsent never
	// persists, because GrantConsent mutates the prospect pointer the mock
	// shares with the repo. Asserting the repository write actually happened
	// is what pins persistence — without it, a "if err != nil" → "if err == nil"
	// mutation (early return before repo.UpdateConsent) survives.
	t.Run("grant persists via repository", func(t *testing.T) {
		repo := newMockRepo()
		uc := NewUseCase(repo)
		userID := uuid.New()
		id := newOwned(repo, userID)
		require.NoError(t, uc.SetConsent(context.Background(), userID, id, domain.ConsentStatusObtained))
		assert.Equal(t, 1, repo.updateConsentCalls, "SetConsent must persist through repo.UpdateConsent exactly once")
	})

	t.Run("unsupported status (none) rejected", func(t *testing.T) {
		repo := newMockRepo()
		uc := NewUseCase(repo)
		userID := uuid.New()
		id := newOwned(repo, userID)
		require.Error(t, uc.SetConsent(context.Background(), userID, id, domain.ConsentStatusNone))
	})

	t.Run("foreign prospect → not found", func(t *testing.T) {
		repo := newMockRepo()
		uc := NewUseCase(repo)
		owner := uuid.New()
		id := newOwned(repo, owner)
		err := uc.SetConsent(context.Background(), uuid.New(), id, domain.ConsentStatusObtained)
		require.ErrorIs(t, err, ErrProspectNotFound)
	})
}
