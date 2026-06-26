//go:build integration

package audit_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/daniil/floq/internal/audit"
	"github.com/daniil/floq/internal/audit/domain"
	"github.com/daniil/floq/internal/testutil"
)

func mustEntry(t *testing.T, p domain.EntryParams) *domain.Entry {
	t.Helper()
	e, err := domain.NewEntry(p)
	require.NoError(t, err)
	return e
}

func TestRepository_SaveEmpty(t *testing.T) {
	pool := testutil.TestDB(t)
	repo := audit.NewRepository(pool)
	require.NoError(t, repo.Save(context.Background(), nil))
	require.NoError(t, repo.Save(context.Background(), []*domain.Entry{}))
}

func TestRepository_SaveSingleSuccessRow(t *testing.T) {
	pool := testutil.TestDB(t)
	userID := testutil.SeedUser(t, pool)
	repo := audit.NewRepository(pool)
	ctx := context.Background()

	e := mustEntry(t, domain.EntryParams{
		UserID:       userID,
		RequestType:  domain.RequestTypeQualification,
		Provider:     "openai",
		Model:        "gpt-4o-mini",
		InputTokens:  120,
		OutputTokens: 80,
		CostUSDMicro: 30_000,
		LatencyMS:    412,
		Status:       domain.StatusSuccess,
	})

	require.NoError(t, repo.Save(ctx, []*domain.Entry{e}))

	var (
		dbInput, dbOutput, dbTotal int
		dbCost                     int64
		dbStatus, dbProvider       string
		dbErrMsg                   *string
		dbLead, dbProspect         *uuid.UUID
		dbCreated                  time.Time
	)
	err := pool.QueryRow(ctx,
		`SELECT input_tokens, output_tokens, total_tokens, cost_usd_micro, status, provider, error_message, lead_id, prospect_id, created_at
		   FROM audit_log WHERE id = $1`, e.ID).
		Scan(&dbInput, &dbOutput, &dbTotal, &dbCost, &dbStatus, &dbProvider, &dbErrMsg, &dbLead, &dbProspect, &dbCreated)
	require.NoError(t, err)
	assert.Equal(t, 120, dbInput)
	assert.Equal(t, 80, dbOutput)
	assert.Equal(t, 200, dbTotal)
	assert.Equal(t, int64(30_000), dbCost)
	assert.Equal(t, "success", dbStatus)
	assert.Equal(t, "openai", dbProvider)
	assert.Nil(t, dbErrMsg, "error_message must be NULL on success rows")
	assert.Nil(t, dbLead, "lead_id NULL when not attributed")
	assert.Nil(t, dbProspect, "prospect_id NULL when not attributed")
	assert.WithinDuration(t, e.CreatedAt, dbCreated, time.Second)
}

func TestRepository_SaveErrorRow(t *testing.T) {
	pool := testutil.TestDB(t)
	userID := testutil.SeedUser(t, pool)
	repo := audit.NewRepository(pool)
	ctx := context.Background()

	e := mustEntry(t, domain.EntryParams{
		UserID:       userID,
		RequestType:  domain.RequestTypeDraftReply,
		Provider:     "anthropic",
		Model:        "claude-3-5-haiku-20241022",
		InputTokens:  0,
		OutputTokens: 0,
		CostUSDMicro: 0,
		LatencyMS:    8000,
		Status:       domain.StatusError,
		ErrorMessage: "anthropic 429 rate_limit_exceeded",
	})
	require.NoError(t, repo.Save(ctx, []*domain.Entry{e}))

	var dbStatus string
	var dbErr *string
	err := pool.QueryRow(ctx, `SELECT status, error_message FROM audit_log WHERE id = $1`, e.ID).
		Scan(&dbStatus, &dbErr)
	require.NoError(t, err)
	assert.Equal(t, "error", dbStatus)
	require.NotNil(t, dbErr)
	assert.Equal(t, "anthropic 429 rate_limit_exceeded", *dbErr)
}

func TestRepository_SaveBatchAtomic(t *testing.T) {
	pool := testutil.TestDB(t)
	userID := testutil.SeedUser(t, pool)
	repo := audit.NewRepository(pool)
	ctx := context.Background()

	batch := make([]*domain.Entry, 0, 5)
	for i := 0; i < 5; i++ {
		batch = append(batch, mustEntry(t, domain.EntryParams{
			UserID:       userID,
			RequestType:  domain.RequestTypeImageAnalysis,
			Provider:     "openai",
			Model:        "gpt-4o-mini",
			InputTokens:  10 * (i + 1),
			OutputTokens: 5 * (i + 1),
			CostUSDMicro: int64(100 * (i + 1)),
			LatencyMS:    200,
			Status:       domain.StatusSuccess,
		}))
	}
	require.NoError(t, repo.Save(ctx, batch))

	var count int
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM audit_log WHERE user_id = $1 AND request_type = 'image_analysis'`, userID).
		Scan(&count))
	assert.Equal(t, 5, count, "all batch rows must commit atomically")
}

func TestRepository_LeadFKSetNullOnDelete(t *testing.T) {
	pool := testutil.TestDB(t)
	userID := testutil.SeedUser(t, pool)
	repo := audit.NewRepository(pool)
	ctx := context.Background()

	// Seed a lead row referenced by the audit entry.
	leadID := uuid.New()
	_, err := pool.Exec(ctx,
		`INSERT INTO leads (id, user_id, contact_name, channel, email_address, first_message)
		   VALUES ($1, $2, $3, $4, $5, $6)`,
		leadID, userID, "alice", "email", "alice@acme.com", "hi")
	require.NoError(t, err)

	e := mustEntry(t, domain.EntryParams{
		UserID:       userID,
		LeadID:       &leadID,
		RequestType:  domain.RequestTypeQualification,
		Provider:     "openai",
		Model:        "gpt-4o-mini",
		InputTokens:  10,
		OutputTokens: 5,
		CostUSDMicro: 100,
		LatencyMS:    100,
		Status:       domain.StatusSuccess,
	})
	require.NoError(t, repo.Save(ctx, []*domain.Entry{e}))

	// Delete the lead — audit row must survive with NULL lead_id.
	_, err = pool.Exec(ctx, `DELETE FROM leads WHERE id = $1`, leadID)
	require.NoError(t, err)

	var dbLead *uuid.UUID
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT lead_id FROM audit_log WHERE id = $1`, e.ID).Scan(&dbLead))
	assert.Nil(t, dbLead, "lead_id must be SET NULL after lead deletion")
}
