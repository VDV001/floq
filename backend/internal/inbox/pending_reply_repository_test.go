//go:build integration

package inbox_test

import (
	"context"
	"testing"
	"time"

	"github.com/daniil/floq/internal/inbox"
	"github.com/daniil/floq/internal/testutil"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// seedLeadForUser inserts a minimal lead row tied to userID and returns
// its id. Required because pending_replies.lead_id is a FK with
// ON DELETE CASCADE — the test fixture has to satisfy it. Cleanup is
// covered by the SeedUser CASCADE.
func seedLeadForUser(t *testing.T, pool *pgxpool.Pool, userID uuid.UUID) uuid.UUID {
	t.Helper()
	leadID := uuid.New()
	_, err := pool.Exec(context.Background(),
		`INSERT INTO leads (id, user_id, channel, contact_name, first_message, status, created_at, updated_at)
		 VALUES ($1, $2, 'telegram', 'Test Lead', 'hi', 'new', NOW(), NOW())`,
		leadID, userID)
	require.NoError(t, err, "seed test lead")
	return leadID
}

func TestPendingReplyRepository_SaveAndGetByID(t *testing.T) {
	pool := testutil.TestDB(t)
	userID := testutil.SeedUser(t, pool)
	leadID := seedLeadForUser(t, pool, userID)

	repo := inbox.NewPendingReplyRepository(pool)
	ctx := context.Background()

	pr, err := inbox.NewPendingReply(userID, leadID, inbox.ChannelTelegram, inbox.PendingReplyKindBookingLink, "hello")
	require.NoError(t, err)
	require.NoError(t, repo.Save(ctx, pr))

	got, err := repo.GetByID(ctx, userID, pr.ID)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, pr.ID, got.ID)
	assert.Equal(t, userID, got.UserID)
	assert.Equal(t, leadID, got.LeadID)
	assert.Equal(t, inbox.ChannelTelegram, got.Channel)
	assert.Equal(t, inbox.PendingReplyKindBookingLink, got.Kind)
	assert.Equal(t, "hello", got.Body)
	assert.Equal(t, inbox.PendingReplyStatusPending, got.Status)
	assert.WithinDuration(t, pr.CreatedAt, got.CreatedAt, time.Second)
	assert.Nil(t, got.DecidedAt)
	assert.Nil(t, got.SentAt)
}

func TestPendingReplyRepository_GetByID_ScopedByUser(t *testing.T) {
	pool := testutil.TestDB(t)
	userA := testutil.SeedUser(t, pool)
	userB := testutil.SeedUser(t, pool)
	leadA := seedLeadForUser(t, pool, userA)

	repo := inbox.NewPendingReplyRepository(pool)
	ctx := context.Background()

	pr, err := inbox.NewPendingReply(userA, leadA, inbox.ChannelTelegram, inbox.PendingReplyKindBookingLink, "owned by A")
	require.NoError(t, err)
	require.NoError(t, repo.Save(ctx, pr))

	got, err := repo.GetByID(ctx, userB, pr.ID)
	require.NoError(t, err, "cross-tenant lookup must not error")
	assert.Nil(t, got, "cross-tenant lookup must return nil — never another user's row")
}

func TestPendingReplyRepository_GetByID_NotFound(t *testing.T) {
	pool := testutil.TestDB(t)
	userID := testutil.SeedUser(t, pool)
	repo := inbox.NewPendingReplyRepository(pool)
	ctx := context.Background()

	got, err := repo.GetByID(ctx, userID, uuid.New())
	require.NoError(t, err, "missing row is not an error")
	assert.Nil(t, got)
}

