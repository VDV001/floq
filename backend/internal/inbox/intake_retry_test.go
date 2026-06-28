package inbox

import "testing"

// retryTracker counts consecutive intake failures per source key and reports
// when a bounded cap is reached so a poisoned source can be quarantined instead
// of hot-looping forever (#208). These cases pin the counting, the cap boundary,
// the per-key isolation, the success-reset, and the "cap disabled" escape hatch.
func TestRetryTracker_FailCountingAndCap(t *testing.T) {
	tests := []struct {
		name         string
		maxAttempts  int
		failsBefore  int  // fails recorded for the key before the asserted call
		wantAttempts int  // running count returned by the asserted fail()
		wantExhaust  bool // whether the asserted fail() reports the cap reached
	}{
		{name: "first failure below cap", maxAttempts: 3, failsBefore: 0, wantAttempts: 1, wantExhaust: false},
		{name: "second failure below cap", maxAttempts: 3, failsBefore: 1, wantAttempts: 2, wantExhaust: false},
		{name: "exactly at cap exhausts", maxAttempts: 3, failsBefore: 2, wantAttempts: 3, wantExhaust: true},
		{name: "cap of one exhausts immediately", maxAttempts: 1, failsBefore: 0, wantAttempts: 1, wantExhaust: true},
		{name: "zero cap never exhausts", maxAttempts: 0, failsBefore: 5, wantAttempts: 6, wantExhaust: false},
		{name: "negative cap never exhausts", maxAttempts: -1, failsBefore: 2, wantAttempts: 3, wantExhaust: false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := newRetryTracker(tc.maxAttempts)
			for i := 0; i < tc.failsBefore; i++ {
				r.fail("k")
			}
			gotAttempts, gotExhaust := r.fail("k")
			if gotAttempts != tc.wantAttempts {
				t.Errorf("attempts = %d, want %d", gotAttempts, tc.wantAttempts)
			}
			if gotExhaust != tc.wantExhaust {
				t.Errorf("exhausted = %v, want %v", gotExhaust, tc.wantExhaust)
			}
		})
	}
}

// Each key is counted independently: failing one must not advance another.
func TestRetryTracker_KeysAreIndependent(t *testing.T) {
	r := newRetryTracker(3)
	r.fail("a")
	r.fail("a")
	gotB, exhaustB := r.fail("b")
	if gotB != 1 || exhaustB {
		t.Fatalf("key b = (%d,%v), want (1,false): one key's failures leaked into another", gotB, exhaustB)
	}
}

// succeed drops the running count so a key that recovers starts fresh, and a
// later failure does not inherit the pre-success attempts.
func TestRetryTracker_SucceedResets(t *testing.T) {
	r := newRetryTracker(3)
	r.fail("k")
	r.fail("k")
	r.succeed("k")
	got, exhaust := r.fail("k")
	if got != 1 || exhaust {
		t.Fatalf("after succeed, fail = (%d,%v), want (1,false): count was not reset", got, exhaust)
	}
}

// Reaching the cap drops the key, so a brand-new source that later reuses the
// same key (a fresh UID/update_id is monotonic in practice, but the map must not
// leak) is counted from scratch rather than re-exhausting on its first failure.
func TestRetryTracker_ExhaustionDropsKey(t *testing.T) {
	r := newRetryTracker(2)
	r.fail("k")
	_, exhaust := r.fail("k")
	if !exhaust {
		t.Fatalf("expected exhaustion at cap 2")
	}
	got, exhaustAgain := r.fail("k")
	if got != 1 || exhaustAgain {
		t.Fatalf("after exhaustion, fail = (%d,%v), want (1,false): key was not dropped", got, exhaustAgain)
	}
}

// A nil tracker behaves as "cap disabled": fail never exhausts and succeed is a
// no-op. This lets call sites that never wire a tracker keep the pre-#208
// fail-closed-forever behaviour without nil guards at every use.
func TestRetryTracker_NilIsDisabled(t *testing.T) {
	var r *retryTracker
	got, exhaust := r.fail("k")
	if got != 0 || exhaust {
		t.Fatalf("nil fail = (%d,%v), want (0,false)", got, exhaust)
	}
	r.succeed("k") // must not panic
}
