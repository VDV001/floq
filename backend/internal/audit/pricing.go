// Pricing data for AI provider calls. Keyed on (provider, model) and
// expressed in micro-USD per million tokens so all arithmetic stays in
// int64 (no float drift in cost aggregations).
//
// Sources, captured on 2026-05-16:
//   * OpenAI       — https://openai.com/api/pricing/
//   * Anthropic    — https://www.anthropic.com/pricing
//
// Update procedure when a provider revises prices: change the constants
// here, bump the captured date above, and add a chronicles entry. Costs
// recorded before the change retain their original values (we store the
// computed cost, not a recompute pointer).
package audit

type modelPrice struct {
	inputPerMillionMicro  int64
	outputPerMillionMicro int64
}

// pricingTable's outer key matches Provider.Name() verbatim so the
// recording layer can look up prices without any string translation.
// "claude" intentionally collides with Anthropic's vendor name in our
// public API surface (settings AIProvider field, frontend display).
var pricingTable = map[string]map[string]modelPrice{
	"openai": {
		"gpt-4o":      {inputPerMillionMicro: 5_000_000, outputPerMillionMicro: 20_000_000},
		"gpt-4o-mini": {inputPerMillionMicro: 150_000, outputPerMillionMicro: 600_000},
		"o1":          {inputPerMillionMicro: 15_000_000, outputPerMillionMicro: 60_000_000},
	},
	"claude": {
		"claude-3-5-haiku-20241022": {inputPerMillionMicro: 1_000_000, outputPerMillionMicro: 5_000_000},
		"claude-sonnet-4-20250514":  {inputPerMillionMicro: 3_000_000, outputPerMillionMicro: 15_000_000},
		"claude-haiku-4-5-20251001": {inputPerMillionMicro: 800_000, outputPerMillionMicro: 4_000_000},
		"claude-opus-4-7":           {inputPerMillionMicro: 15_000_000, outputPerMillionMicro: 75_000_000},
		"claude-sonnet-4-6":         {inputPerMillionMicro: 3_000_000, outputPerMillionMicro: 15_000_000},
	},
}

// CostMicroUSD returns the cost of a single AI call in micro-USD
// (USD * 1_000_000), plus a `known` flag. Floor-division means a call
// that bills less than 1 micro-USD per dimension rounds to 0 — observed
// cost is an under-report rather than over-report (we'd rather miss
// fractions than inflate).
//
// Ollama is special-cased: any model under the "ollama" provider returns
// (0, true) because the spend lives on local compute, not a paid API.
//
// Unknown (provider, model) pairs return (0, false) so the recorder can
// decide whether to drop the entry or log it with cost=0 for visibility.
func CostMicroUSD(provider, model string, inputTokens, outputTokens int) (int64, bool) {
	if provider == "ollama" {
		return 0, true
	}
	models, ok := pricingTable[provider]
	if !ok {
		return 0, false
	}
	p, ok := models[model]
	if !ok {
		return 0, false
	}
	inCost := int64(inputTokens) * p.inputPerMillionMicro / 1_000_000
	outCost := int64(outputTokens) * p.outputPerMillionMicro / 1_000_000
	return inCost + outCost, true
}