func TestPendingReplyRepository_ListByLead_ScopedByUser(t *testing.T) {
	pool := testutil.TestDB(t)
	userA := testutil.SeedUser(t, pool)
	userB := testutil.SeedUser(t, pool)
	leadA := seedLeadForUser(t, pool, userA)
	leadB := seedLeadForUser(t, pool, userB)

	repo := inbox.NewPendingReplyRepository(pool)
	ctx := context.Background()

	prA1, err := inbox.NewPendingReply(userA, leadA, inbox.ChannelTelegram, inbox.PendingReplyKindBookingLink, "a1")
	require.NoError(t, err)
	require.NoError(t, repo.Save(ctx, prA1))

	prA2, err := inbox.NewPendingReply(userA, leadA, inbox.ChannelTelegram, inbox.PendingReplyKindBookingLink, "a2")
	require.NoError(t, err)
	require.NoError(t, repo.Save(ctx, prA2))

	prB, err := inbox.NewPendingReply(userB, leadB, inbox.ChannelTelegram, inbox.PendingReplyKindBookingLink, "b")
	require.NoError(t, err)
	require.NoError(t, repo.Save(ctx, prB))

	got, err := repo.ListByLead(ctx, userA, leadA)
	require.NoError(t, err)
	assert.Len(t, got, 2)
	for _, p := range got {
		assert.Equal(t, userA, p.UserID, "ListByLead must never leak other-user rows")
		assert.Equal(t, leadA, p.LeadID)
	}

	// Cross-tenant probe: userB asking for leadA returns nothing even if
	// the lead exists (could happen via guessed UUIDs).
	cross, err := repo.ListByLead(ctx, userB, leadA)
	require.NoError(t, err)
	assert.Empty(t, cross, "cross-tenant ListByLead must return empty")
}

func TestPendingReplyRepository_Update_RowMissingReturnsNotFoundSentinel(t *testing.T) {
	pool := testutil.TestDB(t)
	userID := testutil.SeedUser(t, pool)
	leadID := seedLeadForUser(t, pool, userID)
	repo := inbox.NewPendingReplyRepository(pool)
	ctx := context.Background()

	// Construct an entity that was never persisted: Update must
	// surface ErrPendingReplyNotFound so the usecase layer can map
	// it to a uniform 404 (or to ErrPendingReplyAlreadyDecided once
	// the WHERE clause adds the status filter for optimistic lock).
	pr, err := inbox.NewPendingReply(userID, leadID, inbox.ChannelTelegram, inbox.PendingReplyKindBookingLink, "ghost")
	require.NoError(t, err)
	require.NoError(t, pr.Approve(time.Now().UTC(), uuid.New()))

	err = repo.Update(ctx, pr, inbox.PendingReplyStatusPending)
	require.Error(t, err)
	require.ErrorIs(t, err, inbox.ErrPendingReplyNotFound,
		"Update must return ErrPendingReplyNotFound for a missing row, not a bare error")
}

func TestPendingReplyRepository_Update_CrossTenantReturnsNotFoundSentinel(t *testing.T) {
	pool := testutil.TestDB(t)
	userA := testutil.SeedUser(t, pool)
	userB := testutil.SeedUser(t, pool)
	leadA := seedLeadForUser(t, pool, userA)
	repo := inbox.NewPendingReplyRepository(pool)
	ctx := context.Background()

	pr, err := inbox.NewPendingReply(userA, leadA, inbox.ChannelTelegram, inbox.PendingReplyKindBookingLink, "owned by A")
	require.NoError(t, err)
	require.NoError(t, repo.Save(ctx, pr))

	// userB attempting an Update on userA's row must NOT silently
	// no-op; it must surface ErrPendingReplyNotFound (same observable
	// as a missing row — uniform 404 contract).
	stolen := *pr
	stolen.UserID = userB
	require.NoError(t, stolen.Reject(time.Now().UTC(), uuid.New()))
	err = repo.Update(ctx, &stolen, inbox.PendingReplyStatusPending)
	require.ErrorIs(t, err, inbox.ErrPendingReplyNotFound)
}

