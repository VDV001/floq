package audit_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/daniil/floq/internal/audit"
	"github.com/daniil/floq/internal/audit/domain"
)

// recordingMockRepo captures every Save batch so tests can assert how
// the recorder slices its input. It can optionally block on Save and
// return an injected error to exercise the recorder's error paths.
type recordingMockRepo struct {
	mu        sync.Mutex
	batches   [][]*domain.Entry
	err       error
	release   chan struct{} // when non-nil, Save blocks until close
	saveCalls int
}

// CostSummary is unused by the recorder tests — recorder writes only.
// Returning zero is enough to satisfy the AuditRepository interface.
func (m *recordingMockRepo) CostSummary(_ context.Context, _ uuid.UUID, _, _ time.Time) (*domain.CostSummary, error) {
	return &domain.CostSummary{}, nil
}

func (m *recordingMockRepo) Save(ctx context.Context, entries []*domain.Entry) error {
	m.mu.Lock()
	m.saveCalls++
	rel := m.release
	err := m.err
	m.mu.Unlock()

	if rel != nil {
		<-rel
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	if err != nil {
		return err
	}
	batch := make([]*domain.Entry, len(entries))
	copy(batch, entries)
	m.batches = append(m.batches, batch)
	return nil
}

func (m *recordingMockRepo) Batches() [][]*domain.Entry {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([][]*domain.Entry, len(m.batches))
	copy(out, m.batches)
	return out
}

func (m *recordingMockRepo) TotalRows() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	n := 0
	for _, b := range m.batches {
		n += len(b)
	}
	return n
}

func mockEntry(t *testing.T) *domain.Entry {
	t.Helper()
	e, err := domain.NewEntry(domain.EntryParams{
		UserID:       uuid.New(),
		RequestType:  domain.RequestTypeQualification,
		Provider:     "openai",
		Model:        "gpt-4o-mini",
		InputTokens:  10,
		OutputTokens: 5,
		CostUSDMicro: 100,
		LatencyMS:    50,
		Status:       domain.StatusSuccess,
	})
	require.NoError(t, err)
	return e
}

func silentLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestAsyncRecorder_FlushesBatchOnSizeThreshold(t *testing.T) {
	repo := &recordingMockRepo{}
	rec := audit.NewAsyncRecorder(repo,
		audit.WithBatchSize(2),
		audit.WithFlushInterval(time.Hour),
		audit.WithBufferSize(16),
		audit.WithLogger(silentLogger()),
	)
	rec.Start()
	t.Cleanup(func() { _ = rec.Stop(context.Background()) })

	for i := 0; i < 4; i++ {
		rec.Record(context.Background(), mockEntry(t))
	}

	require.Eventually(t, func() bool {
		return repo.TotalRows() == 4
	}, time.Second, 10*time.Millisecond)

	assert.Len(t, repo.Batches(), 2, "batchSize=2 with 4 entries must produce 2 batches")
}

func TestAsyncRecorder_FlushesOnInterval(t *testing.T) {
	repo := &recordingMockRepo{}
	rec := audit.NewAsyncRecorder(repo,
		audit.WithBatchSize(100),
		audit.WithFlushInterval(50*time.Millisecond),
		audit.WithBufferSize(16),
		audit.WithLogger(silentLogger()),
	)
	rec.Start()
	t.Cleanup(func() { _ = rec.Stop(context.Background()) })

	for i := 0; i < 3; i++ {
		rec.Record(context.Background(), mockEntry(t))
	}

	require.Eventually(t, func() bool {
		return repo.TotalRows() == 3
	}, time.Second, 10*time.Millisecond)
}

func TestAsyncRecorder_DropsOnBufferOverflow(t *testing.T) {
	release := make(chan struct{})
	repo := &recordingMockRepo{release: release}
	rec := audit.NewAsyncRecorder(repo,
		audit.WithBatchSize(1),
		audit.WithFlushInterval(time.Hour),
		audit.WithBufferSize(2),
		audit.WithLogger(silentLogger()),
	)
	rec.Start()
	defer func() {
		close(release)
		_ = rec.Stop(context.Background())
	}()

	// First Record gets pulled into the worker and stalls inside Save.
	// Two more fit in the buffer. The next two have nowhere to go and
	// must be dropped (counter increments, no panic).
	for i := 0; i < 5; i++ {
		rec.Record(context.Background(), mockEntry(t))
	}

	// Settle: give the worker a moment to take the first entry off the
	// buffer before we read Dropped(); the 4 remaining map to 2 buffered
	// + 2 dropped.
	require.Eventually(t, func() bool {
		return rec.Dropped() >= 2
	}, time.Second, 10*time.Millisecond)

	assert.GreaterOrEqual(t, rec.Dropped(), int64(2))
}

func TestAsyncRecorder_StopDrainsRemainingEntries(t *testing.T) {
	repo := &recordingMockRepo{}
	rec := audit.NewAsyncRecorder(repo,
		audit.WithBatchSize(100),
		audit.WithFlushInterval(time.Hour),
		audit.WithBufferSize(16),
		audit.WithLogger(silentLogger()),
	)
	rec.Start()

	for i := 0; i < 3; i++ {
		rec.Record(context.Background(), mockEntry(t))
	}

	// Neither size nor interval trigger; only Stop's drain can flush.
	require.NoError(t, rec.Stop(context.Background()))
	assert.Equal(t, 3, repo.TotalRows(), "Stop must drain pending entries before returning")
}

func TestAsyncRecorder_SaveErrorDoesNotKillWorker(t *testing.T) {
	repo := &recordingMockRepo{err: errors.New("postgres down")}
	rec := audit.NewAsyncRecorder(repo,
		audit.WithBatchSize(1),
		audit.WithFlushInterval(time.Hour),
		audit.WithBufferSize(16),
		audit.WithLogger(silentLogger()),
	)
	rec.Start()
	t.Cleanup(func() { _ = rec.Stop(context.Background()) })

	rec.Record(context.Background(), mockEntry(t))
	// Wait for first save attempt
	require.Eventually(t, func() bool {
		repo.mu.Lock()
		defer repo.mu.Unlock()
		return repo.saveCalls >= 1
	}, time.Second, 10*time.Millisecond)

	// Heal the repo and verify the worker is still serving.
	repo.mu.Lock()
	repo.err = nil
	repo.mu.Unlock()
	rec.Record(context.Background(), mockEntry(t))

	require.Eventually(t, func() bool {
		return repo.TotalRows() >= 1
	}, time.Second, 10*time.Millisecond)
}

func TestAsyncRecorder_StopBeforeStartIsSafe(t *testing.T) {
	repo := &recordingMockRepo{}
	rec := audit.NewAsyncRecorder(repo)
	require.NoError(t, rec.Stop(context.Background()), "Stop without Start must not panic or deadlock")
}
