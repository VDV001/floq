//go:build integration

package main

import (
	"context"
	"testing"
	"time"

	"github.com/daniil/floq/internal/db"
	"github.com/daniil/floq/internal/prospects"
	prospectsdomain "github.com/daniil/floq/internal/prospects/domain"
	"github.com/daniil/floq/internal/testutil"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestConvertToLead_GrantsInboundConsent verifies that converting a prospect to
// a lead via the inbox boundary (an inbound reply) also records obtained
// consent (source inbound_reply) — the prospect engaged, so the cold-contact
// override is no longer needed. Atomic with the conversion.
func TestConvertToLead_GrantsInboundConsent(t *testing.T) {
	pool := testutil.TestDB(t)
	userID := testutil.SeedUser(t, pool)
	ctx := context.Background()

	repo := prospects.NewRepository(pool)
	adapter := newProspectRepoAdapter(repo, db.NewTxManager(pool))

	p, err := prospectsdomain.NewProspect(userID, "Alice", "Acme", "CEO", "alice@acme.com", "manual")
	require.NoError(t, err)
	require.Equal(t, prospectsdomain.ConsentStatusNone, p.Consent.Status)
	require.NoError(t, repo.CreateProspect(ctx, p))

	now := time.Now().UTC().Truncate(time.Microsecond)
	leadID := uuid.New()
	_, err = pool.Exec(ctx,
		`INSERT INTO leads (id, user_id, channel, contact_name, company, first_message, status, created_at, updated_at)
		 VALUES ($1, $2, 'email', 'Alice', 'Acme', 'msg', 'new', $3, $3)`,
		leadID, userID, now)
	require.NoError(t, err)

	require.NoError(t, adapter.ConvertToLead(ctx, p.ID, leadID))

	got, err := repo.GetProspect(ctx, p.ID)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, prospectsdomain.ProspectStatusConverted, got.Status)
	assert.Equal(t, prospectsdomain.ConsentStatusObtained, got.Consent.Status, "inbound reply should grant consent")
	assert.Equal(t, "inbound_reply", got.Consent.Source)
}
