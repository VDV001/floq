package inbox

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// recordingEnricher captures Enqueue calls so tests can assert the inbound flow
// fires a best-effort enrichment for each new lead. A non-zero returnErr forces
// failure, used to verify the inbound flow still completes.
type recordingEnricher struct {
	mu        sync.Mutex
	emails    []string
	userIDs   []uuid.UUID
	returnErr error
}

func (e *recordingEnricher) Enqueue(_ context.Context, userID uuid.UUID, email string) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.userIDs = append(e.userIDs, userID)
	e.emails = append(e.emails, email)
	return e.returnErr
}

func (e *recordingEnricher) snapshot() ([]uuid.UUID, []string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	return append([]uuid.UUID(nil), e.userIDs...), append([]string(nil), e.emails...)
}

func TestProcessEmail_CallsEnricher_ForNewLead(t *testing.T) {
	repo := newEmailMockLeadRepo()
	prospectRepo := newEmailMockProspectRepo()
	seqRepo := newMockSequenceRepo()
	enr := &recordingEnricher{}
	ownerID := uuid.New()

	poller := NewEmailPoller(nil, ownerID, "", "", "", "", repo, prospectRepo, seqRepo, nil,
		WithEmailEnricher(enr))

	poller.processEmail(context.Background(), "Alice", "alice@acme.com", "Hi", nil)

	userIDs, emails := enr.snapshot()
	require.Len(t, emails, 1, "enricher must be enqueued exactly once for a new lead")
	assert.Equal(t, ownerID, userIDs[0])
	assert.Equal(t, "alice@acme.com", emails[0])
}

func TestProcessEmail_EnricherError_DoesNotBreakInboundFlow(t *testing.T) {
	repo := newEmailMockLeadRepo()
	prospectRepo := newEmailMockProspectRepo()
	seqRepo := newMockSequenceRepo()
	enr := &recordingEnricher{returnErr: errors.New("enrich queue down")}

	poller := NewEmailPoller(nil, uuid.New(), "", "", "", "", repo, prospectRepo, seqRepo, nil,
		WithEmailEnricher(enr))

	poller.processEmail(context.Background(), "Alice", "alice@acme.com", "Hi", nil)

	require.Len(t, repo.mockLeadRepo.leads, 1, "lead must be created even when enrichment enqueue fails")
	require.Len(t, repo.mockLeadRepo.messages, 1, "message must still land")
}
