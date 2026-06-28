package inbox

import (
	"context"
	"errors"
	"testing"
	"time"
)

type fakeTerminalJobPurger struct {
	gotThreshold time.Time
	returnN      int
	returnErr    error
	calls        int
}

func (f *fakeTerminalJobPurger) PurgeTerminalJobsOlderThan(_ context.Context, threshold time.Time) (int, error) {
	f.calls++
	f.gotThreshold = threshold
	return f.returnN, f.returnErr
}

// Purge must turn the retention window (days) into a now-relative cut-off and
// pass it to the repository, returning the deleted-row count.
func TestQualificationRetention_PurgesOlderThanWindow(t *testing.T) {
	fixedNow := time.Date(2026, 6, 28, 12, 0, 0, 0, time.UTC)
	repo := &fakeTerminalJobPurger{returnN: 7}
	uc := NewQualificationRetention(repo, 30)
	uc.now = func() time.Time { return fixedNow }

	n, err := uc.Purge(context.Background())
	if err != nil {
		t.Fatalf("Purge: %v", err)
	}
	if n != 7 {
		t.Errorf("purged = %d, want 7 (repo row count must propagate)", n)
	}
	want := fixedNow.AddDate(0, 0, -30)
	if !repo.gotThreshold.Equal(want) {
		t.Errorf("threshold = %v, want %v (now - retentionDays)", repo.gotThreshold, want)
	}
}

// A repository error must be wrapped, not swallowed.
func TestQualificationRetention_RepoErrorPropagates(t *testing.T) {
	repo := &fakeTerminalJobPurger{returnErr: errors.New("db down")}
	uc := NewQualificationRetention(repo, 30)

	_, err := uc.Purge(context.Background())
	if err == nil {
		t.Fatal("a repository error must propagate out of Purge")
	}
}
