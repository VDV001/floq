package chat

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	auditdomain "github.com/daniil/floq/internal/audit/domain"
	"github.com/daniil/floq/internal/httputil"
)

// auditingAIClient captures the ctx of each Complete call so the
// chat handler's CallMeta wiring can be asserted on.
type auditingAIClient struct {
	lastCtx context.Context
}

func (a *auditingAIClient) Complete(ctx context.Context, _ ChatCompletionRequest) (string, error) {
	a.lastCtx = ctx
	return "ok", nil
}

func (a *auditingAIClient) ProviderName() string { return "test" }

func TestChatHandler_AttachesChatAssistCallMeta(t *testing.T) {
	userID := uuid.New()
	stats := &mockStatsReader{stats: &userStats{StatusCounts: map[string]int{}}}
	ai := &auditingAIClient{}
	h := NewHandler(stats, ai)

	body := bytes.NewBufferString(`{"message":"hi"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/chat", body)
	req = req.WithContext(httputil.WithUserID(req.Context(), userID))
	rr := httptest.NewRecorder()

	h.Chat(rr, req)
	require.Equal(t, http.StatusOK, rr.Code, "chat handler returned %d", rr.Code)

	require.NotNil(t, ai.lastCtx, "AIClient.Complete was never called")
	meta, ok := auditdomain.CallMetaFromContext(ai.lastCtx)
	require.True(t, ok, "chat handler must attach CallMeta before AI.Complete")
	assert.Equal(t, userID, meta.UserID)
	assert.Equal(t, auditdomain.RequestTypeChatAssist, meta.RequestType)
	assert.Nil(t, meta.LeadID, "chat is not lead-attributed")
	assert.Nil(t, meta.ProspectID, "chat is not prospect-attributed")
}
