package analytics_test

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/daniil/floq/internal/analytics"
	"github.com/stretchr/testify/assert"
)

// fakeRefresher counts RefreshMatviews calls; optionally returns an error.
type fakeRefresher struct {
	calls atomic.Int32
	err   error
}

func (f *fakeRefresher) RefreshMatviews(_ context.Context) error {
	f.calls.Add(1)
	return f.err
}

func TestRefreshCron_RefreshesOnceAndStopsOnCtx(t *testing.T) {
	fake := &fakeRefresher{}
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // already cancelled: Start refreshes once, then exits on ctx.Done

	done := make(chan struct{})
	go func() {
		analytics.NewRefreshCron(fake, time.Hour, nil).Start(ctx)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Start did not return after ctx cancel")
	}
	assert.GreaterOrEqual(t, fake.calls.Load(), int32(1), "refreshed at least once on startup")
}

func TestRefreshCron_SwallowsRefreshError(t *testing.T) {
	fake := &fakeRefresher{err: errors.New("boom")}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	done := make(chan struct{})
	go func() {
		// A failing refresh must be logged, not fatal — the cron keeps running.
		analytics.NewRefreshCron(fake, time.Hour, nil).Start(ctx)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Start did not return after a failed refresh")
	}
	assert.GreaterOrEqual(t, fake.calls.Load(), int32(1))
}
