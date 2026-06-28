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

// PurgeTerminalJobsOlderThan must delete only terminal (done/failed) rows whose
// updated_at predates the threshold — recent terminal rows and pending rows of
// any age are spared (#212).
func TestQualificationJobRepo_PurgeTerminalOlderThan(t *testing.T) {
	pool := testutil.TestDB(t)
	userID := testutil.SeedUser(t, pool)
	repo := NewQualificationJobRepository(pool)
	ctx := context.Background()

	leadID := uuid.New()
	_, err := pool.Exec(ctx,
		`INSERT INTO leads (id, user_id, channel, contact_name, company, first_message, status)
		 VALUES ($1, $2, 'email', 'Lead', '', 'hi', 'new')`, leadID, userID)
	require.NoError(t, err)

	mk := func(text string, status JobStatus, updatedAt time.Time) uuid.UUID {
		j, e := NewQualificationJob(leadID, userID, "Alice", ChannelEmail, text)
		require.NoError(t, e)
		require.NoError(t, repo.EnqueueQualificationJob(ctx, j))
		// Backdate updated_at and pin status directly so the row's terminal age
		// is controlled regardless of what enqueue defaults.
		_, e = pool.Exec(ctx,
			`UPDATE lead_qualification_jobs SET status=$2, updated_at=$3 WHERE id=$1`,
			j.ID, string(status), updatedAt)
		require.NoError(t, e)
		return j.ID
	}

	now := time.Now().UTC()
	oldDone := mk("old-done", JobDone, now.Add(-48*time.Hour))
	oldFailed := mk("old-failed", JobFailed, now.Add(-48*time.Hour))
	recentDone := mk("recent-done", JobDone, now.Add(-1*time.Hour))
	oldPending := mk("old-pending", JobPending, now.Add(-48*time.Hour))

	n, err := repo.PurgeTerminalJobsOlderThan(ctx, now.Add(-24*time.Hour))
	require.NoError(t, err)
	assert.Equal(t, 2, n, "only old terminal (done+failed) rows are purged")

	exists := func(id uuid.UUID) bool {
		var x int
		return pool.QueryRow(ctx, `SELECT 1 FROM lead_qualification_jobs WHERE id=$1`, id).Scan(&x) == nil
	}
	assert.False(t, exists(oldDone), "old done must be purged")
	assert.False(t, exists(oldFailed), "old failed must be purged")
	assert.True(t, exists(recentDone), "recent terminal row is within the window, kept")
	assert.True(t, exists(oldPending), "pending rows are never purged regardless of age")
}
