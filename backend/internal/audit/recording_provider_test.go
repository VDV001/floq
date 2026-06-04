package audit_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/daniil/floq/internal/ai"
	"github.com/daniil/floq/internal/audit"
	"github.com/daniil/floq/internal/audit/domain"
)

// fakeRecorder captures Record calls so tests can assert what the
// decorator produced — it does not persist anything.
type fakeRecorder struct {
	mu      sync.Mutex
	entries []*domain.Entry
}

func (f *fakeRecorder) Record(_ context.Context, e *domain.Entry) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.entries = append(f.entries, e)
}

func (f *fakeRecorder) Entries() []*domain.Entry {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]*domain.Entry, len(f.entries))
	copy(out, f.entries)
	return out
}

// stubProvider satisfies ai.Provider and optionally ai.VisionProvider.
// vision flag controls whether the type assertion in the decorator
// succeeds.
type stubProvider struct {
	name       string
	result     *ai.CompletionResult
	err        error
	visionResp *ai.CompletionResult
	visionErr  error
}

func (s *stubProvider) Name() string { return s.name }

func (s *stubProvider) Complete(_ context.Context, _ ai.CompletionRequest) (*ai.CompletionResult, error) {
	return s.result, s.err
}

type stubVisionProvider struct {
	*stubProvider
}

func (s *stubVisionProvider) AnalyzeImage(_ context.Context, _ []byte, _, _ string) (*ai.CompletionResult, error) {
	return s.visionResp, s.visionErr
}

func silentDiscardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestRecordingProvider_CompleteSuccessRecordsEntry(t *testing.T) {
	t.Parallel()
	inner := &stubProvider{
		name: "openai",
		result: &ai.CompletionResult{
			Text:  "hello",
			Usage: ai.TokenUsage{InputTokens: 100, OutputTokens: 50},
			Model: "gpt-4o-mini",
		},
	}
	rec := &fakeRecorder{}
	rp := audit.NewRecordingProvider(inner, rec, silentDiscardLogger())

	userID := uuid.New()
	leadID := uuid.New()
	ctx := domain.ContextWithCallMeta(context.Background(), domain.CallMeta{
		UserID:      userID,
		LeadID:      &leadID,
		RequestType: domain.RequestTypeQualification,
	})

	resp, err := rp.Complete(ctx, ai.CompletionRequest{})
	require.NoError(t, err)
	assert.Equal(t, "hello", resp.Text)

	entries := rec.Entries()
	require.Len(t, entries, 1)
	e := entries[0]
	assert.Equal(t, userID, e.UserID)
	require.NotNil(t, e.LeadID)
	assert.Equal(t, leadID, *e.LeadID)
	assert.Equal(t, domain.RequestTypeQualification, e.RequestType)
	assert.Equal(t, "openai", e.Provider)
	assert.Equal(t, "gpt-4o-mini", e.Model)
	assert.Equal(t, 100, e.InputTokens)
	assert.Equal(t, 50, e.OutputTokens)
	assert.Equal(t, 150, e.TotalTokens)
	// 100*150_000/1e6 + 50*600_000/1e6 = 15 + 30 = 45 micro-USD
	assert.Equal(t, int64(45), e.CostUSDMicro)
	assert.Equal(t, domain.StatusSuccess, e.Status)
	assert.Empty(t, e.ErrorMessage)
	assert.GreaterOrEqual(t, e.LatencyMS, 0)
}

func TestRecordingProvider_CompleteErrorRecordsErrorEntry(t *testing.T) {
	t.Parallel()
	inner := &stubProvider{
		name: "openai",
		err:  errors.New("rate limited"),
	}
	rec := &fakeRecorder{}
	rp := audit.NewRecordingProvider(inner, rec, silentDiscardLogger())

	ctx := domain.ContextWithCallMeta(context.Background(), domain.CallMeta{
		UserID:      uuid.New(),
		RequestType: domain.RequestTypeDraftReply,
	})

	_, err := rp.Complete(ctx, ai.CompletionRequest{})
	require.Error(t, err)

	entries := rec.Entries()
	require.Len(t, entries, 1)
	e := entries[0]
	assert.Equal(t, domain.StatusError, e.Status)
	assert.Equal(t, "rate limited", e.ErrorMessage)
	assert.Equal(t, int64(0), e.CostUSDMicro)
	assert.Equal(t, 0, e.InputTokens)
	assert.Equal(t, 0, e.OutputTokens)
}

func TestRecordingProvider_NoMetaSkipsRecording(t *testing.T) {
	t.Parallel()
	inner := &stubProvider{
		name: "openai",
		result: &ai.CompletionResult{
			Text:  "hi",
			Usage: ai.TokenUsage{InputTokens: 10, OutputTokens: 5},
			Model: "gpt-4o-mini",
		},
	}
	rec := &fakeRecorder{}
	rp := audit.NewRecordingProvider(inner, rec, silentDiscardLogger())

	_, err := rp.Complete(context.Background(), ai.CompletionRequest{})
	require.NoError(t, err)
	assert.Empty(t, rec.Entries(), "no meta in ctx → no audit entry")
}

