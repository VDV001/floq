package inbox

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	auditdomain "github.com/daniil/floq/internal/audit/domain"
	"github.com/daniil/floq/internal/inbox/attachments"
)

// ctxCapturingVisionClient records the ctx passed to AnalyzeImage so
// tests can assert that the attachment-analysis call site stamped the
// right CallMeta before invoking the analyzer.
type ctxCapturingVisionClient struct {
	mu      sync.Mutex
	lastCtx context.Context
	resp    string
}

func (v *ctxCapturingVisionClient) AnalyzeImage(ctx context.Context, _ []byte, _, _ string) (string, error) {
	v.mu.Lock()
	v.lastCtx = ctx
	v.mu.Unlock()
	return v.resp, nil
}

func (v *ctxCapturingVisionClient) captured() context.Context {
	v.mu.Lock()
	defer v.mu.Unlock()
	return v.lastCtx
}

func TestProcessEmail_AttachesQualificationCallMeta(t *testing.T) {
	repo := newEmailMockLeadRepo()
	prospectRepo := newEmailMockProspectRepo()
	seqRepo := newMockSequenceRepo()
	aiClient := newCtxCapturingAIQualifier()
	ownerID := uuid.New()

	poller := NewEmailPoller(nil, ownerID, "", "", "", "", repo, prospectRepo, seqRepo, aiClient, nil)

	poller.processEmail(context.Background(), "Alice", "alice@example.com", "body", nil)

	// Release the qualifier so it returns before t.Cleanup tears the
	// test down — leaving a goroutine blocked produces flakes.
	defer close(aiClient.release)

	var qCtx context.Context
	select {
	case qCtx = <-aiClient.captured:
	case <-time.After(2 * time.Second):
		t.Fatal("Qualify was never invoked")
	}

	meta, ok := auditdomain.CallMetaFromContext(qCtx)
	require.True(t, ok, "qualification goroutine must carry CallMeta in ctx")
	assert.Equal(t, ownerID, meta.UserID)
	require.NotNil(t, meta.LeadID, "qualification is lead-attributed")
	require.Len(t, repo.mockLeadRepo.leads, 1)
	assert.Equal(t, repo.mockLeadRepo.leads[0].ID, *meta.LeadID,
		"lead_id in ctx must match the lead the qualifier was triggered for")
	assert.Equal(t, auditdomain.RequestTypeQualification, meta.RequestType)
}

func TestProcessEmail_AttachmentAnalyzerReceivesImageAnalysisMeta(t *testing.T) {
	repo := newEmailMockLeadRepo()
	prospectRepo := newEmailMockProspectRepo()
	seqRepo := newMockSequenceRepo()
	aiClient := newCtxCapturingAIQualifier()
	ownerID := uuid.New()

	vc := &ctxCapturingVisionClient{resp: "OCR text"}
	analyzer := attachments.New(vc)

	poller := NewEmailPoller(nil, ownerID, "", "", "", "", repo, prospectRepo, seqRepo, aiClient, nil,
		WithAttachmentAnalyzer(analyzer))

	atts := []attachments.Attachment{
		{Filename: "shot.png", ContentType: "image/png", Data: []byte("png-bytes")},
	}
	poller.processEmail(context.Background(), "Alice", "alice@example.com", "body", atts)
	defer close(aiClient.release)

	// Attachment analyzer runs synchronously inside processEmail —
	// the captured ctx is set by the time the call returns.
	visCtx := vc.captured()
	require.NotNil(t, visCtx, "attachment analyzer was never invoked")
	meta, ok := auditdomain.CallMetaFromContext(visCtx)
	require.True(t, ok, "attachment analyzer must run under CallMeta-tagged ctx")
	assert.Equal(t, ownerID, meta.UserID)
	require.NotNil(t, meta.LeadID)
	require.Len(t, repo.mockLeadRepo.leads, 1)
	assert.Equal(t, repo.mockLeadRepo.leads[0].ID, *meta.LeadID)
	assert.Equal(t, auditdomain.RequestTypeImageAnalysis, meta.RequestType,
		"attachment analysis must be tagged image_analysis, not qualification")
}
