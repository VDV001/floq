package main

import (
	"testing"
	"time"

	"github.com/daniil/floq/internal/inbox"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fromInboxQualification is the cross-context DTO->domain adapter. These tests
// pin the domain invariants it must enforce on rehydration — in particular
// ClampScore must always run, otherwise inbox-originated qualifications can
// persist scores outside [0,100] and silently break the IsHot/IsWarm bands.

func TestFromInboxQualification_NilInput(t *testing.T) {
	assert.Nil(t, fromInboxQualification(nil))
}

func TestFromInboxQualification_PreservesIDAndGeneratedAt(t *testing.T) {
	id := uuid.New()
	leadID := uuid.New()
	generatedAt := time.Now().UTC().Add(-5 * time.Minute)
	out := fromInboxQualification(&inbox.InboxQualification{
		ID: id, LeadID: leadID, Score: 60, GeneratedAt: generatedAt,
	})
	require.NotNil(t, out)
	// Caller-supplied identity/timestamp MUST survive the adapter (unlike
	// NewQualification, which would regenerate them).
	assert.Equal(t, id, out.ID)
	assert.Equal(t, leadID, out.LeadID)
	assert.Equal(t, generatedAt, out.GeneratedAt)
}

func TestFromInboxQualification_ClampsScoreAbove100(t *testing.T) {
	// Regression guard: the adapter's ClampScore call at adapters.go is
	// load-bearing. A score of 150 coming from an over-enthusiastic AI must
	// be coerced to 100 BEFORE leaving the adapter. Re-removing the
	// ClampScore call would compile and pass every other test; this test
	// is the only thing that stops it.
	out := fromInboxQualification(&inbox.InboxQualification{
		ID: uuid.New(), LeadID: uuid.New(), Score: 150,
	})
	require.NotNil(t, out)
	assert.Equal(t, 100, out.Score)
}

func TestFromInboxQualification_ClampsScoreBelowZero(t *testing.T) {
	out := fromInboxQualification(&inbox.InboxQualification{
		ID: uuid.New(), LeadID: uuid.New(), Score: -42,
	})
	require.NotNil(t, out)
	assert.Equal(t, 0, out.Score)
}

func TestFromInboxQualification_PassesValidScore(t *testing.T) {
	out := fromInboxQualification(&inbox.InboxQualification{
		ID: uuid.New(), LeadID: uuid.New(), Score: 73,
	})
	require.NotNil(t, out)
	assert.Equal(t, 73, out.Score)
	assert.True(t, out.IsWarm())
	assert.False(t, out.IsHot())
}

// TestFromInboxQualification_ScoreBoundaries pins the clamp contract at the
// adapter boundary across the full legal and illegal range. A future refactor
// that somehow skips the domain factory would need to break one of these rows
// to ship.
func TestFromInboxQualification_ScoreBoundaries(t *testing.T) {
	cases := []struct {
		in, want int
	}{
		{-1 << 30, 0},     // far negative
		{-1, 0},           // just below lower bound
		{0, 0},            // lower boundary
		{1, 1},            // just above lower bound
		{49, 49},          // cold/warm band edge
		{50, 50},          // warm lower bound
		{79, 79},          // warm/hot edge
		{80, 80},          // hot lower bound
		{100, 100},        // upper boundary
		{101, 100},        // just above upper bound
		{150, 100},        // the canonical "over-enthusiastic AI" case
		{1 << 30, 100},    // far positive
	}
	for _, c := range cases {
		out := fromInboxQualification(&inbox.InboxQualification{
			ID: uuid.New(), LeadID: uuid.New(), Score: c.in,
		})
		require.NotNil(t, out)
		assert.Equalf(t, c.want, out.Score, "input=%d", c.in)
	}
}

func TestFromInboxQualification_MapsAllFields(t *testing.T) {
	in := &inbox.InboxQualification{
		ID:                uuid.New(),
		LeadID:            uuid.New(),
		IdentifiedNeed:    "CRM",
		EstimatedBudget:   "100k",
		Deadline:          "Q2",
		Score:             85,
		ScoreReason:       "match",
		RecommendedAction: "call",
		ProviderUsed:      "anthropic",
		GeneratedAt:       time.Now().UTC(),
	}
	out := fromInboxQualification(in)
	require.NotNil(t, out)
	assert.Equal(t, in.IdentifiedNeed, out.IdentifiedNeed)
	assert.Equal(t, in.EstimatedBudget, out.EstimatedBudget)
	assert.Equal(t, in.Deadline, out.Deadline)
	assert.Equal(t, in.ScoreReason, out.ScoreReason)
	assert.Equal(t, in.RecommendedAction, out.RecommendedAction)
	assert.Equal(t, in.ProviderUsed, out.ProviderUsed)
	assert.True(t, out.IsHot())
}
