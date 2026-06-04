package audit

import (
	"context"
	"io"
	"log/slog"
	"sync/atomic"
	"testing"
	"time"
)

type fakePurger struct {
	calls int32
	fn    func()
}

func (f *fakePurger) Purge(_ context.Context) (int, error) {
	atomic.AddInt32(&f.calls, 1)
	if f.fn != nil {
		f.fn()
	}
	return 0, nil
}

func TestRetentionCron_RunsOnStartupThenStopsOnCancel(t *testing.T) {
	ran := make(chan struct{}, 1)
	p := &fakePurger{fn: func() {
		select {
		case ran <- struct{}{}:
		default:
		}
	}}
	cron := NewRetentionCron(p, time.Hour, slog.New(slog.NewTextHandler(io.Discard, nil)))

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { cron.Start(ctx); close(done) }()

	select {
	case <-ran:
	case <-time.After(2 * time.Second):
		t.Fatal("Purge must run once on startup")
	}

	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("cron must stop when ctx is cancelled")
	}
	if atomic.LoadInt32(&p.calls) < 1 {
		t.Fatal("expected at least one purge pass")
	}
}

func TestRetentionCron_RunsAgainOnTick(t *testing.T) {
	p := &fakePurger{}
	cron := NewRetentionCron(p, 20*time.Millisecond, slog.New(slog.NewTextHandler(io.Discard, nil)))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go cron.Start(ctx)

	// Startup pass + at least one tick within the budget.
	deadline := time.After(2 * time.Second)
	for {
		if atomic.LoadInt32(&p.calls) >= 2 {
			return
		}
		select {
		case <-deadline:
			t.Fatalf("expected >=2 purge passes (startup + tick), got %d", atomic.LoadInt32(&p.calls))
		case <-time.After(5 * time.Millisecond):
		}
	}
}
