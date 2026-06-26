package domain_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/daniil/floq/internal/audit/domain"
)

func TestContextWithCallMeta_RoundTrips(t *testing.T) {
	t.Parallel()
	leadID := uuid.New()
	in := domain.CallMeta{
		UserID:      uuid.New(),
		LeadID:      &leadID,
		RequestType: domain.RequestTypeQualification,
	}
	ctx := domain.ContextWithCallMeta(context.Background(), in)

	out, ok := domain.CallMetaFromContext(ctx)
	require.True(t, ok)
	assert.Equal(t, in, out)
}

func TestCallMetaFromContext_MissingReturnsFalse(t *testing.T) {
	t.Parallel()
	_, ok := domain.CallMetaFromContext(context.Background())
	assert.False(t, ok)
}

func TestWithRequestType_PreservesAttributionOverridesType(t *testing.T) {
	t.Parallel()
	userID := uuid.New()
	leadID := uuid.New()
	parent := domain.ContextWithCallMeta(context.Background(), domain.CallMeta{
		UserID:      userID,
		LeadID:      &leadID,
		RequestType: domain.RequestTypeDraftReply,
	})

	child := domain.WithRequestType(parent, domain.RequestTypeStyleCheck)

	meta, ok := domain.CallMetaFromContext(child)
	require.True(t, ok)
	assert.Equal(t, userID, meta.UserID, "user attribution must survive override")
	require.NotNil(t, meta.LeadID)
	assert.Equal(t, leadID, *meta.LeadID, "lead attribution must survive override")
	assert.Equal(t, domain.RequestTypeStyleCheck, meta.RequestType)
}

func TestWithRequestType_NoMetaReturnsCtxUnchanged(t *testing.T) {
	t.Parallel()
	original := context.Background()
	child := domain.WithRequestType(original, domain.RequestTypeStyleCheck)

	_, ok := domain.CallMetaFromContext(child)
	assert.False(t, ok, "WithRequestType must NOT synthesize meta when none present")
}

func TestWithRequestType_ProspectAttributionPreserved(t *testing.T) {
	t.Parallel()
	userID := uuid.New()
	prospectID := uuid.New()
	parent := domain.ContextWithCallMeta(context.Background(), domain.CallMeta{
		UserID:      userID,
		ProspectID:  &prospectID,
		RequestType: domain.RequestTypeColdMessage,
	})

	child := domain.WithRequestType(parent, domain.RequestTypeStyleCheck)
	meta, ok := domain.CallMetaFromContext(child)
	require.True(t, ok)
	require.NotNil(t, meta.ProspectID)
	assert.Equal(t, prospectID, *meta.ProspectID)
	assert.Nil(t, meta.LeadID, "no lead attribution to fabricate")
}
