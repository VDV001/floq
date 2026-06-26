package enrichment

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

type fakeProcessor struct{ calls int32 }

func (f *fakeProcessor) ProcessPending(context.Context) (int, error) {
	atomic.AddInt32(&f.calls, 1)
	return 0, nil
}

func TestEnrichmentCron_RunsOnceThenStopsOnCancel(t *testing.T) {
	fp := &fakeProcessor{}
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel before Start → run once immediately, then shut down

	NewEnrichmentCron(fp, time.Hour, nil).Start(ctx)

	assert.GreaterOrEqual(t, atomic.LoadInt32(&fp.calls), int32(1),
		"cron runs once on startup even if the context is already cancelled")
}
