package inbox

import "sync"

// defaultIntakeMaxAttempts is the retry cap used when a poller is constructed
// without an explicit override. The email poller retries roughly once per minute
// and the telegram poller once per back-off, so ten attempts quarantines a
// poisoned source within minutes — long enough to ride out a genuine transient,
// short enough that a deterministic failure stops hot-looping quickly (#208).
const defaultIntakeMaxAttempts = 10

// retryTracker counts consecutive intake failures per source key (an email UID
// or a telegram update_id) so a deterministically-poisoned source can be
// quarantined after a bounded number of attempts instead of being re-processed
// forever (#208). The #206 pollers are fail-closed — a transient failure leaves
// the source unconsumed so the next poll retries it — which, absent a cap, makes
// a permanent post-validation failure hot-loop every poll.
//
// Counts live in memory: a process restart resets them. That is acceptable
// because a pre-quarantine retry is cheap (an IMAP re-fetch or a getUpdates
// re-delivery plus one DB attempt — the AI Qualify call is downstream of intake,
// so there is no AI-cost amplification), and a source that has already been
// quarantined was marked consumed and is never re-fetched.
//
// A maxAttempts <= 0 disables the cap (fail-closed forever — the pre-#208
// behaviour), and a nil *retryTracker behaves identically, so call sites that
// never wire a tracker need no nil guards.
type retryTracker struct {
	mu          sync.Mutex
	attempts    map[string]int
	maxAttempts int
}

func newRetryTracker(maxAttempts int) *retryTracker {
	return &retryTracker{attempts: make(map[string]int), maxAttempts: maxAttempts}
}

// fail records one failed attempt for key and returns the running count together
// with whether the cap is now reached (the caller should quarantine the source
// and stop retrying). On exhaustion the key is dropped so the map does not grow
// without bound and a future source reusing the key is counted afresh. A
// non-positive cap, or a nil tracker, never exhausts.
func (r *retryTracker) fail(key string) (attempts int, exhausted bool) {
	if r == nil {
		return 0, false
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.attempts[key]++
	n := r.attempts[key]
	if r.maxAttempts > 0 && n >= r.maxAttempts {
		delete(r.attempts, key)
		return n, true
	}
	return n, false
}

// succeed drops any tracked failures for key (the source was consumed cleanly),
// so a later unrelated failure for the same key starts from zero. A no-op on a
// nil tracker.
func (r *retryTracker) succeed(key string) {
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.attempts, key)
}
