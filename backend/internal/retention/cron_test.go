package retention

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync/atomic"
	"testing"
	"time"
)

type fakePurger struct {
	calls int32
	err   error
	fn    func()
}

func (f *fakePurger) Purge(_ context.Context) (int, error) {
	atomic.AddInt32(&f.calls, 1)
	if f.fn != nil {
		f.fn()
	}
	return 0, f.err
}

func quietLogger() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

// The cron must run a purge once on startup (drain any backlog accrued while
// the server was down) and stop cleanly when its context is cancelled.
func TestCron_RunsOnStartupThenStopsOnCancel(t *testing.T) {
	ran := make(chan struct{}, 1)
	p := &fakePurger{fn: func() {
		select {
		case ran <- struct{}{}:
		default:
		}
	}}
	cron := NewCron("test-queue", p, time.Hour, quietLogger())

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

// Subsequent ticks must keep driving the purge.
func TestCron_RunsAgainOnTick(t *testing.T) {
	p := &fakePurger{}
	cron := NewCron("test-queue", p, 20*time.Millisecond, quietLogger())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go cron.Start(ctx)

	deadline := time.After(2 * time.Second)
	for atomic.LoadInt32(&p.calls) < 3 {
		select {
		case <-deadline:
			t.Fatalf("expected ≥3 purge passes, got %d", atomic.LoadInt32(&p.calls))
		case <-time.After(5 * time.Millisecond):
		}
	}
}

// A purge error must not crash the loop: the next tick still runs.
func TestCron_PurgeErrorDoesNotStopLoop(t *testing.T) {
	p := &fakePurger{err: errors.New("boom")}
	cron := NewCron("test-queue", p, 20*time.Millisecond, quietLogger())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go cron.Start(ctx)

	deadline := time.After(2 * time.Second)
	for atomic.LoadInt32(&p.calls) < 3 {
		select {
		case <-deadline:
			t.Fatalf("a failing purge must not stop the loop; got %d passes", atomic.LoadInt32(&p.calls))
		case <-time.After(5 * time.Millisecond):
		}
	}
}
