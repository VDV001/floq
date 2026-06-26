package providers

import (
	"testing"

	"github.com/daniil/floq/internal/ai"
	"github.com/stretchr/testify/assert"
)

func TestClaudeProvider_ModelForMode(t *testing.T) {
	p := NewClaudeProvider("test-key", "", nil)
	cases := []struct {
		mode ai.ModelMode
		want string
	}{
		{ai.ModelModePlan, "claude-opus-4-7"},
		{ai.ModelModeExecute, "claude-sonnet-4-6"},
		{ai.ModelModeBudget, "claude-haiku-4-5-20251001"},
	}
	for _, tc := range cases {
		t.Run(tc.mode.String(), func(t *testing.T) {
			assert.Equal(t, tc.want, p.modelForMode(tc.mode))
		})
	}
}

func TestClaudeProvider_OverrideModelWinsOverMode(t *testing.T) {
	p := NewClaudeProvider("test-key", "claude-custom-model", nil)
	for _, mode := range []ai.ModelMode{ai.ModelModePlan, ai.ModelModeExecute, ai.ModelModeBudget} {
		assert.Equal(t, "claude-custom-model", p.modelForMode(mode))
	}
}

func TestOpenAIProvider_ModelForMode(t *testing.T) {
	p := NewOpenAIProvider("test-key", "")
	cases := []struct {
		mode ai.ModelMode
		want string
	}{
		{ai.ModelModePlan, "o1"},
		{ai.ModelModeExecute, "gpt-4o"},
		{ai.ModelModeBudget, "gpt-4o-mini"},
	}
	for _, tc := range cases {
		t.Run(tc.mode.String(), func(t *testing.T) {
			assert.Equal(t, tc.want, p.modelForMode(tc.mode))
		})
	}
}

func TestOpenAIProvider_OverrideModelWinsOverMode(t *testing.T) {
	p := NewOpenAIProvider("test-key", "gpt-custom")
	for _, mode := range []ai.ModelMode{ai.ModelModePlan, ai.ModelModeExecute, ai.ModelModeBudget} {
		assert.Equal(t, "gpt-custom", p.modelForMode(mode))
	}
}

func TestOllamaProvider_ModelForMode_AlwaysSameLocalModel(t *testing.T) {
	// Ollama is single-model per instance — local hardware constraint.
	// Mode is ignored; the configured model is returned regardless.
	p := NewOllamaProvider("http://localhost:11434", "qwen2.5-coder", nil)
	for _, mode := range []ai.ModelMode{ai.ModelModePlan, ai.ModelModeExecute, ai.ModelModeBudget} {
		assert.Equal(t, "qwen2.5-coder", p.modelForMode(mode))
	}
}
