package reminders

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/daniil/floq/internal/leads/domain"
)

// --- Mock implementations ---

type mockLeadRepo struct {
	staleLeads []domain.Lead
	staleErr   error

	createdReminders []struct {
		LeadID  uuid.UUID
		Message string
	}
	createErr error
}

func (m *mockLeadRepo) StaleLeadsWithoutReminder(_ context.Context, _ int) ([]domain.Lead, error) {
	return m.staleLeads, m.staleErr
}

func (m *mockLeadRepo) CreateReminder(_ context.Context, leadID uuid.UUID, message string) error {
	if m.createErr != nil {
		return m.createErr
	}
	m.createdReminders = append(m.createdReminders, struct {
		LeadID  uuid.UUID
		Message string
	}{leadID, message})
	return nil
}

type mockFollowupGen struct {
	body string
	err  error
}

func (m *mockFollowupGen) GenerateFollowup(_ context.Context, _, _, _, _, _ string) (string, error) {
	return m.body, m.err
}

type mockNotifier struct {
	alerts []struct {
		ContactName string
		Company     string
		Body        string
	}
	err error
}

func (m *mockNotifier) SendAlert(_ context.Context, contactName, company, body string) error {
	if m.err != nil {
		return m.err
	}
	m.alerts = append(m.alerts, struct {
		ContactName string
		Company     string
		Body        string
	}{contactName, company, body})
	return nil
}

// --- Tests ---

func TestCheck_NoStaleLeads(t *testing.T) {
	repo := &mockLeadRepo{}
	ai := &mockFollowupGen{body: "should not be called"}
	notifier := &mockNotifier{}

	c := NewCron(repo, ai, notifier, 7)
	c.check(context.Background())

	assert.Empty(t, repo.createdReminders)
	assert.Empty(t, notifier.alerts)
}

func TestCheck_CreatesReminderAndNotifies(t *testing.T) {
	leadID := uuid.New()
	lead := domain.Lead{
		ID:           leadID,
		ContactName:  "Alice",
		Company:      "Acme",
		FirstMessage: "Hi there",
	}

	repo := &mockLeadRepo{staleLeads: []domain.Lead{lead}}
	ai := &mockFollowupGen{body: "Follow up message"}
	notifier := &mockNotifier{}

	c := NewCron(repo, ai, notifier, 5)
	c.check(context.Background())

	require.Len(t, repo.createdReminders, 1)
	assert.Equal(t, leadID, repo.createdReminders[0].LeadID)
	assert.Equal(t, "Follow up message", repo.createdReminders[0].Message)

	require.Len(t, notifier.alerts, 1)
	assert.Equal(t, "Alice", notifier.alerts[0].ContactName)
	assert.Equal(t, "Acme", notifier.alerts[0].Company)
	assert.Equal(t, "Follow up message", notifier.alerts[0].Body)
}

func TestCheck_AIError(t *testing.T) {
	lead := domain.Lead{
		ID:           uuid.New(),
		ContactName:  "Bob",
		Company:      "Corp",
		FirstMessage: "Hello",
	}

	repo := &mockLeadRepo{staleLeads: []domain.Lead{lead}}
	ai := &mockFollowupGen{err: errors.New("AI unavailable")}
	notifier := &mockNotifier{}

	c := NewCron(repo, ai, notifier, 3)
	c.check(context.Background())

	assert.Empty(t, repo.createdReminders)
	assert.Empty(t, notifier.alerts)
}

func TestCheck_NilNotifier(t *testing.T) {
	leadID := uuid.New()
	lead := domain.Lead{
		ID:           leadID,
		ContactName:  "Carol",
		Company:      "Startup",
		FirstMessage: "Hey",
	}

	repo := &mockLeadRepo{staleLeads: []domain.Lead{lead}}
	ai := &mockFollowupGen{body: "Reminder text"}

	c := NewCron(repo, ai, nil, 7)
	c.check(context.Background())

	require.Len(t, repo.createdReminders, 1)
	assert.Equal(t, leadID, repo.createdReminders[0].LeadID)
	assert.Equal(t, "Reminder text", repo.createdReminders[0].Message)
}
