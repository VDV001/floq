package security

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestCostBreaker_CapInput_Truncates(t *testing.T) {
	b := NewCostBreaker(10, 0, time.Minute)
	out, truncated := b.CapInput("abcdefghijklmnop")
	assert.True(t, truncated)
	assert.Equal(t, 10, len([]rune(out)))
}

func TestCostBreaker_CapInput_ShortUnchanged(t *testing.T) {
	b := NewCostBreaker(10, 0, time.Minute)
	out, truncated := b.CapInput("hello")
	assert.False(t, truncated)
	assert.Equal(t, "hello", out)
}

func TestCostBreaker_CapInput_RuneSafe(t *testing.T) {
	b := NewCostBreaker(3, 0, time.Minute)
	out, truncated := b.CapInput("привет") // 6 Cyrillic runes
	assert.True(t, truncated)
	assert.Equal(t, "при", out)
	assert.True(t, len(out) > 3, "must not split multibyte runes")
}

func TestCostBreaker_CapInput_DisabledWhenZero(t *testing.T) {
	b := NewCostBreaker(0, 0, time.Minute)
	big := strings.Repeat("x", 5000)
	out, truncated := b.CapInput(big)
	assert.False(t, truncated)
	assert.Equal(t, big, out)
}

func TestCostBreaker_Allow_WithinBudget(t *testing.T) {
	b := NewCostBreaker(0, 3, time.Minute)
	assert.True(t, b.Allow("lead-1"))
	assert.True(t, b.Allow("lead-1"))
	assert.True(t, b.Allow("lead-1"))
	assert.False(t, b.Allow("lead-1"), "4th call exceeds budget of 3")
}

func TestCostBreaker_Allow_PerKeyIsolation(t *testing.T) {
	b := NewCostBreaker(0, 1, time.Minute)
	assert.True(t, b.Allow("lead-1"))
	assert.True(t, b.Allow("lead-2"), "different key has its own budget")
	assert.False(t, b.Allow("lead-1"))
}

func TestCostBreaker_Allow_WindowRecovers(t *testing.T) {
	now := time.Date(2026, 6, 7, 12, 0, 0, 0, time.UTC)
	b := NewCostBreaker(0, 1, time.Minute)
	b.now = func() time.Time { return now }

	assert.True(t, b.Allow("lead-1"))
	assert.False(t, b.Allow("lead-1"))

	now = now.Add(2 * time.Minute) // window elapsed
	assert.True(t, b.Allow("lead-1"), "budget recovers after window")
}

func TestCostBreaker_Allow_DisabledWhenZero(t *testing.T) {
	b := NewCostBreaker(0, 0, time.Minute)
	for i := 0; i < 100; i++ {
		assert.True(t, b.Allow("lead-1"), "zero budget disables the breaker")
	}
}
