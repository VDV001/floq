package audit

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeRetentionRepo struct {
	gotThreshold time.Time
	purged       int
	err          error
	calls        int
}

func (f *fakeRetentionRepo) AggregateAndPurgeOlderThan(_ context.Context, threshold time.Time) (int, error) {
	f.calls++
	f.gotThreshold = threshold
	return f.purged, f.err
}

func TestRetentionUseCase_Purge_ThresholdIsNowMinusRetentionDays(t *testing.T) {
	fixedNow := time.Date(2026, 6, 4, 12, 0, 0, 0, time.UTC)
	fake := &fakeRetentionRepo{purged: 7}
	uc := NewRetentionUseCase(fake, 30)
	uc.now = func() time.Time { return fixedNow }

	n, err := uc.Purge(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 7, n, "returns the repo's purged count")
	assert.Equal(t, fixedNow.AddDate(0, 0, -30), fake.gotThreshold, "threshold = now - retentionDays")
	assert.Equal(t, 1, fake.calls)
}

func TestRetentionUseCase_Purge_PropagatesRepoError(t *testing.T) {
	fake := &fakeRetentionRepo{err: errors.New("db down")}
	uc := NewRetentionUseCase(fake, 30)

	_, err := uc.Purge(context.Background())
	require.Error(t, err, "a repo failure must surface so the cron logs it")
}
