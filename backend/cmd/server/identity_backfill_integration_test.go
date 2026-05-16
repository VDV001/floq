//go:build integration

package main

import (
	"context"
	"testing"

	"github.com/daniil/floq/internal/leads"
	"github.com/daniil/floq/internal/testutil"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestIdentityBackfill_3ChannelMerge wires the full stack — SQL
// IdentityRepository + deterministic Resolver + SQL-backed
// BackfillSource — and confirms the multi-source dedup promise: a
// lead and a prospect that share an email collapse to one Identity
// and stay collapsed across re-runs.
func TestIdentityBackfill_3ChannelMerge(t *testing.T) {
	pool := testutil.TestDB(t)
	userID := testutil.SeedUser(t, pool)
	ctx := context.Background()

	leadID := uuid.New()
	_, err := pool.Exec(ctx,
		`INSERT INTO leads (id, user_id, channel, contact_name, first_message, email_address)
		 VALUES ($1, $2, 'email', 'Alice', 'hi', 'alice@acme.com')`,
		leadID, userID)
	require.NoError(t, err)

	prospectID := uuid.New()
	_, err = pool.Exec(ctx,
		`INSERT INTO prospects (id, user_id, name, email, telegram_username)
		 VALUES ($1, $2, 'Alice', 'alice@acme.com', 'alice_bot')`,
		prospectID, userID)
	require.NoError(t, err)

	identityRepo := leads.NewIdentityRepository(pool)
	resolver := leads.NewIdentityResolver(identityRepo)
	source := newSQLBackfillSource(pool)
	backfill := leads.NewIdentityBackfill(source, resolver, identityRepo)

	require.NoError(t, backfill.Run(ctx))

	var identityCount int
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM identities WHERE user_id = $1`, userID).Scan(&identityCount))
	assert.Equal(t, 1, identityCount, "lead + prospect with shared email must collapse into one Identity")

	var leadLinks int
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM lead_identities WHERE lead_id = $1`, leadID).Scan(&leadLinks))
	assert.Equal(t, 1, leadLinks)

	var prospectLinks int
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM prospect_identities WHERE prospect_id = $1`, prospectID).Scan(&prospectLinks))
	assert.Equal(t, 1, prospectLinks)

	// Re-run must be a no-op.
	require.NoError(t, backfill.Run(ctx))
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM identities WHERE user_id = $1`, userID).Scan(&identityCount))
	assert.Equal(t, 1, identityCount, "re-run must not duplicate the Identity")
}

func TestIdentityLinkerAdapter_LinksLeadAndProspect(t *testing.T) {
	pool := testutil.TestDB(t)
	userID := testutil.SeedUser(t, pool)
	ctx := context.Background()

	leadID := uuid.New()
	_, err := pool.Exec(ctx,
		`INSERT INTO leads (id, user_id, channel, contact_name, first_message)
		 VALUES ($1, $2, 'email', 'Alice', 'hi')`,
		leadID, userID)
	require.NoError(t, err)

	prospectID := uuid.New()
	_, err = pool.Exec(ctx,
		`INSERT INTO prospects (id, user_id, name, email) VALUES ($1, $2, 'Alice', 'alice@acme.com')`,
		prospectID, userID)
	require.NoError(t, err)

	identityRepo := leads.NewIdentityRepository(pool)
	resolver := leads.NewIdentityResolver(identityRepo)
	adapter := newIdentityLinkerAdapter(resolver, identityRepo)

	require.NoError(t, adapter.LinkLeadToIdentity(ctx, userID, leadID, "alice@acme.com", "", ""))
	require.NoError(t, adapter.LinkProspectToIdentity(ctx, userID, prospectID, "alice@acme.com", "", ""))

	var identityCount int
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM identities WHERE user_id = $1`, userID).Scan(&identityCount))
	assert.Equal(t, 1, identityCount, "both linker calls must share the resolved Identity")
}
