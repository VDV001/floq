package metrics_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/daniil/floq/internal/metrics"
	"github.com/stretchr/testify/require"
)

type fakeDepthSource struct {
	mu     sync.Mutex
	depths map[string]int
	err    error
	calls  int
}

func (f *fakeDepthSource) QueueDepths(_ context.Context) (map[string]int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	return f.depths, f.err
}

func scrapeBody(m *metrics.Metrics) string {
	rec := httptest.NewRecorder()
	m.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	b, _ := io.ReadAll(rec.Body)
	return string(b)
}

func TestStartQueueScanner_PollsSourceAndUpdatesGauge(t *testing.T) {
	m := metrics.New()
	src := &fakeDepthSource{depths: map[string]int{"booking_link": 2}}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go m.StartQueueScanner(ctx, src, 10*time.Millisecond)

	require.Eventually(t, func() bool {
		return strings.Contains(scrapeBody(m), `pending_replies_queue_depth{kind="booking_link"} 2`)
	}, 2*time.Second, 10*time.Millisecond, "scanner must publish queue depth from the source")
}

func TestStartQueueScanner_StopsOnContextCancel(t *testing.T) {
	m := metrics.New()
	src := &fakeDepthSource{depths: map[string]int{}}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { m.StartQueueScanner(ctx, src, 10*time.Millisecond); close(done) }()

	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("scanner must return when ctx is cancelled")
	}
}