func TestPendingReplyRepository_Update_OptimisticLock_RejectsWhenStatusMoved(t *testing.T) {
	pool := testutil.TestDB(t)
	userID := testutil.SeedUser(t, pool)
	leadID := seedLeadForUser(t, pool, userID)
	repo := inbox.NewPendingReplyRepository(pool)
	ctx := context.Background()

	pr, err := inbox.NewPendingReply(userID, leadID, inbox.ChannelTelegram, inbox.PendingReplyKindBookingLink, "race")
	require.NoError(t, err)
	require.NoError(t, repo.Save(ctx, pr))

	// Simulate the race: operator A loaded the row at status=pending
	// and is about to push status=approved. Meanwhile operator B
	// already approved it — the persisted row is now status=approved.
	bSnap := *pr
	require.NoError(t, bSnap.Approve(time.Now().UTC(), uuid.New()))
	require.NoError(t, repo.Update(ctx, &bSnap, inbox.PendingReplyStatusPending))

	// Operator A's stale snapshot, still expecting status=pending,
	// must fail the optimistic check rather than overwriting B's
	// decision.
	aSnap := *pr
	require.NoError(t, aSnap.Approve(time.Now().UTC(), uuid.New()))
	err = repo.Update(ctx, &aSnap, inbox.PendingReplyStatusPending)
	require.ErrorIs(t, err, inbox.ErrPendingReplyNotFound,
		"second Update with the same expected-status must fail — the row is no longer pending")
}

func TestPendingReplyRepository_Save_DuplicatePendingReturnsSentinel(t *testing.T) {
	pool := testutil.TestDB(t)
	userID := testutil.SeedUser(t, pool)
	leadID := seedLeadForUser(t, pool, userID)
	repo := inbox.NewPendingReplyRepository(pool)
	ctx := context.Background()

	first, err := inbox.NewPendingReply(userID, leadID, inbox.ChannelTelegram, inbox.PendingReplyKindBookingLink, "book me")
	require.NoError(t, err)
	require.NoError(t, repo.Save(ctx, first))

	second, err := inbox.NewPendingReply(userID, leadID, inbox.ChannelTelegram, inbox.PendingReplyKindBookingLink, "book me")
	require.NoError(t, err)
	err = repo.Save(ctx, second)
	require.Error(t, err, "second Save with identical pending content must fail at the dedup index")
	require.ErrorIs(t, err, inbox.ErrPendingReplyDuplicatePending,
		"23505 on the dedup partial-unique index must translate to ErrPendingReplyDuplicatePending so the usecase can branch")

	listed, err := repo.ListByLead(ctx, userID, leadID)
	require.NoError(t, err)
	assert.Len(t, listed, 1, "dedup index must prevent the duplicate row from landing")
	assert.Equal(t, first.ID, listed[0].ID)
}

func TestPendingReplyRepository_Save_DifferentBodyAllowedAlongside(t *testing.T) {
	pool := testutil.TestDB(t)
	userID := testutil.SeedUser(t, pool)
	leadID := seedLeadForUser(t, pool, userID)
	repo := inbox.NewPendingReplyRepository(pool)
	ctx := context.Background()

	first, err := inbox.NewPendingReply(userID, leadID, inbox.ChannelTelegram, inbox.PendingReplyKindBookingLink, "book me")
	require.NoError(t, err)
	require.NoError(t, repo.Save(ctx, first))

	second, err := inbox.NewPendingReply(userID, leadID, inbox.ChannelTelegram, inbox.PendingReplyKindBookingLink, "book me, please")
	require.NoError(t, err)
	require.NoError(t, repo.Save(ctx, second), "different body must NOT trigger dedup")

	listed, err := repo.ListByLead(ctx, userID, leadID)
	require.NoError(t, err)
	assert.Len(t, listed, 2)
}

