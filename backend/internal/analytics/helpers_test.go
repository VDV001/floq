package analytics

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// These pin the pure helpers that guard the analytics wire boundary: rate
// math that must never emit NaN/Inf, the integer cost ratio, and the
// period-window cutoff/NULL serialisation. They need no database — the SQL
// methods that call them are covered by the integration suite.

func TestSafeRate(t *testing.T) {
	tests := []struct {
		name  string
		num   int
		denom int
		want  float64
	}{
		{"zero denominator returns 0 (no Inf)", 5, 0, 0},
		{"negative denominator returns 0", 5, -3, 0},
		{"zero numerator", 0, 5, 0},
		{"normal fraction", 1, 4, 0.25},
		{"whole ratio", 3, 3, 1.0},
		{"greater than one", 9, 4, 2.25},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, safeRate(tt.num, tt.denom))
		})
	}
}

func TestSafeRatioInt(t *testing.T) {
	tests := []struct {
		name       string
		totalMicro int64
		count      int
		want       int64
	}{
		{"zero count returns 0 (no panic)", 1000, 0, 0},
		{"negative count returns 0", 1000, -2, 0},
		{"zero total", 0, 5, 0},
		{"exact division", 1000, 4, 250},
		{"integer truncation toward zero", 10, 3, 3},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, safeRatioInt(tt.totalMicro, tt.count))
		})
	}
}

func TestPeriodCutoff(t *testing.T) {
	now := time.Date(2026, 6, 25, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name     string
		period   Period
		wantHas  bool
		wantTime time.Time
	}{
		{"week → 7 days back", PeriodWeek, true, now.Add(-7 * 24 * time.Hour)},
		{"month → 30 days back", PeriodMonth, true, now.Add(-30 * 24 * time.Hour)},
		{"all → no cutoff", PeriodAll, false, time.Time{}},
		{"unknown period → no cutoff", Period("nonsense"), false, time.Time{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, has := periodCutoff(tt.period, now)
			assert.Equal(t, tt.wantHas, has)
			assert.Equal(t, tt.wantTime, got)
		})
	}
}

func TestNullableCutoff(t *testing.T) {
	ts := time.Date(2026, 6, 18, 12, 0, 0, 0, time.UTC)

	t.Run("no cutoff serialises as nil (SQL NULL)", func(t *testing.T) {
		assert.Nil(t, nullableCutoff(ts, false))
	})
	t.Run("with cutoff returns the timestamp", func(t *testing.T) {
		assert.Equal(t, ts, nullableCutoff(ts, true))
	})
}
