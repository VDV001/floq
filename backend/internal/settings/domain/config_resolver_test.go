package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestResolveConfig_DBValueTakesPrecedence(t *testing.T) {
	result := ResolveConfig("from-db", "from-env")
	assert.Equal(t, "from-db", result)
}

func TestResolveConfig_FallbackWhenDBEmpty(t *testing.T) {
	result := ResolveConfig("", "from-env")
	assert.Equal(t, "from-env", result)
}

func TestResolveConfig_BothEmpty(t *testing.T) {
	result := ResolveConfig("", "")
	assert.Equal(t, "", result)
}
