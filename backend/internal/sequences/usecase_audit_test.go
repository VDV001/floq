package sequences

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	auditdomain "github.com/daniil/floq/internal/audit/domain"
	"github.com/daniil/floq/internal/sequences/domain"
)

// auditingAI records the ctx of each Generate* call so tests can
// assert the use case stamped the right CallMeta before invoking it.
type auditingAI struct {
	coldCtx     context.Context
	telegramCtx context.Context
	briefCtx    context.Context
}

func (a *auditingAI) GenerateColdMessage(ctx context.Context, _, _, _, _, _, _, _, _ string) (string, error) {
	a.coldCtx = ctx
	return "email body", nil
}

func (a *auditingAI) GenerateTelegramMessage(ctx context.Context, _, _, _, _, _, _, _, _ string) (string, error) {
	a.telegramCtx = ctx
	return "tg body", nil
}

func (a *auditingAI) GenerateCallBrief(ctx context.Context, _, _, _, _, _, _ string) (string, error) {
	a.briefCtx = ctx
	return "brief", nil
}

func TestLaunch_AttachesCallMetaPerChannel(t *testing.T) {
	seqID := uuid.New()
	pid := uuid.New()
	prospectUserID := uuid.New()

	repo := &mockRepo{
		steps: []domain.SequenceStep{
			{ID: uuid.New(), SequenceID: seqID, StepOrder: 1, DelayDays: 0, Channel: domain.StepChannelEmail, PromptHint: "h"},
			{ID: uuid.New(), SequenceID: seqID, StepOrder: 2, DelayDays: 1, Channel: domain.StepChannelTelegram, PromptHint: "h"},
			{ID: uuid.New(), SequenceID: seqID, StepOrder: 3, DelayDays: 1, Channel: domain.StepChannelPhoneCall, PromptHint: "h"},
		},
	}
	ai := &auditingAI{}
	pr := newMockProspectReader()
	pr.prospects[pid] = &domain.ProspectView{
		ID:           pid,
		UserID:       prospectUserID,
		Name:         "Alice",
		Company:      "Acme",
		Title:        "CEO",
		Email:        "alice@acme.com",
		Status:       "new",
		VerifyStatus: "valid", IsEligibleForSequence: true,
	}

	uc := NewUseCase(repo, ai, pr, &mockLeadCreator{})

	require.NoError(t, uc.Launch(context.Background(), seqID, []uuid.UUID{pid}))

	cases := []struct {
		channel string
		ctx     context.Context
		wantRT  auditdomain.RequestType
	}{
		{"email/cold", ai.coldCtx, auditdomain.RequestTypeColdMessage},
		{"telegram", ai.telegramCtx, auditdomain.RequestTypeTelegramMessage},
		{"phone/brief", ai.briefCtx, auditdomain.RequestTypeCallBrief},
	}
	for _, tc := range cases {
		t.Run(tc.channel, func(t *testing.T) {
			require.NotNilf(t, tc.ctx, "%s generator was never invoked", tc.channel)
			meta, ok := auditdomain.CallMetaFromContext(tc.ctx)
			require.Truef(t, ok, "%s ctx missing CallMeta", tc.channel)
			assert.Equal(t, prospectUserID, meta.UserID)
			require.NotNil(t, meta.ProspectID)
			assert.Equal(t, pid, *meta.ProspectID)
			assert.Equal(t, tc.wantRT, meta.RequestType)
			assert.Nil(t, meta.LeadID, "sequence launch is prospect-attributed, not lead")
		})
	}
}