func TestPendingReplyRepository_Save_AllowsRePeoposeAfterRejection(t *testing.T) {
	// Partial unique index is scoped to status='pending'. Once the
	// operator rejects the first draft, the same content can be
	// proposed again — the original row is no longer competing for
	// the dedup slot.
	pool := testutil.TestDB(t)
	userID := testutil.SeedUser(t, pool)
	leadID := seedLeadForUser(t, pool, userID)
	repo := inbox.NewPendingReplyRepository(pool)
	ctx := context.Background()

	first, err := inbox.NewPendingReply(userID, leadID, inbox.ChannelTelegram, inbox.PendingReplyKindBookingLink, "book me")
	require.NoError(t, err)
	require.NoError(t, repo.Save(ctx, first))
	require.NoError(t, first.Reject(time.Now().UTC(), uuid.New()))
	require.NoError(t, repo.Update(ctx, first, inbox.PendingReplyStatusPending))

	second, err := inbox.NewPendingReply(userID, leadID, inbox.ChannelTelegram, inbox.PendingReplyKindBookingLink, "book me")
	require.NoError(t, err)
	require.NoError(t, repo.Save(ctx, second),
		"after rejection, the same content must be re-proposable — partial index excludes non-pending rows")
}

func TestPendingReplyRepository_FindPendingByContent_ReturnsExisting(t *testing.T) {
	pool := testutil.TestDB(t)
	userID := testutil.SeedUser(t, pool)
	leadID := seedLeadForUser(t, pool, userID)
	repo := inbox.NewPendingReplyRepository(pool)
	ctx := context.Background()

	first, err := inbox.NewPendingReply(userID, leadID, inbox.ChannelTelegram, inbox.PendingReplyKindBookingLink, "book me")
	require.NoError(t, err)
	require.NoError(t, repo.Save(ctx, first))

	got, err := repo.FindPendingByContent(ctx, userID, leadID, inbox.PendingReplyKindBookingLink, "book me")
	require.NoError(t, err)
	require.NotNil(t, got, "FindPendingByContent must locate the existing pending row")
	assert.Equal(t, first.ID, got.ID)
	assert.Equal(t, inbox.PendingReplyStatusPending, got.Status)
}

func TestPendingReplyRepository_FindPendingByContent_NoMatchReturnsNil(t *testing.T) {
	pool := testutil.TestDB(t)
	userID := testutil.SeedUser(t, pool)
	leadID := seedLeadForUser(t, pool, userID)
	repo := inbox.NewPendingReplyRepository(pool)
	ctx := context.Background()

	got, err := repo.FindPendingByContent(ctx, userID, leadID, inbox.PendingReplyKindBookingLink, "nothing here")
	require.NoError(t, err, "missing match is not an error")
	assert.Nil(t, got)
}

func TestPendingReplyRepository_FindPendingByContent_IgnoresDecidedRows(t *testing.T) {
	// After Reject, the matching row should NOT be returned —
	// FindPendingByContent is the dedup-recovery path, scoped to the
	// same status='pending' slice as the partial unique index.
	pool := testutil.TestDB(t)
	userID := testutil.SeedUser(t, pool)
	leadID := seedLeadForUser(t, pool, userID)
	repo := inbox.NewPendingReplyRepository(pool)
	ctx := context.Background()

	pr, err := inbox.NewPendingReply(userID, leadID, inbox.ChannelTelegram, inbox.PendingReplyKindBookingLink, "book me")
	require.NoError(t, err)
	require.NoError(t, repo.Save(ctx, pr))
	require.NoError(t, pr.Reject(time.Now().UTC(), uuid.New()))
	require.NoError(t, repo.Update(ctx, pr, inbox.PendingReplyStatusPending))

	got, err := repo.FindPendingByContent(ctx, userID, leadID, inbox.PendingReplyKindBookingLink, "book me")
	require.NoError(t, err)
	assert.Nil(t, got, "FindPendingByContent must skip rejected/sent/approved rows — only the pending slice is dedup-relevant")
}

