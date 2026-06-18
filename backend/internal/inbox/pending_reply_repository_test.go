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

func TestPendingReplyRepository_PersistsInputSeverity(t *testing.T) {
	pool := testutil.TestDB(t)
	userID := testutil.SeedUser(t, pool)
	leadID := seedLeadForUser(t, pool, userID)

	repo := inbox.NewPendingReplyRepository(pool)
	ctx := context.Background()

	// A reply triggered by a warn-flagged inbound message must carry that
	// verdict all the way through persistence so the dispatch gate sees it.
	pr, err := inbox.NewClassifiedPendingReply(userID, leadID, inbox.ChannelTelegram, inbox.PendingReplyKindBookingLink, "book me", inbox.SeverityWarn)
	require.NoError(t, err)
	require.NoError(t, repo.Save(ctx, pr))

	got, err := repo.GetByID(ctx, userID, pr.ID)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, inbox.SeverityWarn, got.InputSeverity, "GetByID must round-trip input_severity")

	// The list path the operator queue uses must preserve it too.
	list, err := repo.ListByLead(ctx, userID, leadID)
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Equal(t, inbox.SeverityWarn, list[0].InputSeverity, "ListByLead must round-trip input_severity")
}

func TestPendingReplyRepository_CountPendingByKind(t *testing.T) {
	pool := testutil.TestDB(t)
	userA := testutil.SeedUser(t, pool)
	userB := testutil.SeedUser(t, pool)
	leadA := seedLeadForUser(t, pool, userA)
	leadB := seedLeadForUser(t, pool, userB)
	repo := inbox.NewPendingReplyRepository(pool)
	ctx := context.Background()

	// 2 pending for userA + 1 for userB — the queue-depth metric is an
	// aggregate across tenants (no per-user label on the public endpoint).
	for i, body := range []string{"a-1", "a-2"} {
		pr, err := inbox.NewPendingReply(userA, leadA, inbox.ChannelTelegram, inbox.PendingReplyKindBookingLink, body)
		require.NoError(t, err, "seed %d", i)
		require.NoError(t, repo.Save(ctx, pr))
	}
	prB, err := inbox.NewPendingReply(userB, leadB, inbox.ChannelTelegram, inbox.PendingReplyKindBookingLink, "b-1")
	require.NoError(t, err)
	require.NoError(t, repo.Save(ctx, prB))

	// An already-sent row must be excluded by the status='pending' filter.
	_, err = pool.Exec(ctx,
		`INSERT INTO pending_replies (id, user_id, lead_id, channel, kind, body, status, created_at)
		 VALUES ($1, $2, $3, 'telegram', 'booking_link', 'done', 'sent', NOW())`,
		uuid.New(), userA, leadA)
	require.NoError(t, err)

	depths, err := repo.CountPendingByKind(ctx)
	require.NoError(t, err)
	assert.Equal(t, 3, depths["booking_link"], "aggregate pending across users; non-pending excluded")
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

	operatorB := testutil.SeedUser(t, pool)
	operatorA := testutil.SeedUser(t, pool)

	// Simulate the race: operator A loaded the row at status=pending
	// and is about to push status=approved. Meanwhile operator B
	// already approved it — the persisted row is now status=approved.
	bSnap := *pr
	require.NoError(t, bSnap.Approve(time.Now().UTC(), operatorB))
	require.NoError(t, repo.Update(ctx, &bSnap, inbox.PendingReplyStatusPending))

	// Operator A's stale snapshot, still expecting status=pending,
	// must fail the optimistic check rather than overwriting B's
	// decision.
	aSnap := *pr
	require.NoError(t, aSnap.Approve(time.Now().UTC(), operatorA))
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
	operator := testutil.SeedUser(t, pool)
	require.NoError(t, first.Reject(time.Now().UTC(), operator))
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
	operator := testutil.SeedUser(t, pool)
	require.NoError(t, pr.Reject(time.Now().UTC(), operator))
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

	operator := testutil.SeedUser(t, pool)
	decidedAt := time.Now().UTC().Truncate(time.Microsecond)
	require.NoError(t, pr.Approve(decidedAt, operator))
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

// --- UpdateBody (#48) ---

func TestPendingReplyRepository_UpdateBody_PersistsBodyOnly(t *testing.T) {
	// Backfills integration coverage for the body-only column write
	// shipped alongside the usecase RED commit (#48 plan put the SQL in
	// before the integration test, so this is honest backfill — not a
	// fresh TDD RED).
	pool := testutil.TestDB(t)
	userID := testutil.SeedUser(t, pool)
	leadID := seedLeadForUser(t, pool, userID)

	repo := inbox.NewPendingReplyRepository(pool)
	ctx := context.Background()

	pr, err := inbox.NewPendingReply(userID, leadID, inbox.ChannelTelegram, inbox.PendingReplyKindBookingLink, "original")
	require.NoError(t, err)
	require.NoError(t, repo.Save(ctx, pr))

	pr.Body = "edited body"
	require.NoError(t, repo.UpdateBody(ctx, pr, inbox.PendingReplyStatusPending))

	got, err := repo.GetByID(ctx, userID, pr.ID)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, "edited body", got.Body)
	// Status stays Pending; decided_* columns stay nil — UpdateBody
	// only touches body, never the decision columns.
	assert.Equal(t, inbox.PendingReplyStatusPending, got.Status)
	assert.Nil(t, got.DecidedAt)
	assert.Nil(t, got.DecidedBy)
	assert.Nil(t, got.SentAt)
}

