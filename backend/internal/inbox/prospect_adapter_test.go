package inbox

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	prospectsdomain "github.com/daniil/floq/internal/prospects/domain"
)

// --- Fake prospects domain repository ---

type fakeProspectsRepo struct {
	mu          sync.Mutex
	byEmail     map[string]*prospectsdomain.Prospect
	byTgUser    map[string]*prospectsdomain.Prospect
	converted      map[uuid.UUID]uuid.UUID // prospectID -> leadID
	findByEmailErr  error
	findByTgUserErr error
}

func newFakeProspectsRepo() *fakeProspectsRepo {
	return &fakeProspectsRepo{
		byEmail:   make(map[string]*prospectsdomain.Prospect),
		byTgUser:  make(map[string]*prospectsdomain.Prospect),
		converted: make(map[uuid.UUID]uuid.UUID),
	}
}

func (f *fakeProspectsRepo) ListProspects(_ context.Context, _ uuid.UUID) ([]prospectsdomain.Prospect, error) {
	return nil, nil
}

func (f *fakeProspectsRepo) GetProspect(_ context.Context, _ uuid.UUID) (*prospectsdomain.Prospect, error) {
	return nil, nil
}

func (f *fakeProspectsRepo) FindByEmail(_ context.Context, _ uuid.UUID, email string) (*prospectsdomain.Prospect, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.findByEmailErr != nil {
		return nil, f.findByEmailErr
	}
	return f.byEmail[email], nil
}

func (f *fakeProspectsRepo) FindByTelegramUsername(_ context.Context, _ uuid.UUID, username string) (*prospectsdomain.Prospect, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.findByTgUserErr != nil {
		return nil, f.findByTgUserErr
	}
	return f.byTgUser[username], nil
}

func (f *fakeProspectsRepo) CreateProspect(_ context.Context, _ *prospectsdomain.Prospect) error {
	return nil
}

func (f *fakeProspectsRepo) CreateProspectsBatch(_ context.Context, _ []prospectsdomain.Prospect) error {
	return nil
}

func (f *fakeProspectsRepo) DeleteProspect(_ context.Context, _ uuid.UUID) error {
	return nil
}

func (f *fakeProspectsRepo) UpdateStatus(_ context.Context, _ uuid.UUID, _ prospectsdomain.ProspectStatus) error {
	return nil
}

func (f *fakeProspectsRepo) ConvertToLead(_ context.Context, prospectID, leadID uuid.UUID) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.converted[prospectID] = leadID
	return nil
}

func (f *fakeProspectsRepo) UpdateVerification(_ context.Context, _ uuid.UUID, _ prospectsdomain.VerifyStatus, _ int, _ string, _ time.Time) error {
	return nil
}

// =============================================
// toMatch tests
// =============================================

func TestToMatch_Nil(t *testing.T) {
	result := toMatch(nil)
	assert.Nil(t, result)
}

func TestToMatch_FilledProspect(t *testing.T) {
	srcID := uuid.New()
	p := &prospectsdomain.Prospect{
		ID:      uuid.New(),
		Name:    "Test Prospect",
		Company: "TestCo",
		SourceID: &srcID,
		Status:  prospectsdomain.ProspectStatusNew,
	}

	result := toMatch(p)
	require.NotNil(t, result)
	assert.Equal(t, p.ID, result.ID)
	assert.Equal(t, "Test Prospect", result.Name)
	assert.Equal(t, "TestCo", result.Company)
	assert.Equal(t, &srcID, result.SourceID)
	assert.Equal(t, "new", result.Status)
}

func TestToMatch_NilSourceID(t *testing.T) {
	p := &prospectsdomain.Prospect{
		ID:     uuid.New(),
		Name:   "No Source",
		Status: prospectsdomain.ProspectStatusInSequence,
	}

	result := toMatch(p)
	require.NotNil(t, result)
	assert.Nil(t, result.SourceID)
	assert.Equal(t, "in_sequence", result.Status)
}

// =============================================
// ProspectRepoAdapter.FindByEmail tests
// =============================================

func TestProspectRepoAdapter_FindByEmail_Found(t *testing.T) {
	fake := newFakeProspectsRepo()
	srcID := uuid.New()
	prospect := &prospectsdomain.Prospect{
		ID:       uuid.New(),
		Name:     "Email Prospect",
		Company:  "EmailCo",
		SourceID: &srcID,
		Status:   prospectsdomain.ProspectStatusNew,
	}
	fake.byEmail["test@example.com"] = prospect

	adapter := NewProspectRepoAdapter(fake)
	userID := uuid.New()

	result, err := adapter.FindByEmail(context.Background(), userID, "test@example.com")
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, prospect.ID, result.ID)
	assert.Equal(t, "Email Prospect", result.Name)
	assert.Equal(t, "EmailCo", result.Company)
	assert.Equal(t, &srcID, result.SourceID)
}

func TestProspectRepoAdapter_FindByEmail_NotFound(t *testing.T) {
	fake := newFakeProspectsRepo()
	adapter := NewProspectRepoAdapter(fake)

	result, err := adapter.FindByEmail(context.Background(), uuid.New(), "unknown@example.com")
	require.NoError(t, err)
	assert.Nil(t, result)
}

func TestProspectRepoAdapter_FindByEmail_Error(t *testing.T) {
	fake := newFakeProspectsRepo()
	fake.findByEmailErr = errors.New("db connection failed")
	adapter := NewProspectRepoAdapter(fake)

	result, err := adapter.FindByEmail(context.Background(), uuid.New(), "test@example.com")
	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "db connection failed")
}

// =============================================
// ProspectRepoAdapter.FindByTelegramUsername tests
// =============================================

func TestProspectRepoAdapter_FindByTelegramUsername_Found(t *testing.T) {
	fake := newFakeProspectsRepo()
	prospect := &prospectsdomain.Prospect{
		ID:     uuid.New(),
		Name:   "TG Prospect",
		Status: prospectsdomain.ProspectStatusNew,
	}
	fake.byTgUser["tguser"] = prospect

	adapter := NewProspectRepoAdapter(fake)

	result, err := adapter.FindByTelegramUsername(context.Background(), uuid.New(), "tguser")
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, prospect.ID, result.ID)
	assert.Equal(t, "TG Prospect", result.Name)
}

func TestProspectRepoAdapter_FindByTelegramUsername_NotFound(t *testing.T) {
	fake := newFakeProspectsRepo()
	adapter := NewProspectRepoAdapter(fake)

	result, err := adapter.FindByTelegramUsername(context.Background(), uuid.New(), "nobody")
	require.NoError(t, err)
	assert.Nil(t, result)
}

func TestProspectRepoAdapter_FindByTelegramUsername_Error(t *testing.T) {
	fake := newFakeProspectsRepo()
	fake.findByTgUserErr = errors.New("tg lookup failed")
	adapter := NewProspectRepoAdapter(fake)

	result, err := adapter.FindByTelegramUsername(context.Background(), uuid.New(), "anyone")
	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "tg lookup failed")
}

// =============================================
// ProspectRepoAdapter.ConvertToLead tests
// =============================================

func TestProspectRepoAdapter_ConvertToLead(t *testing.T) {
	fake := newFakeProspectsRepo()
	adapter := NewProspectRepoAdapter(fake)

	prospectID := uuid.New()
	leadID := uuid.New()

	err := adapter.ConvertToLead(context.Background(), prospectID, leadID)
	require.NoError(t, err)

	fake.mu.Lock()
	defer fake.mu.Unlock()
	assert.Equal(t, leadID, fake.converted[prospectID])
}
