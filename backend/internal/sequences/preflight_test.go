package sequences

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/daniil/floq/internal/sequences/domain"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockEmailChecker is a fake EmailConfigChecker recording its call count.
type mockEmailChecker struct {
	err   error
	calls int
}

func (m *mockEmailChecker) IsEmailConfigured(_ context.Context, _ uuid.UUID) error {
	m.calls++
	return m.err
}

// Launch preflights email configuration only when the sequence has an email
// step: an unconfigured mailer would otherwise let the async sender drop the
// message silently, with nothing to surface to the operator.
func TestLaunch_EmailConfigPreflight(t *testing.T) {
	emailStep := domain.SequenceStep{ID: uuid.New(), StepOrder: 1, Channel: domain.StepChannelEmail, PromptHint: "intro"}
	tgStep := domain.SequenceStep{ID: uuid.New(), StepOrder: 1, Channel: domain.StepChannelTelegram, PromptHint: "intro"}

	tests := []struct {
		name         string
		steps        []domain.SequenceStep
		checkerErr   error
		wantErr      error
		wantChecked  bool
		wantMessages int
	}{
		{"email step, mailer not configured", []domain.SequenceStep{emailStep}, domain.ErrEmailNotConfigured, domain.ErrEmailNotConfigured, true, 0},
		{"email step, mailer configured", []domain.SequenceStep{emailStep}, nil, nil, true, 1},
		{"telegram only, preflight skipped", []domain.SequenceStep{tgStep}, domain.ErrEmailNotConfigured, nil, false, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			seqID := uuid.New()
			pid := uuid.New()
			steps := make([]domain.SequenceStep, len(tt.steps))
			for i, s := range tt.steps {
				s.SequenceID = seqID
				steps[i] = s
			}
			repo := &mockRepo{steps: steps}
			pr := newMockProspectReader()
			pr.prospects[pid] = &domain.ProspectView{
				ID: pid, UserID: uuid.New(), Name: "Alice", Company: "Acme", Title: "CEO",
				Email: "alice@acme.com", Status: "new", VerifyStatus: "valid", IsEligibleForSequence: true,
			}
			checker := &mockEmailChecker{err: tt.checkerErr}
			uc := NewUseCase(repo, &mockAI{coldBody: "hi", telegramBody: "hi"}, pr, &mockLeadCreator{},
				WithEmailConfigChecker(checker))

			err := uc.Launch(context.Background(), uuid.Nil, seqID, []uuid.UUID{pid}, true)

			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
			} else {
				require.NoError(t, err)
			}
			if tt.wantChecked {
				assert.Equal(t, 1, checker.calls, "email config must be checked")
			} else {
				assert.Equal(t, 0, checker.calls, "preflight skipped for non-email sequence")
			}
			assert.Len(t, repo.messages, tt.wantMessages)
		})
	}
}

// A nil checker (the default wiring before this feature) must not change
// launch behaviour — preflight is skipped entirely.
func TestLaunch_NoEmailChecker_SkipsPreflight(t *testing.T) {
	seqID := uuid.New()
	pid := uuid.New()
	repo := &mockRepo{steps: []domain.SequenceStep{
		{ID: uuid.New(), SequenceID: seqID, StepOrder: 1, Channel: domain.StepChannelEmail, PromptHint: "intro"},
	}}
	pr := newMockProspectReader()
	pr.prospects[pid] = &domain.ProspectView{
		ID: pid, UserID: uuid.New(), Name: "Alice", Status: "new", VerifyStatus: "valid", IsEligibleForSequence: true,
	}
	uc := NewUseCase(repo, &mockAI{coldBody: "hi"}, pr, &mockLeadCreator{})

	require.NoError(t, uc.Launch(context.Background(), uuid.Nil, seqID, []uuid.UUID{pid}, true))
	assert.Len(t, repo.messages, 1)
}

func TestHandler_LaunchSequence_EmailNotConfigured(t *testing.T) {
	userID := uuid.New()
	seqID := uuid.New()
	pid := uuid.New()

	repo := &mockRepo{steps: []domain.SequenceStep{
		{ID: uuid.New(), SequenceID: seqID, StepOrder: 1, Channel: domain.StepChannelEmail, PromptHint: "intro"},
	}}
	pr := newMockProspectReader()
	pr.prospects[pid] = &domain.ProspectView{
		ID: pid, UserID: userID, Name: "Alice", Company: "Acme", Title: "CEO",
		Email: "alice@acme.com", Status: "new", VerifyStatus: "valid", IsEligibleForSequence: true,
	}
	uc := NewUseCase(repo, &mockAI{coldBody: "hi"}, pr, &mockLeadCreator{},
		WithEmailConfigChecker(&mockEmailChecker{err: domain.ErrEmailNotConfigured}))
	router := setupRouter(uc, userID)

	rr := doRequest(router, http.MethodPost, "/api/sequences/"+seqID.String()+"/launch",
		map[string]any{"prospect_ids": []string{pid.String()}, "send_now": true})

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	var body map[string]string
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&body))
	assert.Equal(t, "email_not_configured", body["code"])
	assert.NotEmpty(t, body["remedy"])
	assert.NotEmpty(t, body["error"])
}
