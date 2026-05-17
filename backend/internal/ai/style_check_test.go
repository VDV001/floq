package ai

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	auditdomain "github.com/daniil/floq/internal/audit/domain"
)

func TestStyleCheck_HighScore(t *testing.T) {
	jsonResp := `{"score":9,"issues":[],"feedback":""}`
	c := NewAIClient(&mockProvider{response: jsonResp}, "", "", "", "", "")

	got, err := c.StyleCheck(context.Background(), "Hello, hope you are well", "email")
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, 9, got.Score)
	assert.Empty(t, got.Issues)
}

func TestStyleCheck_LowScore(t *testing.T) {
	jsonResp := `{"score":3,"issues":["jargon: безусловно","deck-style sentence"],"feedback":"Уберите канцелярит, добавьте конкретики"}`
	c := NewAIClient(&mockProvider{response: jsonResp}, "", "", "", "", "")

	got, err := c.StyleCheck(context.Background(), "Безусловно, в современном мире...", "email")
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, 3, got.Score)
	assert.Len(t, got.Issues, 2)
	assert.Contains(t, got.Feedback, "канцелярит")
}

func TestStyleCheck_MarkdownWrappedJSON(t *testing.T) {
	jsonResp := "```json\n{\"score\":7,\"issues\":[],\"feedback\":\"\"}\n```"
	c := NewAIClient(&mockProvider{response: jsonResp}, "", "", "", "", "")

	got, err := c.StyleCheck(context.Background(), "draft", "telegram")
	require.NoError(t, err)
	assert.Equal(t, 7, got.Score)
}

func TestStyleCheck_MalformedJSON(t *testing.T) {
	c := NewAIClient(&mockProvider{response: "definitely not json"}, "", "", "", "", "")
	_, err := c.StyleCheck(context.Background(), "draft", "email")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "style check")
}

func TestStyleCheck_ProviderError(t *testing.T) {
	c := NewAIClient(&mockProvider{err: errors.New("api down")}, "", "", "", "", "")
	_, err := c.StyleCheck(context.Background(), "draft", "email")
	require.Error(t, err)
}

func TestStyleCheck_ChannelTunesPrompt(t *testing.T) {
	// The channel argument must reach the LLM via the user prompt — verify
	// by checking the recorded request. We swap in a recording mock that
	// captures the last request body.
	rec := &recordingProvider{response: `{"score":8,"issues":[],"feedback":""}`}
	c := NewAIClient(rec, "", "", "", "", "")

	_, err := c.StyleCheck(context.Background(), "hello", "telegram")
	require.NoError(t, err)
	require.NotNil(t, rec.lastRequest)

	var sawChannel bool
	for _, m := range rec.lastRequest.Messages {
		if m.Role == "user" && contains(m.Content, "telegram") {
			sawChannel = true
		}
	}
	assert.True(t, sawChannel, "channel arg must be interpolated into the user prompt")
}

func TestStyleCheck_UsesBudgetMode(t *testing.T) {
	rec := &recordingProvider{response: `{"score":8,"issues":[],"feedback":""}`}
	c := NewAIClient(rec, "", "", "", "", "")

	_, err := c.StyleCheck(context.Background(), "hello", "email")
	require.NoError(t, err)
	require.NotNil(t, rec.lastRequest)
	assert.Equal(t, ModelModeBudget, rec.lastRequest.Mode, "style check should use Budget mode for cost")
}

// TestStyleCheck_OverridesRequestTypeToStyleCheck locks the production
// invariant that the inner Complete call fires under a ctx carrying
// RequestTypeStyleCheck while preserving the parent's user/lead
// attribution. Without this test, a typo in style_check.go's
// WithRequestType wiring (or a refactor that drops the call) would
// silently re-tag style-check spend under the parent's RequestType.
func TestStyleCheck_OverridesRequestTypeToStyleCheck(t *testing.T) {
	rec := &recordingProvider{response: `{"score":8,"issues":[],"feedback":""}`}
	c := NewAIClient(rec, "", "", "", "", "")

	userID := uuid.New()
	leadID := uuid.New()
	parentCtx := auditdomain.ContextWithCallMeta(context.Background(), auditdomain.CallMeta{
		UserID:      userID,
		LeadID:      &leadID,
		RequestType: auditdomain.RequestTypeDraftReply,
	})

	_, err := c.StyleCheck(parentCtx, "draft body", "email")
	require.NoError(t, err)
	require.NotNil(t, rec.lastCtx, "provider Complete was never invoked")

	meta, ok := auditdomain.CallMetaFromContext(rec.lastCtx)
	require.True(t, ok, "style-check inner Complete must keep audit meta in ctx")
	assert.Equal(t, userID, meta.UserID, "user attribution must survive")
	require.NotNil(t, meta.LeadID)
	assert.Equal(t, leadID, *meta.LeadID, "lead attribution must survive")
	assert.Equal(t, auditdomain.RequestTypeStyleCheck, meta.RequestType,
		"inner call must be re-tagged as style_check, not draft_reply")
}

