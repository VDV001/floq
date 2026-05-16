package inbox

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
)

// ctxCapturingAIQualifier records the context Qualify is called with and
// blocks until either that context is cancelled or release is closed.
// Tests use it to verify that async qualification goroutines propagate
// cancellation from their parent context (graceful shutdown).
//
// captured is buffered (cap=1); the non-blocking send with a default
// arm is already race-safe, so no mutex is needed.
type ctxCapturingAIQualifier struct {
	captured chan context.Context
	release  chan struct{}
}

func newCtxCapturingAIQualifier() *ctxCapturingAIQualifier {
	return &ctxCapturingAIQualifier{
		captured: make(chan context.Context, 1),
		release:  make(chan struct{}),
	}
}

func (m *ctxCapturingAIQualifier) Qualify(ctx context.Context, _, _, _ string) (*QualificationResult, error) {
	select {
	case m.captured <- ctx:
	default:
	}

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-m.release:
		return &QualificationResult{Score: 5}, nil
	}
}

func (m *ctxCapturingAIQualifier) ProviderName() string { return "capturing" }

func TestProcessEmail_AsyncQualify_DerivesFromParentContext(t *testing.T) {
	repo := newEmailMockLeadRepo()
	prospectRepo := newEmailMockProspectRepo()
	seqRepo := newMockSequenceRepo()
	aiClient := newCtxCapturingAIQualifier()

	poller := NewEmailPoller(nil, uuid.New(), "", "", "", "", repo, prospectRepo, seqRepo, aiClient, nil)

	parentCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	poller.processEmail(parentCtx, "Alice", "alice@example.com", "Hello, need a CRM", nil)

	var qCtx context.Context
	select {
	case qCtx = <-aiClient.captured:
	case <-time.After(2 * time.Second):
		t.Fatal("Qualify never invoked")
	}

	cancel()

	select {
	case <-qCtx.Done():
		// pass — async ctx propagates parent cancellation
	case <-time.After(1 * time.Second):
		t.Fatal("async qualify ctx did not cancel when parent ctx was cancelled — goroutine uses context.Background() instead of parent ctx")
	}

	close(aiClient.release)
}

// TestProcessEmail_AsyncQualify_RetainsTimeout asserts that the qualify
// context still carries a finite deadline derived from the 30s timeout
// when the parent ctx is never cancelled — so the goroutine cannot wedge
// forever on a stuck AI call.
func TestProcessEmail_AsyncQualify_RetainsTimeout(t *testing.T) {
	repo := newEmailMockLeadRepo()
	prospectRepo := newEmailMockProspectRepo()
	seqRepo := newMockSequenceRepo()
	aiClient := newCtxCapturingAIQualifier()

	poller := NewEmailPoller(nil, uuid.New(), "", "", "", "", repo, prospectRepo, seqRepo, aiClient, nil)

	parentCtx := context.Background()
	poller.processEmail(parentCtx, "Alice", "alice@example.com", "Hello", nil)

	var qCtx context.Context
	select {
	case qCtx = <-aiClient.captured:
	case <-time.After(2 * time.Second):
		t.Fatal("Qualify never invoked")
	}

	deadline, ok := qCtx.Deadline()
	if !ok {
		t.Fatal("async qualify ctx has no deadline — timeout was dropped")
	}
	remaining := time.Until(deadline)
	if remaining <= 0 || remaining > 31*time.Second {
		t.Fatalf("async qualify deadline outside expected window: remaining=%v", remaining)
	}

	close(aiClient.release)
	// Drain the qualifyDone signal so the test doesn't leak the goroutine
	// past its UpdateLeadStatus call.
	select {
	case <-repo.qualifyDone:
	case <-time.After(2 * time.Second):
	}
}

func TestHandleMessage_AsyncQualify_DerivesFromParentContext(t *testing.T) {
	repo := newMockLeadRepo()
	aiClient := newCtxCapturingAIQualifier()
	ownerID := uuid.New()
	bot := newTestBot(repo, aiClient, ownerID, "https://cal.com/test")

	parentCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	msg := makeTgMessage(12345, "Ivan", "Petrov", "Hello, I need a CRM")
	bot.handleMessage(parentCtx, msg)

	var qCtx context.Context
	select {
	case qCtx = <-aiClient.captured:
	case <-time.After(2 * time.Second):
		t.Fatal("Qualify never invoked")
	}

	cancel()

	select {
	case <-qCtx.Done():
		// pass
	case <-time.After(1 * time.Second):
		t.Fatal("async qualify ctx did not cancel when parent ctx was cancelled — goroutine uses context.Background() instead of parent ctx")
	}

	close(aiClient.release)
}
