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

// #212 part 2: the claim must lease the row it takes so a second concurrent
// worker skips it (no double-processing), hand rows out earliest-due-first, and
// exclude not-due / terminal rows.
func TestQualificationJobRepo_ClaimOne_LeasesAndOrders(t *testing.T) {
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

	c := mkJob("c", -10*time.Second, 0, JobPending)
	a := mkJob("a", -270*time.Second, 0, JobPending)
	b := mkJob("b", -70*time.Second, 0, JobPending)
	_ = mkJob("future", 1*time.Hour, 0, JobPending) // not due
	_ = mkJob("dead", -1*time.Hour, 3, JobFailed)   // terminal

	const lease = 300 // seconds

	// Successive claims hand out the due pending rows earliest-due-first, each
	// leased so the next claim cannot re-take it — exactly the two-worker race.
	got1, err := repo.ClaimDueQualificationJob(ctx, 5, lease)
	require.NoError(t, err)
	require.NotNil(t, got1)
	assert.Equal(t, a.ID, got1.ID, "first claim takes the earliest-due row")
	assert.Equal(t, "a", got1.QualifyText, "fields round-trip")

	got2, err := repo.ClaimDueQualificationJob(ctx, 5, lease)
	require.NoError(t, err)
	require.NotNil(t, got2)
	assert.Equal(t, b.ID, got2.ID, "second claim skips the leased 'a' and takes 'b'")

	got3, err := repo.ClaimDueQualificationJob(ctx, 5, lease)
	require.NoError(t, err)
	require.NotNil(t, got3)
	assert.Equal(t, c.ID, got3.ID)

	// a/b/c leased, future not due, dead terminal → nothing claimable.
	got4, err := repo.ClaimDueQualificationJob(ctx, 5, lease)
	require.NoError(t, err)
	assert.Nil(t, got4, "no claimable rows remain")
}

// A crashed worker never clears its lease; the lease simply expires and the row
// becomes reclaimable — the recovery path with no separate sweep.
func TestQualificationJobRepo_ClaimOne_ExpiredLeaseReclaimable(t *testing.T) {
	pool := testutil.TestDB(t)
	userID := testutil.SeedUser(t, pool)
	repo := NewQualificationJobRepository(pool)
	ctx := context.Background()

	leadID := uuid.New()
	_, err := pool.Exec(ctx,
		`INSERT INTO leads (id, user_id, channel, contact_name, company, first_message, status)
		 VALUES ($1, $2, 'email', 'Lead', '', 'hi', 'new')`, leadID, userID)
	require.NoError(t, err)

	j, e := NewQualificationJob(leadID, userID, "Alice", ChannelEmail, "x")
	require.NoError(t, e)
	require.NoError(t, repo.EnqueueQualificationJob(ctx, j))

	got1, err := repo.ClaimDueQualificationJob(ctx, 5, 300)
	require.NoError(t, err)
	require.NotNil(t, got1)

	// Immediately re-claiming finds nothing: the row is leased.
	none, err := repo.ClaimDueQualificationJob(ctx, 5, 300)
	require.NoError(t, err)
	require.Nil(t, none)

	// Simulate the lease expiring (worker died mid-process).
	_, err = pool.Exec(ctx,
		`UPDATE lead_qualification_jobs SET locked_until = now() - interval '1 second' WHERE id = $1`, j.ID)
	require.NoError(t, err)

	got2, err := repo.ClaimDueQualificationJob(ctx, 5, 300)
	require.NoError(t, err)
	require.NotNil(t, got2, "an expired lease must be reclaimable")
	assert.Equal(t, j.ID, got2.ID)
}

// Saving an outcome releases the lease so next_retry_at alone governs the next
// attempt (a retrying row must not stay locked for the whole lease window).
func TestQualificationJobRepo_Save_ClearsLease(t *testing.T) {
	pool := testutil.TestDB(t)
	userID := testutil.SeedUser(t, pool)
	repo := NewQualificationJobRepository(pool)
	ctx := context.Background()

	leadID := uuid.New()
	_, err := pool.Exec(ctx,
		`INSERT INTO leads (id, user_id, channel, contact_name, company, first_message, status)
		 VALUES ($1, $2, 'email', 'Lead', '', 'hi', 'new')`, leadID, userID)
	require.NoError(t, err)

	j, e := NewQualificationJob(leadID, userID, "Alice", ChannelEmail, "x")
	require.NoError(t, e)
	require.NoError(t, repo.EnqueueQualificationJob(ctx, j))

	got, err := repo.ClaimDueQualificationJob(ctx, 5, 300)
	require.NoError(t, err)
	require.NotNil(t, got)

	got.MarkFailed("transient", 5, time.Now().UTC()) // back to pending + backoff
	require.NoError(t, repo.SaveQualificationJob(ctx, got))

	var lockedUntil *time.Time
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT locked_until FROM lead_qualification_jobs WHERE id = $1`, j.ID).Scan(&lockedUntil))
	assert.Nil(t, lockedUntil, "Save must clear the lease")
}
