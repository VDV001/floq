package providers

import (
	"errors"
	"testing"

	"github.com/daniil/floq/internal/ai"
)

func TestValidateProviderConfig(t *testing.T) {
	tests := []struct {
		name     string
		provider string
		apiKey   string
		wantErr  error
	}{
		{"openai without key is not configured", "openai", "", ai.ErrNotConfigured},
		{"openai with key is ok", "openai", "sk-123", nil},
		{"claude without key is not configured", "claude", "", ai.ErrNotConfigured},
		{"groq without key is not configured", "groq", "", ai.ErrNotConfigured},
		{"gemini without key is not configured", "gemini", "", ai.ErrNotConfigured},
		{"gemini with key is ok", "gemini", "AIza-123", nil},
		{"openrouter without key is not configured", "openrouter", "", ai.ErrNotConfigured},
		{"openrouter with key is ok", "openrouter", "sk-or-123", nil},
		{"ollama needs no key", "ollama", "", nil},
		{"unknown provider is not flagged here", "weird", "", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateProviderConfig(tt.provider, tt.apiKey)
			if tt.wantErr == nil && err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
			if tt.wantErr != nil && !errors.Is(err, tt.wantErr) {
				t.Fatalf("expected %v, got %v", tt.wantErr, err)
			}
		})
	}
}
