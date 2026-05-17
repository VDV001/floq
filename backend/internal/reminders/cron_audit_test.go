package reminders

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	auditdomain "github.com/daniil/floq/internal/audit/domain"
	"github.com/daniil/floq/internal/leads/domain"
)

type auditingFollowupGen struct {
	lastCtx context.Context
}

func (a *auditingFollowupGen) GenerateFollowup(ctx context.Context, _, _, _, _, _ string) (string, error) {
	a.lastCtx = ctx
	return "Followup body", nil
}

func TestCheck_AttachesFollowupCallMetaPerLead(t *testing.T) {
	userID := uuid.New()
	leadID := uuid.New()
	repo := &mockLeadRepo{staleLeads: []domain.Lead{{
		ID:           leadID,
		UserID:       userID,
		ContactName:  "Alice",
		Company:      "Acme",
		FirstMessage: "Hi",
	}}}
	ai := &auditingFollowupGen{}

	c := NewCron(repo, ai, nil, 5)
	c.check(context.Background())

	require.NotNil(t, ai.lastCtx, "GenerateFollowup was never called")
	meta, ok := auditdomain.CallMetaFromContext(ai.lastCtx)
	require.True(t, ok, "Cron must attach CallMeta before generating followup")
	assert.Equal(t, userID, meta.UserID)
	require.NotNil(t, meta.LeadID)
	assert.Equal(t, leadID, *meta.LeadID)
	assert.Equal(t, auditdomain.RequestTypeFollowup, meta.RequestType)
}
