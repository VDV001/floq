package inbox

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeQualProcessor struct {
	mu    sync.Mutex
	calls int
}

func (f *fakeQualProcessor) ProcessPending(context.Context) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	return 0, nil
}

func (f *fakeQualProcessor) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls
}

// The cron drains once immediately on start (to clear any backlog accrued while
// the server was down) and stops promptly when its context is cancelled.
func TestQualificationCron_RunsOnceThenStopsOnCancel(t *testing.T) {
	proc := &fakeQualProcessor{}
	cron := NewQualificationCron(proc, time.Hour, nil) // long interval: only the startup run fires

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { cron.Start(ctx); close(done) }()

	require.Eventually(t, func() bool { return proc.count() >= 1 }, time.Second, 5*time.Millisecond,
		"cron must drain once on startup")
	cancel()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("cron did not stop on ctx cancellation")
	}
	assert.GreaterOrEqual(t, proc.count(), 1)
}
