package inbox

import (
	"context"
	"sync"
	"testing"

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

func TestProcessEmail_AttachmentAnalyzerReceivesImageAnalysisMeta(t *testing.T) {
	repo := newEmailMockLeadRepo()
	prospectRepo := newEmailMockProspectRepo()
	seqRepo := newMockSequenceRepo()
	ownerID := uuid.New()

	vc := &ctxCapturingVisionClient{resp: "OCR text"}
	analyzer := attachments.New(vc)

	// The analyzer runs inside newQualificationJob, which only executes when a
	// qualification enqueuer is wired (#206 Part C). Wire a stub enqueuer so the
	// attachment-analysis call site is exercised and stamps its CallMeta.
	poller := NewEmailPoller(nil, ownerID, "", "", "", "", repo, prospectRepo, seqRepo, nil,
		WithAttachmentAnalyzer(analyzer))
	poller.SetQualificationEnqueuer(&stubQualEnqueuer{})

	atts := []attachments.Attachment{
		{Filename: "shot.png", ContentType: "image/png", Data: []byte("png-bytes")},
	}
	poller.processEmail(context.Background(), "Alice", "alice@example.com", "body", atts)

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