func TestRecordingProvider_AnalyzeImageRecordsImageAnalysisType(t *testing.T) {
	t.Parallel()
	stub := &stubProvider{name: "openai"}
	inner := &stubVisionProvider{
		stubProvider: stub,
	}
	stub.visionResp = &ai.CompletionResult{
		Text:  "OCR text",
		Usage: ai.TokenUsage{InputTokens: 200, OutputTokens: 100},
		Model: "gpt-4o-mini",
	}
	rec := &fakeRecorder{}
	rp := audit.NewRecordingProvider(inner, rec, silentDiscardLogger())

	userID := uuid.New()
	leadID := uuid.New()
	ctx := domain.ContextWithCallMeta(context.Background(), domain.CallMeta{
		UserID:      userID,
		LeadID:      &leadID,
		RequestType: domain.RequestTypeImageAnalysis,
	})

	resp, err := rp.AnalyzeImage(ctx, []byte("png"), "image/png", "transcribe")
	require.NoError(t, err)
	assert.Equal(t, "OCR text", resp.Text)

	entries := rec.Entries()
	require.Len(t, entries, 1)
	e := entries[0]
	assert.Equal(t, domain.RequestTypeImageAnalysis, e.RequestType)
	assert.Equal(t, "gpt-4o-mini", e.Model)
	assert.Equal(t, 200, e.InputTokens)
	require.NotNil(t, e.LeadID)
	assert.Equal(t, leadID, *e.LeadID)
}

func TestRecordingProvider_AnalyzeImageOnNonVisionProviderReturnsUnsupported(t *testing.T) {
	t.Parallel()
	inner := &stubProvider{name: "ollama"}
	rec := &fakeRecorder{}
	rp := audit.NewRecordingProvider(inner, rec, silentDiscardLogger())

	ctx := domain.ContextWithCallMeta(context.Background(), domain.CallMeta{
		UserID:      uuid.New(),
		RequestType: domain.RequestTypeImageAnalysis,
	})
	_, err := rp.AnalyzeImage(ctx, []byte("x"), "image/png", "p")
	assert.ErrorIs(t, err, ai.ErrVisionUnsupported)
	assert.Empty(t, rec.Entries(), "no underlying call happened → no audit row")
}

func TestRecordingProvider_PassThroughName(t *testing.T) {
	t.Parallel()
	rp := audit.NewRecordingProvider(&stubProvider{name: "claude"}, &fakeRecorder{}, silentDiscardLogger())
	assert.Equal(t, "claude", rp.Name())
}

func TestRecordingProvider_ObserverFiresOnSuccess(t *testing.T) {
	t.Parallel()
	inner := &stubProvider{
		name: "openai",
		result: &ai.CompletionResult{
			Usage: ai.TokenUsage{InputTokens: 100, OutputTokens: 50},
			Model: "gpt-4o-mini",
		},
	}
	var observed []*domain.Entry
	rp := audit.NewRecordingProvider(inner, &fakeRecorder{}, silentDiscardLogger(),
		audit.WithObserver(func(e *domain.Entry) { observed = append(observed, e) }))

	ctx := domain.ContextWithCallMeta(context.Background(), domain.CallMeta{
		UserID:      uuid.New(),
		RequestType: domain.RequestTypeQualification,
	})
	_, err := rp.Complete(ctx, ai.CompletionRequest{})
	require.NoError(t, err)

	require.Len(t, observed, 1, "observer must fire once per constructed entry")
	assert.Equal(t, "openai", observed[0].Provider)
	assert.Equal(t, domain.RequestTypeQualification, observed[0].RequestType)
}

func TestRecordingProvider_ObserverFiresOnError(t *testing.T) {
	t.Parallel()
	inner := &stubProvider{name: "openai", err: errors.New("rate limited")}
	var observed []*domain.Entry
	rp := audit.NewRecordingProvider(inner, &fakeRecorder{}, silentDiscardLogger(),
		audit.WithObserver(func(e *domain.Entry) { observed = append(observed, e) }))

	ctx := domain.ContextWithCallMeta(context.Background(), domain.CallMeta{
		UserID:      uuid.New(),
		RequestType: domain.RequestTypeDraftReply,
	})
	_, err := rp.Complete(ctx, ai.CompletionRequest{})
	require.Error(t, err)

	// Failed-but-billed calls are real spend signal — observe them too.
	require.Len(t, observed, 1)
	assert.Equal(t, domain.StatusError, observed[0].Status)
}

func TestRecordingProvider_ObserverSkippedWhenNoMeta(t *testing.T) {
	t.Parallel()
	inner := &stubProvider{
		name:   "openai",
		result: &ai.CompletionResult{Model: "gpt-4o-mini"},
	}
	called := 0
	rp := audit.NewRecordingProvider(inner, &fakeRecorder{}, silentDiscardLogger(),
		audit.WithObserver(func(_ *domain.Entry) { called++ }))

	// No CallMeta in ctx → no entry constructed → observer must not fire.
	_, err := rp.Complete(context.Background(), ai.CompletionRequest{})
	require.NoError(t, err)
	assert.Equal(t, 0, called)
}
