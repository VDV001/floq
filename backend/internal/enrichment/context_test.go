package enrichment_test

import (
	"context"
	"testing"

	"github.com/daniil/floq/internal/enrichment"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestSubjectUserContext_RoundTrip(t *testing.T) {
	uid := uuid.New()
	ctx := enrichment.WithSubjectUser(context.Background(), uid)

	got, ok := enrichment.SubjectUserFromContext(ctx)
	assert.True(t, ok)
	assert.Equal(t, uid, got)
}

func TestSubjectUserContext_Absent(t *testing.T) {
	_, ok := enrichment.SubjectUserFromContext(context.Background())
	assert.False(t, ok, "no subject user set -> not ok")
}

func TestSubjectUserContext_NilNotStored(t *testing.T) {
	// A nil user is no attribution: WithSubjectUser(Nil) must not pretend a
	// subject is present (otherwise the adapter would build a CallMeta that
	// NewEntry rejects for UserID==Nil).
	ctx := enrichment.WithSubjectUser(context.Background(), uuid.Nil)
	_, ok := enrichment.SubjectUserFromContext(ctx)
	assert.False(t, ok)
}
