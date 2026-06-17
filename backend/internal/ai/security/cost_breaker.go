package security

import (
	"sync"
	"time"
)

// CostBreaker is agent-security-defaults layer 4: it bounds the blast radius
// of untrusted inbound on the (paid, rate-limited) LLM. Two controls:
//
//   1. Input length cap — an oversized payload (a 1 MB email, a pasted dump)
//      is truncated before it reaches the model. Long inputs cost tokens and
//      widen the injection surface; qualification never needs the whole thing.
//   2. Per-key call budget — a sliding window of allowed LLM calls per key
//      (e.g. per lead/conversation). A flood of inbound from one source trips
//      the breaker, so the bill cannot run away and a loop cannot recurse
//      unbounded.
//
// Thread-safe: the call ledger is mutex-guarded. The clock is injectable for
// deterministic tests.
type CostBreaker struct {
	maxInputRunes  int
	maxCallsPerKey int
	window         time.Duration

	mu    sync.Mutex
	calls map[string][]time.Time
	now   func() time.Time
}

// NewCostBreaker builds a breaker. maxInputRunes ≤ 0 disables truncation;
// maxCallsPerKey ≤ 0 disables the call budget.
func NewCostBreaker(maxInputRunes, maxCallsPerKey int, window time.Duration) *CostBreaker {
	return &CostBreaker{
		maxInputRunes:  maxInputRunes,
		maxCallsPerKey: maxCallsPerKey,
		window:         window,
		calls:          make(map[string][]time.Time),
		now:            time.Now,
	}
}

// CapInput truncates text to maxInputRunes, returning the (possibly shortened)
// text and whether truncation happened. Rune-safe (never splits a multibyte
// character).
func (b *CostBreaker) CapInput(text string) (string, bool) {
	return "", false
}

// Allow records a call against key and reports whether it is within budget.
// Calls older than the window are evicted. A tripped breaker returns false and
// does NOT record the call (so the window can recover).
func (b *CostBreaker) Allow(key string) bool {
	return false
}
