package audit_test

import (
	"testing"

	"github.com/daniil/floq/internal/audit"
)

func TestCostMicroUSD_KnownModels(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name         string
		provider     string
		model        string
		inputTokens  int
		outputTokens int
		wantCost     int64 // micro-USD
	}{
		// gpt-4o-mini: $0.15 / $0.60 per 1M → 150_000 / 600_000 micro per 1M
		{"gpt-4o-mini 1M+0.5M", "openai", "gpt-4o-mini", 1_000_000, 500_000, 150_000 + 300_000},
		{"gpt-4o-mini 100 in", "openai", "gpt-4o-mini", 100, 0, 15},   // 100*150_000/1e6 = 15
		{"gpt-4o-mini 100 out", "openai", "gpt-4o-mini", 0, 100, 60},  // 100*600_000/1e6 = 60
		{"gpt-4o-mini floor sub-micro", "openai", "gpt-4o-mini", 5, 0, 0}, // 5*150_000/1e6 = 0.75 → floor 0

		// gpt-4o: $5 / $20 per 1M → 5_000_000 / 20_000_000 micro per 1M
		{"gpt-4o small call", "openai", "gpt-4o", 1_000, 500, 5_000 + 10_000},

		// o1 (Plan mode default): $15 / $60 per 1M
		{"o1 medium call", "openai", "o1", 10_000, 5_000, 150_000 + 300_000},

		// Anthropic Claude
		{"claude-3-5-haiku 1M each", "claude", "claude-3-5-haiku-20241022", 1_000_000, 1_000_000, 1_000_000 + 5_000_000},
		{"claude-sonnet-4 small", "claude", "claude-sonnet-4-20250514", 1_000, 500, 3_000 + 7_500},
		{"claude-sonnet-4-6 (default Execute) 10k each", "claude", "claude-sonnet-4-6", 10_000, 10_000, 30_000 + 150_000},
		{"claude-haiku-4-5 (default Budget) 100k each", "claude", "claude-haiku-4-5-20251001", 100_000, 100_000, 80_000 + 400_000},

		// Ollama is local — provider cost is 0 regardless of model.
		{"ollama llama3 1k+1k", "ollama", "llama3", 1_000, 1_000, 0},
		{"ollama unknown model", "ollama", "whatever", 999_999, 999_999, 0},

		// Zero-token success
		{"zero tokens gpt-4o", "openai", "gpt-4o", 0, 0, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, known := audit.CostMicroUSD(tc.provider, tc.model, tc.inputTokens, tc.outputTokens)
			if !known {
				t.Fatalf("expected known=true for %s/%s", tc.provider, tc.model)
			}
			if got != tc.wantCost {
				t.Errorf("cost = %d, want %d", got, tc.wantCost)
			}
		})
	}
}

func TestCostMicroUSD_UnknownModel(t *testing.T) {
	t.Parallel()
	cases := []struct {
		provider string
		model    string
	}{
		{"openai", "gpt-99"},
		{"claude", "claude-future"},
		{"anthropic", "anything"}, // unknown provider — we key on "claude"
		{"groq", "anything"},      // unknown provider altogether
		{"", "gpt-4o"},             // empty provider
		{"openai", ""},             // empty model
	}
	for _, tc := range cases {
		t.Run(tc.provider+"/"+tc.model, func(t *testing.T) {
			t.Parallel()
			cost, known := audit.CostMicroUSD(tc.provider, tc.model, 1_000, 500)
			if known {
				t.Errorf("expected known=false for %s/%s", tc.provider, tc.model)
			}
			if cost != 0 {
				t.Errorf("cost = %d, want 0 for unknown", cost)
			}
		})
	}
}