func TestPendingReplyRepository_UpdateBody_OptimisticLockRejectsWhenStatusMoved(t *testing.T) {
	pool := testutil.TestDB(t)
	userID := testutil.SeedUser(t, pool)
	leadID := seedLeadForUser(t, pool, userID)

	repo := inbox.NewPendingReplyRepository(pool)
	ctx := context.Background()

	pr, err := inbox.NewPendingReply(userID, leadID, inbox.ChannelTelegram, inbox.PendingReplyKindBookingLink, "original")
	require.NoError(t, err)
	require.NoError(t, repo.Save(ctx, pr))

	// Simulate concurrent approve before edit lands: directly move
	// status to approved in the DB.
	_, err = pool.Exec(ctx,
		`UPDATE pending_replies SET status = 'approved' WHERE id = $1`, pr.ID)
	require.NoError(t, err)

	pr.Body = "edit too late"
	err = repo.UpdateBody(ctx, pr, inbox.PendingReplyStatusPending)
	require.ErrorIs(t, err, inbox.ErrPendingReplyNotFound)

	// Verify body was not silently written.
	got, _ := repo.GetByID(ctx, userID, pr.ID)
	require.NotNil(t, got)
	assert.Equal(t, "original", got.Body)
}

func TestPendingReplyRepository_UpdateBody_CrossTenantReturnsNotFound(t *testing.T) {
	pool := testutil.TestDB(t)
	owner := testutil.SeedUser(t, pool)
	attacker := testutil.SeedUser(t, pool)
	leadID := seedLeadForUser(t, pool, owner)

	repo := inbox.NewPendingReplyRepository(pool)
	ctx := context.Background()

	pr, err := inbox.NewPendingReply(owner, leadID, inbox.ChannelTelegram, inbox.PendingReplyKindBookingLink, "victim body")
	require.NoError(t, err)
	require.NoError(t, repo.Save(ctx, pr))

	// Attacker tries to edit with their UserID — repo scopes by user_id
	// so the row is invisible.
	pr.UserID = attacker
	pr.Body = "tampered"
	err = repo.UpdateBody(ctx, pr, inbox.PendingReplyStatusPending)
	require.ErrorIs(t, err, inbox.ErrPendingReplyNotFound)

	// Restore owner perspective and verify untouched.
	pr.UserID = owner
	got, _ := repo.GetByID(ctx, owner, pr.ID)
	require.NotNil(t, got)
	assert.Equal(t, "victim body", got.Body)
}

// seedRichLead inserts a lead with caller-controlled snippet fields so
// the ListPendingByUser tests can assert the JOIN payload (contact +
// company + channel + identifiers). Keeps seedLeadForUser as the
// minimal helper for the rest of the suite.
func seedRichLead(t *testing.T, pool *pgxpool.Pool, userID uuid.UUID, channel, contactName, company string, tgChatID *int64, email *string) uuid.UUID {
	t.Helper()
	leadID := uuid.New()
	_, err := pool.Exec(context.Background(),
		`INSERT INTO leads (id, user_id, channel, contact_name, company, first_message, status, telegram_chat_id, email_address, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, 'hi', 'new', $6, $7, NOW(), NOW())`,
		leadID, userID, channel, contactName, company, tgChatID, email)
	require.NoError(t, err, "seed rich test lead")
	return leadID
}

