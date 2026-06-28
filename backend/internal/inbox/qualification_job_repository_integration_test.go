//go:build integration

package inbox

import (
	"context"
	"testing"
	"time"

	"github.com/daniil/floq/internal/testutil"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestQualificationJobRepo_RoundTripAndClaimOrdering(t *testing.T) {
	pool := testutil.TestDB(t)
	userID := testutil.SeedUser(t, pool)
	repo := NewQualificationJobRepository(pool)
	ctx := context.Background()

	leadID := uuid.New()
	_, err := pool.Exec(ctx,
		`INSERT INTO leads (id, user_id, channel, contact_name, company, first_message, status)
		 VALUES ($1, $2, 'email', 'Lead', '', 'hi', 'new')`, leadID, userID)
	require.NoError(t, err)

	now := time.Now().UTC()
	mkJob := func(text string, dueOffset time.Duration, attempts int, status JobStatus) *QualificationJob {
		j, e := NewQualificationJob(leadID, userID, "Alice", ChannelEmail, text)
		require.NoError(t, e)
		due := now.Add(dueOffset)
		j.NextRetryAt = &due
		j.Attempts = attempts
		j.Status = status
		require.NoError(t, repo.EnqueueQualificationJob(ctx, j))
		return j
	}

	// Three due jobs at distinct effective due-times, plus one not-yet-due and
	// one dead-lettered job that must be excluded.
	c := mkJob("c", -10*time.Second, 0, JobPending)
	a := mkJob("a", -270*time.Second, 0, JobPending)
	b := mkJob("b", -70*time.Second, 0, JobPending)
	_ = mkJob("future", 1*time.Hour, 0, JobPending) // not due
	_ = mkJob("dead", -1*time.Hour, 3, JobFailed)   // terminal

	got, err := repo.ClaimDueQualificationJobs(ctx, 10, 5)
	require.NoError(t, err)
	require.Len(t, got, 3, "only the three due pending jobs are claimed")
	assert.Equal(t, []uuid.UUID{a.ID, b.ID, c.ID}, []uuid.UUID{got[0].ID, got[1].ID, got[2].ID},
		"claimed earliest-due-first")
	assert.Equal(t, "a", got[0].QualifyText, "fields round-trip")
	assert.Equal(t, ChannelEmail, got[0].Channel)
	assert.Equal(t, JobPending, got[0].Status)

	// Save an outcome and confirm it persists + drops out of the due set.
	a.MarkDone(now)
	require.NoError(t, repo.SaveQualificationJob(ctx, a))
	got2, err := repo.ClaimDueQualificationJobs(ctx, 10, 5)
	require.NoError(t, err)
	require.Len(t, got2, 2, "a done job is no longer claimed")
	assert.Equal(t, b.ID, got2[0].ID)

	// The attempts < maxAttempts gate: bump b to 2 attempts and claim with cap 2.
	b.Attempts = 2
	b.Status = JobPending
	require.NoError(t, repo.SaveQualificationJob(ctx, b))
	got3, err := repo.ClaimDueQualificationJobs(ctx, 10, 2)
	require.NoError(t, err)
	require.Len(t, got3, 1, "b (attempts=2) is excluded at maxAttempts=2; only c remains")
	assert.Equal(t, c.ID, got3[0].ID)
}
