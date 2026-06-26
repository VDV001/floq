package audit

import (
	"context"
	"log/slog"
	"sync/atomic"
	"time"

	"github.com/daniil/floq/internal/audit/domain"
)

// AsyncRecorder is the production domain.Recorder: Record returns
// immediately, a single worker goroutine consumes the channel and
// batches inserts through the supplied repository. On buffer overflow,
// entries are dropped (counted via Dropped() and warn-logged) — the
// audit log is the observed-cost ledger, not financial truth, so it
// must never block the AI hot path.
//
// Lifecycle: NewAsyncRecorder → Start → Record* → Stop. Start is
// idempotent (call again is a no-op). Stop without Start is safe and
// returns nil. Calls to Record after Stop are silently dropped (with a
// metric bump). Errors from the underlying repository are logged but
// never propagated — there is no caller to surface them to.
type AsyncRecorder struct {
	repo         domain.AuditRepository
	in           chan *domain.Entry
	quit         chan struct{}
	done         chan struct{}
	batchSize    int
	flushEvery   time.Duration
	logger       *slog.Logger
	dropped      atomic.Int64
	started      atomic.Bool
	stopped      atomic.Bool
}

type recorderOptions struct {
	bufferSize    int
	batchSize     int
	flushInterval time.Duration
	logger        *slog.Logger
}

type AsyncRecorderOption func(*recorderOptions)

func WithBufferSize(n int) AsyncRecorderOption {
	return func(o *recorderOptions) { o.bufferSize = n }
}

func WithBatchSize(n int) AsyncRecorderOption {
	return func(o *recorderOptions) { o.batchSize = n }
}

func WithFlushInterval(d time.Duration) AsyncRecorderOption {
	return func(o *recorderOptions) { o.flushInterval = d }
}

func WithLogger(logger *slog.Logger) AsyncRecorderOption {
	return func(o *recorderOptions) { o.logger = logger }
}

func NewAsyncRecorder(repo domain.AuditRepository, opts ...AsyncRecorderOption) *AsyncRecorder {
	cfg := recorderOptions{
		bufferSize:    1024,
		batchSize:     50,
		flushInterval: 5 * time.Second,
	}
	for _, opt := range opts {
		opt(&cfg)
	}
	if cfg.logger == nil {
		cfg.logger = slog.Default()
	}
	if cfg.bufferSize <= 0 {
		cfg.bufferSize = 1024
	}
	if cfg.batchSize <= 0 {
		cfg.batchSize = 50
	}
	if cfg.flushInterval <= 0 {
		cfg.flushInterval = 5 * time.Second
	}
	return &AsyncRecorder{
		repo:       repo,
		in:         make(chan *domain.Entry, cfg.bufferSize),
		quit:       make(chan struct{}),
		done:       make(chan struct{}),
		batchSize:  cfg.batchSize,
		flushEvery: cfg.flushInterval,
		logger:     cfg.logger,
	}
}

// Compile-time check: AsyncRecorder satisfies the domain port.
var _ domain.Recorder = (*AsyncRecorder)(nil)

// Start spawns the worker goroutine. Idempotent.
func (r *AsyncRecorder) Start() {
	if !r.started.CompareAndSwap(false, true) {
		return
	}
	go r.run()
}

// Record hands an entry to the worker without blocking. If the buffer
// is full or the recorder is stopped, the entry is dropped and the
// dropped counter increments. Callers must not assume their entry
// survives — this is observed cost, not financial truth.
func (r *AsyncRecorder) Record(ctx context.Context, e *domain.Entry) {
	if e == nil {
		return
	}
	if r.stopped.Load() {
		r.dropped.Add(1)
		r.logger.WarnContext(ctx, "audit recorder: Record after Stop, dropping entry")
		return
	}
	select {
	case r.in <- e:
	default:
		total := r.dropped.Add(1)
		r.logger.WarnContext(ctx, "audit recorder: buffer full, dropping entry",
			"dropped_total", total)
	}
}

// Stop signals the worker to drain the buffer and exit, blocking until
// either drainage completes or ctx is cancelled. Safe to call even if
// Start was never invoked.
func (r *AsyncRecorder) Stop(ctx context.Context) error {
	if !r.stopped.CompareAndSwap(false, true) {
		return nil
	}
	if !r.started.Load() {
		// Never ran; release the done channel for symmetry with the
		// started case where it's closed by the worker.
		close(r.done)
		return nil
	}
	close(r.quit)
	select {
	case <-r.done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Dropped returns the cumulative count of entries the recorder rejected
// (full buffer or Record-after-Stop). Used by ops dashboards to spot
// undersized buffers or unhealthy backpressure.
func (r *AsyncRecorder) Dropped() int64 {
	return r.dropped.Load()
}

func (r *AsyncRecorder) run() {
	defer close(r.done)
	batch := make([]*domain.Entry, 0, r.batchSize)
	timer := time.NewTimer(r.flushEvery)
	defer timer.Stop()

	flush := func() {
		if len(batch) == 0 {
			return
		}
		// Use a fresh context: audit writes must succeed regardless of
		// the HTTP request that triggered them (request cancellation
		// is not a reason to lose the cost record).
		if err := r.repo.Save(context.Background(), batch); err != nil {
			r.logger.Warn("audit recorder: save failed", "err", err, "batch_size", len(batch))
		}
		batch = batch[:0]
	}

	resetTimer := func() {
		if !timer.Stop() {
			select {
			case <-timer.C:
			default:
			}
		}
		timer.Reset(r.flushEvery)
	}

	for {
		select {
		case <-r.quit:
			// Drain anything left in the buffer non-blocking, then
			// flush and exit.
		drain:
			for {
				select {
				case e := <-r.in:
					batch = append(batch, e)
				default:
					break drain
				}
			}
			flush()
			return
		case e := <-r.in:
			batch = append(batch, e)
			if len(batch) >= r.batchSize {
				flush()
				resetTimer()
			}
		case <-timer.C:
			flush()
			timer.Reset(r.flushEvery)
		}
	}
}
