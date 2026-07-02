package providers

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// #238: an OpenAI-compatible provider (Groq, Together, …) must report its
// real name, not the hard-coded "openai" — otherwise the settings
// connection test says «Подключение к openai успешно» for a Groq key.
func TestOpenAIProvider_Name_Default(t *testing.T) {
	p := NewOpenAIProvider("k", "gpt-4o")
	assert.Equal(t, "openai", p.Name(), "the plain OpenAI provider keeps its name")
}

func TestOpenAICompatibleProvider_Name_UsesGivenName(t *testing.T) {
	p := NewOpenAICompatibleProvider("k", "llama", "groq", "https://api.groq.com/openai/v1", nil)
	assert.Equal(t, "groq", p.Name(), "a compatible provider reports the name it was constructed with")
}
