//go:build integration

package audit_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/daniil/floq/internal/ai"
	"github.com/daniil/floq/internal/ai/providers"
	"github.com/daniil/floq/internal/audit"
	"github.com/daniil/floq/internal/audit/domain"
	"github.com/daniil/floq/internal/testutil"
)

// TestAcceptance_ImageAttachmentCostLoggedToAuditLog satisfies the
// final acceptance criterion of issue #25: a real image-analysis call
// (provider stub stands in for the OpenAI vision endpoint, everything
// else is production wiring) must leave a cost row in audit_log
// attributed to the user_id and lead_id of the originating inbound
// email. End-to-end through the live database — this is the contract
// the rest of the audit pipeline exists to enforce.
type fakeProvider struct {
	name       string
	visionResp *ai.CompletionResult
	visionErr  error
}

func (f *fakeProvider) Name() string { return f.name }

func (f *fakeProvider) Complete(_ context.Context, _ ai.CompletionRequest) (*ai.CompletionResult, error) {
	return &ai.CompletionResult{Text: "", Model: f.name}, nil
}

func (f *fakeProvider) AnalyzeImage(_ context.Context, _ []byte, _, _ string) (*ai.CompletionResult, error) {
	return f.visionResp, f.visionErr
}

func TestAcceptance_ImageAttachmentCostLoggedToAuditLog(t *testing.T) {
	pool := testutil.TestDB(t)
	userID := testutil.SeedUser(t, pool)
	ctx := context.Background()

	// Seed a lead so the FK in audit_log holds.
	leadID := uuid.New()
	_, err := pool.Exec(ctx,
		`INSERT INTO leads (id, user_id, contact_name, channel, email_address, first_message)
		   VALUES ($1, $2, $3, $4, $5, $6)`,
		leadID, userID, "alice", "email", "alice@acme.com", "see attached screenshot")
	require.NoError(t, err)

	// Provider stub mimics OpenAI vision: real usage numbers so the
	// pricing math produces a predictable non-zero cost.
	stub := &fakeProvider{
		name: "openai",
		visionResp: &ai.CompletionResult{
			Text:  "Backlog: fix login bug",
			Usage: ai.TokenUsage{InputTokens: 1000, OutputTokens: 500},
			Model: "gpt-4o-mini",
		},
	}

	repo := audit.NewRepository(pool)
	recorder := audit.NewAsyncRecorder(repo,
		audit.WithBatchSize(1),
		audit.WithFlushInterval(50*time.Millisecond))
	recorder.Start()
	t.Cleanup(func() {
		stopCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = recorder.Stop(stopCtx)
	})

	wrapped := audit.NewRecordingProvider(stub, recorder, nil)
	aiClient := ai.NewAIClient(wrapped, "", "", "", "", "")

	imgCtx := domain.ContextWithCallMeta(ctx, domain.CallMeta{
		UserID:      userID,
		LeadID:      &leadID,
		RequestType: domain.RequestTypeImageAnalysis,
	})

	text, err := aiClient.AnalyzeImage(imgCtx, []byte("png-bytes"), "image/png", "Transcribe the screenshot")
	require.NoError(t, err)
	assert.Equal(t, "Backlog: fix login bug", text)

	// Async path: poll until the audit_log row lands.
	var rowCount int
	require.Eventually(t, func() bool {
		err := pool.QueryRow(ctx,
			`SELECT COUNT(*) FROM audit_log WHERE user_id = $1 AND request_type = 'image_analysis'`,
			userID).Scan(&rowCount)
		return err == nil && rowCount == 1
	}, 2*time.Second, 25*time.Millisecond, "audit_log row never appeared")

	var (
		dbUserID                    uuid.UUID
		dbLeadID                    *uuid.UUID
		dbRequestType, dbProvider   string
		dbModel                     string
		dbInputTokens, dbOutputTokens int
		dbCost                      int64
		dbStatus                    string
		dbLatency                   int
	)
	err = pool.QueryRow(ctx,
		`SELECT user_id, lead_id, request_type, provider, model, input_tokens, output_tokens, cost_usd_micro, status, latency_ms
		   FROM audit_log WHERE user_id = $1`, userID).
		Scan(&dbUserID, &dbLeadID, &dbRequestType, &dbProvider, &dbModel, &dbInputTokens, &dbOutputTokens, &dbCost, &dbStatus, &dbLatency)
	require.NoError(t, err)

	assert.Equal(t, userID, dbUserID)
	require.NotNil(t, dbLeadID)
	assert.Equal(t, leadID, *dbLeadID, "lead attribution propagated end-to-end")
	assert.Equal(t, "image_analysis", dbRequestType)
	assert.Equal(t, "openai", dbProvider)
	assert.Equal(t, "gpt-4o-mini", dbModel)
	assert.Equal(t, 1000, dbInputTokens)
	assert.Equal(t, 500, dbOutputTokens)
	// gpt-4o-mini pricing: $0.15/1M input ($150_000 micro-USD per 1M),
	// $0.60/1M output ($600_000 per 1M). 1000 in + 500 out:
	//   in:  1000 * 150_000 / 1_000_000 = 150
	//   out: 500  * 600_000 / 1_000_000 = 300
	// total = 450 micro-USD ≈ $0.00045.
	assert.Equal(t, int64(450), dbCost, "cost computed from pricing table")
	assert.Equal(t, "success", dbStatus)
	assert.GreaterOrEqual(t, dbLatency, 0)
}

