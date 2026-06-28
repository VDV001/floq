package inbox

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewQualificationJob_Valid(t *testing.T) {
	leadID, userID := uuid.New(), uuid.New()
	j, err := NewQualificationJob(leadID, userID, "Alice", ChannelEmail, "I need a website")
	require.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, j.ID)
	assert.Equal(t, leadID, j.LeadID)
	assert.Equal(t, userID, j.UserID)
	assert.Equal(t, "Alice", j.ContactName)
	assert.Equal(t, ChannelEmail, j.Channel)
	assert.Equal(t, "I need a website", j.QualifyText)
	assert.Equal(t, JobPending, j.Status)
	assert.Equal(t, 0, j.Attempts)
	assert.Nil(t, j.NextRetryAt, "a fresh job is due immediately")
}

func TestNewQualificationJob_Invariants(t *testing.T) {
	leadID, userID := uuid.New(), uuid.New()
	cases := []struct {
		name    string
		leadID  uuid.UUID
		userID  uuid.UUID
		text    string
		wantErr error
	}{
		{"empty text", leadID, userID, "   ", ErrEmptyQualifyText},
		{"nil lead", uuid.Nil, userID, "hi", ErrEmptyJobLead},
		{"nil owner", leadID, uuid.Nil, "hi", ErrEmptyJobOwner},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := NewQualificationJob(tc.leadID, tc.userID, "Alice", ChannelEmail, tc.text)
			require.Error(t, err)
			assert.True(t, errors.Is(err, tc.wantErr), "want %v, got %v", tc.wantErr, err)
		})
	}
}

func TestQualificationJob_MarkDone(t *testing.T) {
	j, _ := NewQualificationJob(uuid.New(), uuid.New(), "Alice", ChannelEmail, "hi")
	now := time.Date(2026, 6, 28, 12, 0, 0, 0, time.UTC)
	j.MarkDone(now)
	assert.Equal(t, JobDone, j.Status)
	assert.Nil(t, j.NextRetryAt, "a done job is never retried")
}

func TestQualificationJob_MarkFailed_Retryable(t *testing.T) {
	j, _ := NewQualificationJob(uuid.New(), uuid.New(), "Alice", ChannelEmail, "hi")
	now := time.Date(2026, 6, 28, 12, 0, 0, 0, time.UTC)
	j.MarkFailed("ai timeout", 5, now)
	assert.Equal(t, JobPending, j.Status, "below max attempts stays pending (retryable)")
	assert.Equal(t, 1, j.Attempts)
	assert.Equal(t, "ai timeout", j.LastError)
	require.NotNil(t, j.NextRetryAt)
	assert.Equal(t, now.Add(QualRetryBaseBackoff), *j.NextRetryAt, "first retry is one base backoff out")
}

func TestQualificationJob_MarkFailed_DeadLetter(t *testing.T) {
	j, _ := NewQualificationJob(uuid.New(), uuid.New(), "Alice", ChannelEmail, "hi")
	now := time.Date(2026, 6, 28, 12, 0, 0, 0, time.UTC)
	j.Attempts = 4
	j.MarkFailed("ai timeout", 5, now)
	assert.Equal(t, JobFailed, j.Status, "reaching max attempts is terminal (dead-letter)")
	assert.Equal(t, 5, j.Attempts)
	assert.Nil(t, j.NextRetryAt, "a dead-lettered job is never retried")
}

func TestQualificationJob_NextRetryAfter_ExponentialBackoff(t *testing.T) {
	j, _ := NewQualificationJob(uuid.New(), uuid.New(), "Alice", ChannelEmail, "hi")
	base := time.Date(2026, 6, 28, 12, 0, 0, 0, time.UTC)
	j.Attempts = 1
	assert.Equal(t, base.Add(QualRetryBaseBackoff), j.NextRetryAfter(base))
	j.Attempts = 3
	assert.Equal(t, base.Add(4*QualRetryBaseBackoff), j.NextRetryAfter(base), "doubles per attempt")
}

// The terminal-status set is the single source the retention GC sweeps; pin it so
// a new terminal status can't be added to the enum without joining the set (#212).
func TestTerminalJobStatuses_IsTheTerminalSet(t *testing.T) {
	got := TerminalJobStatuses()
	assert.ElementsMatch(t, []JobStatus{JobDone, JobFailed}, got,
		"terminal set must be exactly done+failed")
	assert.NotContains(t, got, JobPending, "pending is never terminal")
}
