package security

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestOutputValidator_ValidPasses(t *testing.T) {
	v := NewOutputValidator(20)
	got := v.Validate(QualificationView{Score: 75, ScoreReason: "clear need, budget fits", RecommendedAction: "engage"})
	assert.Equal(t, 75, got.Score)
	assert.Equal(t, "engage", got.RecommendedAction)
	assert.False(t, got.Flagged)
}

func TestOutputValidator_ClampsHighScore(t *testing.T) {
	v := NewOutputValidator(20)
	got := v.Validate(QualificationView{Score: 150, ScoreReason: "x", RecommendedAction: "engage"})
	assert.Equal(t, 100, got.Score)
	assert.True(t, got.Flagged)
}

func TestOutputValidator_ClampsNegativeScore(t *testing.T) {
	v := NewOutputValidator(20)
	got := v.Validate(QualificationView{Score: -5, ScoreReason: "x", RecommendedAction: "engage"})
	assert.Equal(t, 0, got.Score)
	assert.True(t, got.Flagged)
}

func TestOutputValidator_RedactsLeakedPII(t *testing.T) {
	v := NewOutputValidator(20)
	got := v.Validate(QualificationView{
		Score:             80,
		ScoreReason:       "lead asked to email ivan@example.com directly",
		RecommendedAction: "engage",
	})
	assert.NotContains(t, got.ScoreReason, "ivan@example.com")
	assert.Contains(t, got.ScoreReason, "[EMAIL_1]")
	assert.True(t, got.Flagged)
}

func TestOutputValidator_LowConfidenceForcesManualReview(t *testing.T) {
	v := NewOutputValidator(20)
	got := v.Validate(QualificationView{Score: 10, ScoreReason: "weak signal", RecommendedAction: "engage"})
	assert.Equal(t, "manual_review", got.RecommendedAction)
	assert.True(t, got.Flagged)
}

func TestOutputValidator_AtConfidenceFloorPasses(t *testing.T) {
	v := NewOutputValidator(20)
	got := v.Validate(QualificationView{Score: 20, ScoreReason: "ok", RecommendedAction: "engage"})
	assert.Equal(t, "engage", got.RecommendedAction)
	assert.False(t, got.Flagged)
}