// TestAcceptance_ImageAnalysisThroughProductionProviderChain exercises
// the real OpenAI-compatible provider against a httptest server that
// mimics the chat-completions schema (incl. the usage object). This is
// the regression guard for the first-pass review bug: production type
// assertion in RecordingProvider must reach the inner OpenAIProvider's
// AnalyzeImage method, not silently fall through to ErrVisionUnsupported.
func TestAcceptance_ImageAnalysisThroughProductionProviderChain(t *testing.T) {
	pool := testutil.TestDB(t)
	userID := testutil.SeedUser(t, pool)
	ctx := context.Background()

	leadID := uuid.New()
	_, err := pool.Exec(ctx,
		`INSERT INTO leads (id, user_id, contact_name, channel, email_address, first_message)
		   VALUES ($1, $2, $3, $4, $5, $6)`,
		leadID, userID, "alice", "email", "alice@acme.com", "see screenshot")
	require.NoError(t, err)

	// httptest emulating OpenAI's POST /chat/completions with a usage
	// block — the cost calculation downstream is deterministic.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"id":"x","object":"chat.completion","model":"gpt-4o-mini",
			"choices":[{"message":{"role":"assistant","content":"OCR: backlog fix login"}}],
			"usage":{"prompt_tokens":1000,"completion_tokens":500,"total_tokens":1500}
		}`))
	}))
	defer srv.Close()

	innerProvider := providers.NewOpenAICompatibleProvider("test-key", "gpt-4o-mini", srv.URL+"/", nil)

	repo := audit.NewRepository(pool)
	recorder := audit.NewAsyncRecorder(repo,
		audit.WithBatchSize(1),
		audit.WithFlushInterval(50*time.Millisecond))
	recorder.Start()
	t.Cleanup(func() {
		stopCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = recorder.Stop(stopCtx)
	})

	wrapped := audit.NewRecordingProvider(innerProvider, recorder, nil)
	aiClient := ai.NewAIClient(wrapped, "", "", "", "", "")

	imgCtx := domain.ContextWithCallMeta(ctx, domain.CallMeta{
		UserID:      userID,
		LeadID:      &leadID,
		RequestType: domain.RequestTypeImageAnalysis,
	})

	text, err := aiClient.AnalyzeImage(imgCtx, []byte("png-bytes"), "image/png", "OCR")
	require.NoError(t, err)
	assert.Equal(t, "OCR: backlog fix login", text, "vision response propagates through decorator")

	require.Eventually(t, func() bool {
		var count int
		err := pool.QueryRow(ctx,
			`SELECT COUNT(*) FROM audit_log WHERE user_id = $1 AND request_type = 'image_analysis'`,
			userID).Scan(&count)
		return err == nil && count == 1
	}, 2*time.Second, 25*time.Millisecond, "production-chain image_analysis row missing")

	var (
		dbProvider  string
		dbModel     string
		dbIn, dbOut int
		dbCost      int64
	)
	err = pool.QueryRow(ctx,
		`SELECT provider, model, input_tokens, output_tokens, cost_usd_micro
		   FROM audit_log WHERE user_id = $1`, userID).
		Scan(&dbProvider, &dbModel, &dbIn, &dbOut, &dbCost)
	require.NoError(t, err)
	assert.Equal(t, "openai", dbProvider, "Provider.Name() reaches audit row")
	assert.Equal(t, "gpt-4o-mini", dbModel)
	assert.Equal(t, 1000, dbIn)
	assert.Equal(t, 500, dbOut)
	assert.Equal(t, int64(450), dbCost, "pricing applied through production wiring")
}

// TestAcceptance_UserDeletionCascadesToAuditLog locks the GDPR
// contract documented in docs/audit-log.md: deleting a user must
// remove all of that user's audit rows, regardless of which lead or
// prospect attribution they carry. Without this test, a future
// migration that silently changes the FK action (e.g. SET NULL or
// RESTRICT) would break the erasure pathway without breaking any
// unit test.
func TestAcceptance_UserDeletionCascadesToAuditLog(t *testing.T) {
	pool := testutil.TestDB(t)
	userID := testutil.SeedUser(t, pool)
	ctx := context.Background()

	// Seed a lead + audit row attached to it.
	leadID := uuid.New()
	_, err := pool.Exec(ctx,
		`INSERT INTO leads (id, user_id, contact_name, channel, email_address, first_message)
		   VALUES ($1, $2, $3, $4, $5, $6)`,
		leadID, userID, "alice", "email", "alice@acme.com", "hi")
	require.NoError(t, err)

	repo := audit.NewRepository(pool)
	row1, err := domain.NewEntry(domain.EntryParams{
		UserID: userID, LeadID: &leadID,
		RequestType: domain.RequestTypeQualification,
		Provider:    "openai", Model: "gpt-4o-mini",
		InputTokens: 10, OutputTokens: 5,
		CostUSDMicro: 50, LatencyMS: 100,
		Status: domain.StatusSuccess,
	})
	require.NoError(t, err)
	row2, err := domain.NewEntry(domain.EntryParams{
		UserID: userID,
		RequestType: domain.RequestTypeChatAssist,
		Provider:    "openai", Model: "gpt-4o-mini",
		InputTokens: 20, OutputTokens: 10,
		CostUSDMicro: 100, LatencyMS: 200,
		Status: domain.StatusSuccess,
	})
	require.NoError(t, err)
	require.NoError(t, repo.Save(ctx, []*domain.Entry{row1, row2}))

	// Sanity: two rows exist for this user before deletion.
	var before int
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM audit_log WHERE user_id = $1`, userID).Scan(&before))
	require.Equal(t, 2, before)

	// GDPR erasure pathway.
	_, err = pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, userID)
	require.NoError(t, err)

	var after int
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM audit_log WHERE user_id = $1`, userID).Scan(&after))
	assert.Equal(t, 0, after, "audit_log rows must CASCADE-delete when their user is removed")
}
