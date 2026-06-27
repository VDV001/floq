package webhooks

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

type fakeProcessor struct {
	calls atomic.Int32
}

func (f *fakeProcessor) ProcessPending(_ context.Context) (int, error) {
	f.calls.Add(1)
	return 0, nil
}

func TestDeliveryCron_RunsOnceAndStopsOnCancel(t *testing.T) {
	p := &fakeProcessor{}
	cron := NewDeliveryCron(p, 10*time.Millisecond, nil)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { cron.Start(ctx); close(done) }()

	// It runs once on startup; give the ticker a couple of cycles.
	time.Sleep(35 * time.Millisecond)
	cancel()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("cron did not shut down on context cancel")
	}
	if p.calls.Load() < 1 {
		t.Fatal("cron must process at least once (startup drain)")
	}
}
