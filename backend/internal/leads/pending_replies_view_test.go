package leads

import (
	"context"
	"errors"
	"testing"

	"github.com/daniil/floq/internal/leads/domain"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakePendingCounter struct {
	counts map[uuid.UUID]int
	err    error
}

func (f *fakePendingCounter) CountPendingByUser(_ context.Context, _ uuid.UUID) (map[uuid.UUID]int, error) {
	return f.counts, f.err
}

func TestUseCase_ListLeadsWithPendingCounts_AttachesCounts(t *testing.T) {
	repo := newMockRepo()
	userID := uuid.New()
	leadA := uuid.New()
	leadB := uuid.New()
	leadC := uuid.New()
	repo.leads[leadA] = &domain.Lead{ID: leadA, UserID: userID, ContactName: "A"}
	repo.leads[leadB] = &domain.Lead{ID: leadB, UserID: userID, ContactName: "B"}
	repo.leads[leadC] = &domain.Lead{ID: leadC, UserID: userID, ContactName: "C"}

	counter := &fakePendingCounter{counts: map[uuid.UUID]int{leadA: 2, leadB: 0}}
	uc := NewUseCase(repo, &mockAI{}, nil, WithPendingReplyCounter(counter))

	out, err := uc.ListLeadsWithPendingCounts(context.Background(), userID)
	require.NoError(t, err)
	require.Len(t, out, 3)

	byLead := map[uuid.UUID]int{}
	for _, lwpc := range out {
		byLead[lwpc.LeadWithSource.ID] = lwpc.PendingCount
	}
	assert.Equal(t, 2, byLead[leadA], "leadA has 2 pending — must surface")
	assert.Equal(t, 0, byLead[leadB], "leadB has 0 in the map — must serialise as 0")
	assert.Equal(t, 0, byLead[leadC], "leadC absent from the counter map — default 0")
}

func TestUseCase_ListLeadsWithPendingCounts_NoCounterWired_AllZero(t *testing.T) {
	// Without a counter wired (composition root opted out, or boot
	// order issue), every lead gets count=0 rather than 500.
	// Defensive degrade — UI shows no badges, no operator visibility,
	// but the list endpoint stays usable.
	repo := newMockRepo()
	userID := uuid.New()
	leadID := uuid.New()
	repo.leads[leadID] = &domain.Lead{ID: leadID, UserID: userID, ContactName: "A"}

	uc := NewUseCase(repo, &mockAI{}, nil) // no WithPendingReplyCounter

	out, err := uc.ListLeadsWithPendingCounts(context.Background(), userID)
	require.NoError(t, err)
	require.Len(t, out, 1)
	assert.Equal(t, 0, out[0].PendingCount)
}

func TestUseCase_ListLeadsWithPendingCounts_CounterErrorDegrades(t *testing.T) {
	// Counter failure must not 500 the inbox list — the lead list
	// itself is critical operator surface. Degrade to count=0.
	repo := newMockRepo()
	userID := uuid.New()
	leadID := uuid.New()
	repo.leads[leadID] = &domain.Lead{ID: leadID, UserID: userID, ContactName: "A"}

	counter := &fakePendingCounter{err: errors.New("db down")}
	uc := NewUseCase(repo, &mockAI{}, nil, WithPendingReplyCounter(counter))

	out, err := uc.ListLeadsWithPendingCounts(context.Background(), userID)
	require.NoError(t, err, "counter error must NOT bubble up — degrade to zero badges instead of 500'ing the inbox")
	require.Len(t, out, 1)
	assert.Equal(t, 0, out[0].PendingCount)
}