func TestPendingReplyRepository_ListPendingByUser_ReturnsJoinedSnippet(t *testing.T) {
	pool := testutil.TestDB(t)
	userID := testutil.SeedUser(t, pool)

	tgChatID := int64(1234567)
	email := "lead@example.com"
	tgLead := seedRichLead(t, pool, userID, "telegram", "Иван Петров", "ACME", &tgChatID, nil)
	mailLead := seedRichLead(t, pool, userID, "email", "Jane Doe", "Globex", nil, &email)

	repo := inbox.NewPendingReplyRepository(pool)
	ctx := context.Background()

	tgPR, err := inbox.NewPendingReply(userID, tgLead, inbox.ChannelTelegram, inbox.PendingReplyKindBookingLink, "tg draft")
	require.NoError(t, err)
	require.NoError(t, repo.Save(ctx, tgPR))

	mailPR, err := inbox.NewPendingReply(userID, mailLead, inbox.ChannelEmail, inbox.PendingReplyKindBookingLink, "email draft")
	require.NoError(t, err)
	require.NoError(t, repo.Save(ctx, mailPR))

	got, err := repo.ListPendingByUser(ctx, userID)
	require.NoError(t, err)
	require.Len(t, got, 2, "both pending rows must surface")

	byReply := map[uuid.UUID]*inbox.PendingReplyWithLead{}
	for _, row := range got {
		require.NotNil(t, row, "no nil entries in the result slice")
		require.NotNil(t, row.Reply, "Reply must be populated")
		byReply[row.Reply.ID] = row
	}

	tgRow := byReply[tgPR.ID]
	require.NotNil(t, tgRow, "telegram pending row must be present")
	assert.Equal(t, "Иван Петров", tgRow.Lead.ContactName)
	assert.Equal(t, "ACME", tgRow.Lead.Company)
	assert.Equal(t, inbox.ChannelTelegram, tgRow.Lead.Channel)
	require.NotNil(t, tgRow.Lead.TelegramChatID)
	assert.Equal(t, int64(1234567), *tgRow.Lead.TelegramChatID)
	assert.Nil(t, tgRow.Lead.EmailAddress, "telegram lead has no email — column must surface as nil")

	mailRow := byReply[mailPR.ID]
	require.NotNil(t, mailRow, "email pending row must be present")
	assert.Equal(t, "Jane Doe", mailRow.Lead.ContactName)
	assert.Equal(t, "Globex", mailRow.Lead.Company)
	assert.Equal(t, inbox.ChannelEmail, mailRow.Lead.Channel)
	assert.Nil(t, mailRow.Lead.TelegramChatID)
	require.NotNil(t, mailRow.Lead.EmailAddress)
	assert.Equal(t, "lead@example.com", *mailRow.Lead.EmailAddress)
}

