package onec

import (
	"context"
	"io"
	"log/slog"
	"sync/atomic"
	"testing"
	"time"
)

type fakeReconciler struct {
	calls int32
	fn    func()
}

func (f *fakeReconciler) ReconcileAll(_ context.Context) error {
	atomic.AddInt32(&f.calls, 1)
	if f.fn != nil {
		f.fn()
	}
	return nil
}

func TestReconcileCron_RunsOnStartupThenStopsOnCancel(t *testing.T) {
	ran := make(chan struct{}, 1)
	rec := &fakeReconciler{fn: func() {
		select {
		case ran <- struct{}{}:
		default:
		}
	}}
	cron := NewReconcileCron(rec, time.Hour, slog.New(slog.NewTextHandler(io.Discard, nil)))

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { cron.Start(ctx); close(done) }()

	select {
	case <-ran:
	case <-time.After(2 * time.Second):
		t.Fatal("ReconcileAll must run once on startup")
	}

	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("cron must stop when ctx is cancelled")
	}
	if atomic.LoadInt32(&rec.calls) < 1 {
		t.Fatal("expected at least one reconcile pass")
	}
}
