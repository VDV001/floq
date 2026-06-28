package inbox

import (
	"context"
	"errors"
	"sync"
	"testing"

	auditdomain "github.com/daniil/floq/internal/audit/domain"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- fakes ---

type fakeQualJobStore struct {
	mu       sync.Mutex
	due      []*QualificationJob
	claimErr error
	saved    []*QualificationJob
}

func (f *fakeQualJobStore) EnqueueQualificationJob(context.Context, *QualificationJob) error {
	return nil
}

func (f *fakeQualJobStore) ClaimDueQualificationJobs(context.Context, int, int) ([]*QualificationJob, error) {
	return f.due, f.claimErr
}

func (f *fakeQualJobStore) SaveQualificationJob(_ context.Context, j *QualificationJob) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	// Snapshot so later mutation of j does not rewrite history.
	cp := *j
	f.saved = append(f.saved, &cp)
	return nil
}

func (f *fakeQualJobStore) lastSaved() *QualificationJob {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.saved) == 0 {
		return nil
	}
	return f.saved[len(f.saved)-1]
}

type fakeQualWriter struct {
	mu            sync.Mutex
	upserted      []*InboxQualification
	statusUpdates map[uuid.UUID]LeadStatus
	lead          *InboxLead
	getLeadErr    error
	getLeadCalls  int
}

func newFakeQualWriter(lead *InboxLead) *fakeQualWriter {
	return &fakeQualWriter{statusUpdates: map[uuid.UUID]LeadStatus{}, lead: lead}
}

func (w *fakeQualWriter) UpsertQualification(_ context.Context, q *InboxQualification) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.upserted = append(w.upserted, q)
	return nil
}

func (w *fakeQualWriter) UpdateLeadStatus(_ context.Context, id uuid.UUID, status LeadStatus) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.statusUpdates[id] = status
	return nil
}

func (w *fakeQualWriter) GetLead(_ context.Context, _ uuid.UUID) (*InboxLead, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.getLeadCalls++
	return w.lead, w.getLeadErr
}

type captureCtxQualifier struct {
	mu      sync.Mutex
	result  *QualificationResult
	err     error
	lastCtx context.Context
}

func (q *captureCtxQualifier) Qualify(ctx context.Context, _, _, _ string) (*QualificationResult, error) {
	q.mu.Lock()
	q.lastCtx = ctx
	q.mu.Unlock()
	return q.result, q.err
}

func (q *captureCtxQualifier) ProviderName() string { return "fake-provider" }

// --- helpers ---

func newWorkerJob(t *testing.T, userID, leadID uuid.UUID) *QualificationJob {
	t.Helper()
	j, err := NewQualificationJob(leadID, userID, "Alice", ChannelEmail, "I need a website")
	require.NoError(t, err)
	return j
}

func newTestWorker(store QualificationJobStore, ai AIQualifier, w QualificationWriter) *QualificationWorker {
	return NewQualificationWorker(store, ai, w, QualificationWorkerConfig{BatchLimit: 50, MaxAttempts: 5})
}

// --- tests ---

func TestQualificationWorker_ProcessPending_Success(t *testing.T) {
	userID, leadID := uuid.New(), uuid.New()
	job := newWorkerJob(t, userID, leadID)
	store := &fakeQualJobStore{due: []*QualificationJob{job}}
	ai := &captureCtxQualifier{result: &QualificationResult{Score: 7, IdentifiedNeed: "site"}}
	writer := newFakeQualWriter(&InboxLead{ID: leadID, UserID: userID, ContactName: "Alice", Channel: ChannelEmail})
	emit := &spyLeadQualifiedEmitter{}
	tx := &inlineTx{}

	w := newTestWorker(store, ai, writer)
	w.SetLeadQualifiedEmitter(emit)
	w.SetTxManager(tx)

	n, err := w.ProcessPending(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1, n, "one job qualified")
	assert.Equal(t, 1, tx.count(), "qualify writes + emit + job-done run in one WithTx")
	require.Len(t, writer.upserted, 1, "qualification persisted")
	assert.Equal(t, 7, writer.upserted[0].Score)
	assert.Equal(t, "fake-provider", writer.upserted[0].ProviderUsed)
	assert.Equal(t, StatusQualified, writer.statusUpdates[leadID])
	require.Equal(t, 1, emit.count(), "lead.qualified emitted")
	assert.Equal(t, StatusQualified, emit.leads[0].Status, "emitted lead carries the new status")
	require.NotNil(t, store.lastSaved())
	assert.Equal(t, JobDone, store.lastSaved().Status, "job marked done")
}