func TestPendingReplyRepository_ListPendingByUser_FiltersDecidedRows(t *testing.T) {
	pool := testutil.TestDB(t)
	userID := testutil.SeedUser(t, pool)
	leadID := seedLeadForUser(t, pool, userID)

	repo := inbox.NewPendingReplyRepository(pool)
	ctx := context.Background()

	// Pending — must surface.
	pending, err := inbox.NewPendingReply(userID, leadID, inbox.ChannelTelegram, inbox.PendingReplyKindBookingLink, "still pending")
	require.NoError(t, err)
	require.NoError(t, repo.Save(ctx, pending))

	// Approved — must be filtered.
	approved, err := inbox.NewPendingReply(userID, leadID, inbox.ChannelTelegram, inbox.PendingReplyKindBookingLink, "already approved")
	require.NoError(t, err)
	require.NoError(t, repo.Save(ctx, approved))
	_, err = pool.Exec(ctx,
		`UPDATE pending_replies SET status = 'approved' WHERE id = $1`, approved.ID)
	require.NoError(t, err)

	// Sent — must be filtered.
	sent, err := inbox.NewPendingReply(userID, leadID, inbox.ChannelTelegram, inbox.PendingReplyKindBookingLink, "already sent")
	require.NoError(t, err)
	require.NoError(t, repo.Save(ctx, sent))
	_, err = pool.Exec(ctx,
		`UPDATE pending_replies SET status = 'sent' WHERE id = $1`, sent.ID)
	require.NoError(t, err)

	// Rejected — must be filtered.
	rejected, err := inbox.NewPendingReply(userID, leadID, inbox.ChannelTelegram, inbox.PendingReplyKindBookingLink, "already rejected")
	require.NoError(t, err)
	require.NoError(t, repo.Save(ctx, rejected))
	_, err = pool.Exec(ctx,
		`UPDATE pending_replies SET status = 'rejected' WHERE id = $1`, rejected.ID)
	require.NoError(t, err)

	got, err := repo.ListPendingByUser(ctx, userID)
	require.NoError(t, err)
	require.Len(t, got, 1, "only the pending row surfaces — decided rows are out of operator queue scope")
	assert.Equal(t, pending.ID, got[0].Reply.ID)
}

func TestPendingReplyRepository_ListPendingByUser_ScopedByUser(t *testing.T) {
	pool := testutil.TestDB(t)
	userA := testutil.SeedUser(t, pool)
	userB := testutil.SeedUser(t, pool)
	leadA := seedLeadForUser(t, pool, userA)

	repo := inbox.NewPendingReplyRepository(pool)
	ctx := context.Background()

	prA, err := inbox.NewPendingReply(userA, leadA, inbox.ChannelTelegram, inbox.PendingReplyKindBookingLink, "owned by A")
	require.NoError(t, err)
	require.NoError(t, repo.Save(ctx, prA))

	// userB has no leads and no pending rows.
	got, err := repo.ListPendingByUser(ctx, userB)
	require.NoError(t, err, "cross-tenant list must not error — empty is the right shape")
	assert.Empty(t, got, "userB must never see userA's pending rows")
}

func TestPendingReplyRepository_ListPendingByUser_OrdersByCreatedAtDesc(t *testing.T) {
	pool := testutil.TestDB(t)
	userID := testutil.SeedUser(t, pool)
	leadID := seedLeadForUser(t, pool, userID)

	repo := inbox.NewPendingReplyRepository(pool)
	ctx := context.Background()

	// Insert three rows with distinct created_at to pin ordering. Use
	// direct SQL so we can control the timestamp — NewPendingReply
	// stamps time.Now() which would race within a single test.
	t0 := time.Now().UTC().Add(-3 * time.Hour)
	t1 := t0.Add(time.Hour)
	t2 := t1.Add(time.Hour)
	ids := []uuid.UUID{uuid.New(), uuid.New(), uuid.New()}
	for i, ts := range []time.Time{t0, t1, t2} {
		_, err := pool.Exec(ctx,
			`INSERT INTO pending_replies (id, user_id, lead_id, channel, kind, body, status, created_at)
			 VALUES ($1, $2, $3, 'telegram', 'booking_link', $4, 'pending', $5)`,
			ids[i], userID, leadID, "body"+ts.Format("150405"), ts)
		require.NoError(t, err)
	}

	got, err := repo.ListPendingByUser(ctx, userID)
	require.NoError(t, err)
	require.Len(t, got, 3)
	// Newest (t2) first, oldest (t0) last.
	assert.Equal(t, ids[2], got[0].Reply.ID, "newest pending row must be first")
	assert.Equal(t, ids[1], got[1].Reply.ID)
	assert.Equal(t, ids[0], got[2].Reply.ID, "oldest pending row must be last")
}

func TestPendingReplyRepository_ListPendingByUser_EmptyReturnsEmptySlice(t *testing.T) {
	pool := testutil.TestDB(t)
	userID := testutil.SeedUser(t, pool)

	repo := inbox.NewPendingReplyRepository(pool)
	ctx := context.Background()

	got, err := repo.ListPendingByUser(ctx, userID)
	require.NoError(t, err)
	require.NotNil(t, got, "empty result must be a non-nil empty slice — callers iterate without nil-check")
	assert.Empty(t, got)
}
