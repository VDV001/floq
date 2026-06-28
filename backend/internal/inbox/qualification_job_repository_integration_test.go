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

func TestQualificationJobRepo_ClaimOne_RoundTripAttemptsCapAndSaveDrop(t *testing.T) {
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

	a := mkJob("a", -270*time.Second, 0, JobPending)
	b := mkJob("b", -70*time.Second, 2, JobPending) // 2 attempts already

	// At cap 2, b (attempts=2) is excluded; a is the only claimable row.
	got, err := repo.ClaimDueQualificationJob(ctx, 2, 300)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, a.ID, got.ID)
	assert.Equal(t, "a", got.QualifyText, "fields round-trip")
	assert.Equal(t, ChannelEmail, got.Channel)
	assert.Equal(t, JobPending, got.Status)

	// a is now leased and b is over the cap → nothing else claimable at cap 2.
	none, err := repo.ClaimDueQualificationJob(ctx, 2, 300)
	require.NoError(t, err)
	assert.Nil(t, none)

	// Mark a done and save → terminal, excluded from future claims.
	a.MarkDone(now)
	require.NoError(t, repo.SaveQualificationJob(ctx, a))

	// At cap 5, a is terminal (excluded) and b (attempts=2 < 5) is now claimable.
	got2, err := repo.ClaimDueQualificationJob(ctx, 5, 300)
	require.NoError(t, err)
	require.NotNil(t, got2)
	assert.Equal(t, b.ID, got2.ID, "a done job is no longer claimed; b clears the cap at 5")
}
