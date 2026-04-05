package verify

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestVerifyTelegram_EmptyUsername(t *testing.T) {
	result := VerifyTelegram(nil, "")
	assert.False(t, result.Exists)
	assert.Equal(t, "", result.Username)
	assert.Equal(t, "empty username", result.Error)
}

func TestVerifyTelegram_WhitespaceOnly(t *testing.T) {
	result := VerifyTelegram(nil, "   ")
	assert.False(t, result.Exists)
	assert.Equal(t, "", result.Username)
	assert.Equal(t, "empty username", result.Error)
}

func TestVerifyTelegram_AtSignStripped(t *testing.T) {
	// With nil bot, calling GetChat will panic, so we only test
	// the empty-after-trimming path: "@" becomes "" after trim.
	result := VerifyTelegram(nil, "@")
	assert.False(t, result.Exists)
	assert.Equal(t, "", result.Username)
	assert.Equal(t, "empty username", result.Error)
}

func TestVerifyTelegram_MultipleAtSigns(t *testing.T) {
	// "@@@@" -> TrimLeft "@" -> "" -> empty username
	result := VerifyTelegram(nil, "@@@@")
	assert.False(t, result.Exists)
	assert.Equal(t, "", result.Username)
	assert.Equal(t, "empty username", result.Error)
}
