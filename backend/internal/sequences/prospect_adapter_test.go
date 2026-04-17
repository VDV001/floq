package sequences

import (
	"context"
	"errors"
	"testing"
	"time"

	prospectsdomain "github.com/daniil/floq/internal/prospects/domain"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Mock prospects repository ---

type mockProspectsRepo struct {
	prospect  *prospectsdomain.Prospect
	getErr    error
	updateErr error
}

func (m *mockProspectsRepo) ListProspects(_ context.Context, _ uuid.UUID) ([]prospectsdomain.ProspectWithSource, error) {
	return nil, nil
}
func (m *mockProspectsRepo) GetProspect(_ context.Context, _ uuid.UUID) (*prospectsdomain.Prospect, error) {
	return m.prospect, m.getErr
}
func (m *mockProspectsRepo) GetProspectForUser(_ context.Context, _ uuid.UUID, _ uuid.UUID) (*prospectsdomain.Prospect, error) {
	return nil, nil
}
func (m *mockProspectsRepo) FindByEmail(_ context.Context, _ uuid.UUID, _ string) (*prospectsdomain.Prospect, error) {
	return nil, nil
}
func (m *mockProspectsRepo) FindByTelegramUsername(_ context.Context, _ uuid.UUID, _ string) (*prospectsdomain.Prospect, error) {
	return nil, nil
}
func (m *mockProspectsRepo) CreateProspect(_ context.Context, _ *prospectsdomain.Prospect) error {
	return nil
}
func (m *mockProspectsRepo) CreateProspectsBatch(_ context.Context, _ []prospectsdomain.Prospect) error {
	return nil
}
func (m *mockProspectsRepo) DeleteProspect(_ context.Context, _ uuid.UUID) error { return nil }
func (m *mockProspectsRepo) UpdateStatus(_ context.Context, _ uuid.UUID, _ prospectsdomain.ProspectStatus) error {
	return m.updateErr
}
func (m *mockProspectsRepo) ConvertToLead(_ context.Context, _, _ uuid.UUID) error { return nil }
func (m *mockProspectsRepo) UpdateVerification(_ context.Context, _ uuid.UUID, _ prospectsdomain.VerifyStatus, _ int, _ string, _ time.Time) error {
	return nil
}

// --- Tests ---

func TestProspectReaderAdapter_GetProspect(t *testing.T) {
	pid := uuid.New()
	userID := uuid.New()
	sourceID := uuid.New()
	now := time.Now().UTC()

	repo := &mockProspectsRepo{
		prospect: &prospectsdomain.Prospect{
			ID:               pid,
			UserID:           userID,
			Name:             "Alice",
			Company:          "Acme",
			Title:            "CEO",
			Email:            "alice@acme.com",
			Phone:            "+1234",
			WhatsApp:         "+1234",
			TelegramUsername: "@alice",
			Context:          "CEO context",
			Source:           "linkedin",
			SourceID:         &sourceID,
			Status:           "new",
			VerifyStatus:     "valid",
			VerifiedAt:       &now,
		},
	}

	adapter := NewProspectReaderAdapter(repo)
	view, err := adapter.GetProspect(context.Background(), pid)
	require.NoError(t, err)
	require.NotNil(t, view)
	assert.Equal(t, pid, view.ID)
	assert.Equal(t, userID, view.UserID)
	assert.Equal(t, "Alice", view.Name)
	assert.Equal(t, "Acme", view.Company)
	assert.Equal(t, "CEO", view.Title)
	assert.Equal(t, "alice@acme.com", view.Email)
	assert.Equal(t, "+1234", view.Phone)
	assert.Equal(t, "@alice", view.TelegramUsername)
	assert.Equal(t, "CEO context", view.Context)
	assert.Equal(t, "linkedin", view.Source)
	assert.Equal(t, &sourceID, view.SourceID)
	assert.Equal(t, "new", view.Status)
	assert.Equal(t, "valid", view.VerifyStatus)
	assert.NotNil(t, view.VerifiedAt)
}

func TestProspectReaderAdapter_GetProspect_NotFound(t *testing.T) {
	repo := &mockProspectsRepo{prospect: nil}
	adapter := NewProspectReaderAdapter(repo)

	view, err := adapter.GetProspect(context.Background(), uuid.New())
	require.NoError(t, err)
	assert.Nil(t, view)
}

func TestProspectReaderAdapter_GetProspect_Error(t *testing.T) {
	repo := &mockProspectsRepo{getErr: errors.New("db error")}
	adapter := NewProspectReaderAdapter(repo)

	view, err := adapter.GetProspect(context.Background(), uuid.New())
	require.Error(t, err)
	assert.Nil(t, view)
}

func TestProspectReaderAdapter_MarkInSequence(t *testing.T) {
	// UpdateStatus now routes through Prospect.TransitionTo (see adapter
	// comment), so it must find a prospect to transition. Seed a draft one.
	p, _ := prospectsdomain.NewProspect(uuid.New(), "X", "Y", "", "", "manual")
	repo := &mockProspectsRepo{prospect: p}
	adapter := NewProspectReaderAdapter(repo)

	err := adapter.MarkInSequence(context.Background(), p.ID)
	require.NoError(t, err)
}

func TestProspectReaderAdapter_MarkInSequence_IllegalTransition(t *testing.T) {
	p, _ := prospectsdomain.NewProspect(uuid.New(), "X", "Y", "", "", "manual")
	p.Status = prospectsdomain.ProspectStatusConverted // terminal
	repo := &mockProspectsRepo{prospect: p}
	adapter := NewProspectReaderAdapter(repo)

	err := adapter.MarkInSequence(context.Background(), p.ID)
	require.Error(t, err)
}

func TestProspectReaderAdapter_MarkInSequence_NotFound(t *testing.T) {
	repo := &mockProspectsRepo{}
	adapter := NewProspectReaderAdapter(repo)
	err := adapter.MarkInSequence(context.Background(), uuid.New())
	require.Error(t, err)
}

func TestProspectReaderAdapter_MarkInSequence_PersistError(t *testing.T) {
	p, _ := prospectsdomain.NewProspect(uuid.New(), "X", "Y", "", "", "manual")
	repo := &mockProspectsRepo{prospect: p, updateErr: errors.New("update failed")}
	adapter := NewProspectReaderAdapter(repo)

	err := adapter.MarkInSequence(context.Background(), p.ID)
	require.Error(t, err)
}