func TestQualificationWorker_EmitFailure_MarksJobForRetry(t *testing.T) {
	userID, leadID := uuid.New(), uuid.New()
	job := newWorkerJob(t, userID, leadID)
	store := &fakeQualJobStore{due: []*QualificationJob{job}}
	ai := &captureCtxQualifier{result: &QualificationResult{Score: 7}}
	writer := newFakeQualWriter(&InboxLead{ID: leadID, UserID: userID})
	emit := &spyLeadQualifiedEmitter{err: errors.New("enqueue failed")}
	tx := &inlineTx{}

	w := newTestWorker(store, ai, writer)
	w.SetLeadQualifiedEmitter(emit)
	w.SetTxManager(tx)

	n, err := w.ProcessPending(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 0, n, "a failed emit does not count as qualified")
	assert.Equal(t, 1, tx.count(), "the failing emit ran inside the transaction")
	require.NotNil(t, store.lastSaved())
	assert.Equal(t, JobPending, store.lastSaved().Status, "job stays pending (retryable) after a failed emit")
	assert.Equal(t, 1, store.lastSaved().Attempts)
	assert.NotNil(t, store.lastSaved().NextRetryAt, "retry is scheduled")
}

func TestQualificationWorker_AIFailure_MarksJobForRetry(t *testing.T) {
	userID, leadID := uuid.New(), uuid.New()
	job := newWorkerJob(t, userID, leadID)
	store := &fakeQualJobStore{due: []*QualificationJob{job}}
	ai := &captureCtxQualifier{err: errors.New("ai timeout")}
	writer := newFakeQualWriter(&InboxLead{ID: leadID})
	tx := &inlineTx{}

	w := newTestWorker(store, ai, writer)
	w.SetTxManager(tx)

	n, err := w.ProcessPending(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 0, n)
	assert.Equal(t, 0, tx.count(), "no transaction is opened when qualification itself fails")
	assert.Empty(t, writer.upserted, "nothing persisted when the AI call fails")
	require.NotNil(t, store.lastSaved())
	assert.Equal(t, JobPending, store.lastSaved().Status)
	assert.Equal(t, 1, store.lastSaved().Attempts)
}

func TestQualificationWorker_WebhooksOff_StillQualifies(t *testing.T) {
	userID, leadID := uuid.New(), uuid.New()
	job := newWorkerJob(t, userID, leadID)
	store := &fakeQualJobStore{due: []*QualificationJob{job}}
	ai := &captureCtxQualifier{result: &QualificationResult{Score: 4}}
	writer := newFakeQualWriter(&InboxLead{ID: leadID})
	tx := &inlineTx{}

	w := newTestWorker(store, ai, writer)
	w.SetTxManager(tx) // no emitter wired

	n, err := w.ProcessPending(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1, n, "qualification runs even with webhooks disabled")
	require.Len(t, writer.upserted, 1)
	assert.Equal(t, StatusQualified, writer.statusUpdates[leadID])
	assert.Equal(t, 0, writer.getLeadCalls, "no lead load without an emitter")
	assert.Equal(t, JobDone, store.lastSaved().Status)
}

func TestQualificationWorker_BuildsQualificationCallMeta(t *testing.T) {
	userID, leadID := uuid.New(), uuid.New()
	job := newWorkerJob(t, userID, leadID)
	store := &fakeQualJobStore{due: []*QualificationJob{job}}
	ai := &captureCtxQualifier{result: &QualificationResult{Score: 1}}
	writer := newFakeQualWriter(&InboxLead{ID: leadID})

	w := newTestWorker(store, ai, writer)
	w.SetTxManager(&inlineTx{})

	_, err := w.ProcessPending(context.Background())
	require.NoError(t, err)

	meta, ok := auditdomain.CallMetaFromContext(ai.lastCtx)
	require.True(t, ok, "the worker must stamp fresh CallMeta so the AI call is audited (#182)")
	assert.Equal(t, userID, meta.UserID)
	require.NotNil(t, meta.LeadID)
	assert.Equal(t, leadID, *meta.LeadID)
	assert.Equal(t, auditdomain.RequestTypeQualification, meta.RequestType)
}

func TestQualificationWorker_DeadLetters_AfterMaxAttempts(t *testing.T) {
	userID, leadID := uuid.New(), uuid.New()
	job := newWorkerJob(t, userID, leadID)
	job.Attempts = 4 // one below the cap
	store := &fakeQualJobStore{due: []*QualificationJob{job}}
	ai := &captureCtxQualifier{err: errors.New("ai down")}
	writer := newFakeQualWriter(&InboxLead{ID: leadID})

	w := newTestWorker(store, ai, writer)
	w.SetTxManager(&inlineTx{})

	_, err := w.ProcessPending(context.Background())
	require.NoError(t, err)
	require.NotNil(t, store.lastSaved())
	assert.Equal(t, JobFailed, store.lastSaved().Status, "5th failure dead-letters the job")
	assert.Nil(t, store.lastSaved().NextRetryAt)
}