func TestStyleCheck_NoParentMetaLeavesCtxClean(t *testing.T) {
	rec := &recordingProvider{response: `{"score":8,"issues":[],"feedback":""}`}
	c := NewAIClient(rec, "", "", "", "", "")

	_, err := c.StyleCheck(context.Background(), "draft body", "email")
	require.NoError(t, err)
	require.NotNil(t, rec.lastCtx)

	_, ok := auditdomain.CallMetaFromContext(rec.lastCtx)
	assert.False(t, ok, "WithRequestType must NOT synthesize meta when none present")
}

// recordingProvider captures the most recent CompletionRequest and ctx
// so tests can assert on prompt body, mode, and audit attribution.
// Lives in style_check_test.go because that's the only file using it
// today; promote if shared.
type recordingProvider struct {
	response    string
	err         error
	lastRequest *CompletionRequest
	lastCtx     context.Context
	calls       int
}

func (r *recordingProvider) Complete(ctx context.Context, req CompletionRequest) (*CompletionResult, error) {
	r.calls++
	cp := req
	r.lastRequest = &cp
	r.lastCtx = ctx
	return &CompletionResult{Text: r.response, Model: "recording"}, r.err
}

func (r *recordingProvider) Name() string { return "recording" }

// --- AnalyzeImage (vision) ---

// visionMockProvider satisfies the (yet-to-be-added) VisionProvider
// interface that AIClient checks via type assertion. The interface is
// optional: only providers that support multimodal calls implement it.
type visionMockProvider struct {
	mockProvider
	visionResp string
	visionErr  error
	lastMime   string
	lastPrompt string
	lastBytes  int
}

func (v *visionMockProvider) AnalyzeImage(_ context.Context, data []byte, mimeType, prompt string) (*CompletionResult, error) {
	v.lastMime = mimeType
	v.lastPrompt = prompt
	v.lastBytes = len(data)
	if v.visionErr != nil {
		return nil, v.visionErr
	}
	return &CompletionResult{Text: v.visionResp, Model: "gpt-4o-mini"}, nil
}

func TestAIClient_AnalyzeImage_Success(t *testing.T) {
	vp := &visionMockProvider{visionResp: "Backlog: fix login"}
	c := NewAIClient(vp, "", "", "", "", "")

	got, err := c.AnalyzeImage(context.Background(), []byte("png"), "image/png", "OCR this")
	require.NoError(t, err)
	assert.Equal(t, "Backlog: fix login", got)
	assert.Equal(t, "image/png", vp.lastMime)
	assert.Equal(t, "OCR this", vp.lastPrompt)
	assert.Equal(t, 3, vp.lastBytes)
}

func TestAIClient_AnalyzeImage_UnsupportedProvider(t *testing.T) {
	// mockProvider does NOT implement VisionProvider; AnalyzeImage must
	// return ErrVisionUnsupported so callers can route the attachment to
	// a graceful skip path.
	mp := &mockProvider{response: "irrelevant"}
	c := NewAIClient(mp, "", "", "", "", "")

	_, err := c.AnalyzeImage(context.Background(), []byte("png"), "image/png", "p")
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrVisionUnsupported)
}

func TestAIClient_AnalyzeImage_ProviderError(t *testing.T) {
	vp := &visionMockProvider{visionErr: errors.New("rate limit")}
	c := NewAIClient(vp, "", "", "", "", "")

	_, err := c.AnalyzeImage(context.Background(), []byte("png"), "image/png", "p")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "analyze image")
}

// --- helpers ---

func contains(haystack, needle string) bool {
	// avoid pulling in strings import here to keep the helper self-contained
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}