func TestPendingReplyRepository_FindPendingByContent_ScopedByUser(t *testing.T) {
	pool := testutil.TestDB(t)
	userA := testutil.SeedUser(t, pool)
	userB := testutil.SeedUser(t, pool)
	leadA := seedLeadForUser(t, pool, userA)
	repo := inbox.NewPendingReplyRepository(pool)
	ctx := context.Background()

	pr, err := inbox.NewPendingReply(userA, leadA, inbox.ChannelTelegram, inbox.PendingReplyKindBookingLink, "owned by A")
	require.NoError(t, err)
	require.NoError(t, repo.Save(ctx, pr))

	got, err := repo.FindPendingByContent(ctx, userB, leadA, inbox.PendingReplyKindBookingLink, "owned by A")
	require.NoError(t, err, "cross-tenant lookup must not error")
	assert.Nil(t, got, "cross-tenant FindPendingByContent must return nil — never another user's row")
}

func TestPendingReplyRepository_Update_PersistsDecidedBy(t *testing.T) {
	// After Approve(at, by) + Update, GetByID returns the row with
	// DecidedBy populated. Pins migration 032 + repo round-trip.
	pool := testutil.TestDB(t)
	userID := testutil.SeedUser(t, pool)
	leadID := seedLeadForUser(t, pool, userID)
	repo := inbox.NewPendingReplyRepository(pool)
	ctx := context.Background()

	pr, err := inbox.NewPendingReply(userID, leadID, inbox.ChannelTelegram, inbox.PendingReplyKindBookingLink, "body")
	require.NoError(t, err)
	require.NoError(t, repo.Save(ctx, pr))
	// Newly-created pending row has no decision yet — both fields nil.
	got, err := repo.GetByID(ctx, userID, pr.ID)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Nil(t, got.DecidedBy, "freshly Saved pending row must have nil DecidedBy")

	operator := testutil.SeedUser(t, pool)
	require.NoError(t, pr.Approve(time.Now().UTC(), operator))
	require.NoError(t, repo.Update(ctx, pr, inbox.PendingReplyStatusPending))

	got, err = repo.GetByID(ctx, userID, pr.ID)
	require.NoError(t, err)
	require.NotNil(t, got.DecidedBy, "Update must persist DecidedBy")
	assert.Equal(t, operator, *got.DecidedBy)
}

func TestPendingReplyRepository_Update_PersistsStatusAndTimestamps(t *testing.T) {
	pool := testutil.TestDB(t)
	userID := testutil.SeedUser(t, pool)
	leadID := seedLeadForUser(t, pool, userID)
	repo := inbox.NewPendingReplyRepository(pool)
	ctx := context.Background()

	pr, err := inbox.NewPendingReply(userID, leadID, inbox.ChannelTelegram, inbox.PendingReplyKindBookingLink, "body")
	require.NoError(t, err)
	require.NoError(t, repo.Save(ctx, pr))

	decidedAt := time.Now().UTC().Truncate(time.Microsecond)
	require.NoError(t, pr.Approve(decidedAt, uuid.New()))
	require.NoError(t, repo.Update(ctx, pr, inbox.PendingReplyStatusPending))

	got, err := repo.GetByID(ctx, userID, pr.ID)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, inbox.PendingReplyStatusApproved, got.Status)
	require.NotNil(t, got.DecidedAt)
	assert.WithinDuration(t, decidedAt, *got.DecidedAt, time.Second)
	assert.Nil(t, got.SentAt)

	sentAt := time.Now().UTC().Truncate(time.Microsecond)
	require.NoError(t, pr.MarkSent(sentAt))
	require.NoError(t, repo.Update(ctx, pr, inbox.PendingReplyStatusApproved))

	got2, err := repo.GetByID(ctx, userID, pr.ID)
	require.NoError(t, err)
	require.NotNil(t, got2)
	assert.Equal(t, inbox.PendingReplyStatusSent, got2.Status)
	require.NotNil(t, got2.SentAt)
	assert.WithinDuration(t, sentAt, *got2.SentAt, time.Second)
}
