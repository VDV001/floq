package reminders

import (
	"context"
	"errors"
	"testing"
	"time"

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

func TestCheck_StaleLeadsQueryError(t *testing.T) {
	repo := &mockLeadRepo{staleErr: errors.New("db connection lost")}
	ai := &mockFollowupGen{body: "should not be called"}
	notifier := &mockNotifier{}

	c := NewCron(repo, ai, notifier, 7)
	c.check(context.Background())

	assert.Empty(t, repo.createdReminders)
	assert.Empty(t, notifier.alerts)
}

func TestCheck_CreateReminderError(t *testing.T) {
	leadID := uuid.New()
	lead := domain.Lead{
		ID:           leadID,
		ContactName:  "Dave",
		Company:      "DevCorp",
		FirstMessage: "Need help",
	}

	repo := &mockLeadRepo{
		staleLeads: []domain.Lead{lead},
		createErr:  errors.New("insert failed"),
	}
	ai := &mockFollowupGen{body: "Follow up text"}
	notifier := &mockNotifier{}

	c := NewCron(repo, ai, notifier, 5)
	c.check(context.Background())

	// CreateReminder fails, so no reminders are persisted
	assert.Empty(t, repo.createdReminders)
	// Notifier should NOT be called if CreateReminder fails
	assert.Empty(t, notifier.alerts)
}

func TestCheck_NotifierError(t *testing.T) {
	leadID := uuid.New()
	lead := domain.Lead{
		ID:           leadID,
		ContactName:  "Eve",
		Company:      "EvilCorp",
		FirstMessage: "Interested",
	}

	repo := &mockLeadRepo{staleLeads: []domain.Lead{lead}}
	ai := &mockFollowupGen{body: "Followup message"}
	notifier := &mockNotifier{err: errors.New("telegram API down")}

	c := NewCron(repo, ai, notifier, 3)
	c.check(context.Background())

	// Reminder should still be created even if notifier fails
	require.Len(t, repo.createdReminders, 1)
	assert.Equal(t, leadID, repo.createdReminders[0].LeadID)
	// Notifier was called but errored — no alerts recorded
	assert.Empty(t, notifier.alerts)
}

func TestCheck_MultipleStaleLeads(t *testing.T) {
	lead1 := domain.Lead{
		ID:           uuid.New(),
		ContactName:  "Alice",
		Company:      "Acme",
		FirstMessage: "Hi",
	}
	lead2 := domain.Lead{
		ID:           uuid.New(),
		ContactName:  "Bob",
		Company:      "Corp",
		FirstMessage: "Hello",
	}

	repo := &mockLeadRepo{staleLeads: []domain.Lead{lead1, lead2}}
	ai := &mockFollowupGen{body: "Follow up!"}
	notifier := &mockNotifier{}

	c := NewCron(repo, ai, notifier, 7)
	c.check(context.Background())

	require.Len(t, repo.createdReminders, 2)
	require.Len(t, notifier.alerts, 2)
	assert.Equal(t, "Alice", notifier.alerts[0].ContactName)
	assert.Equal(t, "Bob", notifier.alerts[1].ContactName)
}

func TestCheck_AIErrorSkipsOneLeadContinuesNext(t *testing.T) {
	lead1 := domain.Lead{
		ID:           uuid.New(),
		ContactName:  "Fail",
		Company:      "Corp",
		FirstMessage: "msg1",
	}
	lead2 := domain.Lead{
		ID:           uuid.New(),
		ContactName:  "Success",
		Company:      "Inc",
		FirstMessage: "msg2",
	}

	// AI alternates: first call fails, second succeeds.
	// We can't do that with simple mockFollowupGen, so we use a counter-based mock.
	aiMock := &mockFollowupGenAlternating{
		results: []followupResult{
			{body: "", err: errors.New("AI fail")},
			{body: "Generated followup", err: nil},
		},
	}

	repo := &mockLeadRepo{staleLeads: []domain.Lead{lead1, lead2}}
	notifier := &mockNotifier{}

	c := NewCron(repo, aiMock, notifier, 5)
	c.check(context.Background())

	// Only second lead should get a reminder
	require.Len(t, repo.createdReminders, 1)
	assert.Equal(t, lead2.ID, repo.createdReminders[0].LeadID)
	require.Len(t, notifier.alerts, 1)
	assert.Equal(t, "Success", notifier.alerts[0].ContactName)
}

// --- Additional mock for alternating AI results ---

type followupResult struct {
	body string
	err  error
}

type mockFollowupGenAlternating struct {
	results []followupResult
	idx     int
}

func (m *mockFollowupGenAlternating) GenerateFollowup(_ context.Context, _, _, _, _, _ string) (string, error) {
	if m.idx >= len(m.results) {
		return "", errors.New("no more results")
	}
	r := m.results[m.idx]
	m.idx++
	return r.body, r.err
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

func TestCron_Start_CancelledContext(t *testing.T) {
	repo := &mockLeadRepo{}
	c := NewCron(repo, &mockFollowupGen{}, nil, 7)

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		c.Start(ctx)
		close(done)
	}()

	// Let Start run check once, then cancel
	cancel()

	select {
	case <-done:
		// OK
	case <-time.After(2 * time.Second):
		t.Fatal("Start did not return after context cancellation")
	}
}
