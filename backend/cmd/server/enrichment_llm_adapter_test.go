package main

import (
	"context"
	"strings"
	"testing"

	"github.com/daniil/floq/internal/ai"
	auditdomain "github.com/daniil/floq/internal/audit/domain"
	"github.com/daniil/floq/internal/enrichment"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// captureProvider records the ctx + request it was called with.
type captureProvider struct {
	gotCtx context.Context
	gotReq ai.CompletionRequest
}

func (p *captureProvider) Complete(ctx context.Context, req ai.CompletionRequest) (*ai.CompletionResult, error) {
	p.gotCtx = ctx
	p.gotReq = req
	return &ai.CompletionResult{Text: "{}", Model: "gpt-4o-mini"}, nil
}
func (p *captureProvider) Name() string { return "capture" }

func TestEnrichmentLLMAdapter_CapsInputAndForcesBudget(t *testing.T) {
	cap := &captureProvider{}
	const maxRunes = 100
	adapter := newEnrichmentLLMAdapter(cap, maxRunes, 64)

	longPage := strings.Repeat("x", 500)
	_, err := adapter.Complete(context.Background(), "system", longPage)
	require.NoError(t, err)

	// Input capped before reaching the provider.
	require.Len(t, cap.gotReq.Messages, 2)
	assert.Equal(t, "system", cap.gotReq.Messages[0].Content)
	assert.Len(t, []rune(cap.gotReq.Messages[1].Content), maxRunes, "user prompt capped to maxRunes")
	// Cheapest model mode + tight output cap.
	assert.Equal(t, ai.ModelModeBudget, cap.gotReq.Mode)
	assert.Equal(t, 64, cap.gotReq.MaxTokens)
}

func TestEnrichmentLLMAdapter_TagsAuditMetaFromSubjectUser(t *testing.T) {
	cap := &captureProvider{}
	adapter := newEnrichmentLLMAdapter(cap, 8000, 64)

	uid := uuid.New()
	ctx := enrichment.WithSubjectUser(context.Background(), uid)
	_, err := adapter.Complete(ctx, "system", "page")
	require.NoError(t, err)

	// The provider call MUST carry a CallMeta the RecordingProvider can use —
	// otherwise the audit row is dropped (un-audited enrichment spend).
	meta, ok := auditdomain.CallMetaFromContext(cap.gotCtx)
	require.True(t, ok, "audit CallMeta must be attached so the call is recorded")
	assert.Equal(t, auditdomain.RequestTypeEnrichment, meta.RequestType)
	assert.Equal(t, uid, meta.UserID)
}

func TestEnrichmentLLMAdapter_NoSubjectUser_NoMeta(t *testing.T) {
	cap := &captureProvider{}
	adapter := newEnrichmentLLMAdapter(cap, 8000, 64)

	_, err := adapter.Complete(context.Background(), "system", "page")
	require.NoError(t, err)

	// Without a subject user we attach no meta (a Nil-user CallMeta would be
	// rejected by NewEntry); the call still goes through, just un-attributed.
	_, ok := auditdomain.CallMetaFromContext(cap.gotCtx)
	assert.False(t, ok)
}
