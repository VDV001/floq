# LLM Style Check Tool

**Purpose:** post-generation refinement pass that catches "corporate AI smell" in
AI-generated B2B outreach copy (cold emails, Telegram messages, inbound reply
drafts, follow-ups) before it reaches a prospect.

## Why it exists

AI-generated cold copy tends to hit a narrow band of clichés that humans don't
write: "безусловно", "не побоимся этого слова", "в современном мире",
"революционный", English fillers like "delve" / "inquire" / "furthermore",
deck-style multi-clause sentences, and an absence of "я" / "мы" / direct
questions. These templates depress open- and reply-rates. The style-check pass
exists to score a draft on that axis and trigger a single retry when it falls
below the threshold.

## Where it lives

- **Prompts:** `backend/internal/ai/prompts.go` — `StyleCheckSystem`,
  `StyleCheckUser`, `StyleRetryHint`.
- **Use case:** `backend/internal/ai/style_check.go` —
  `(*AIClient).StyleCheck(ctx, draft, channel)` returns `*StyleResult{Score,
  Issues, Feedback}`.
- **Wiring:** `applyStyleCheck(ctx, draft, channel, regenFn)` is called from
  `GenerateColdMessage`, `GenerateTelegramMessage`, `DraftReply`, and
  `GenerateFollowup`. `regenFn` re-runs the original generator with
  `StyleRetryHint` appended to the user prompt.
- **Toggle:** `user_settings.ai_style_check_enabled` (BOOLEAN, default `TRUE`,
  migration 025). The composition root in `cmd/server/main.go` reads the
  owner's setting at boot and calls `AIClient.EnableStyleCheck()`.
- **Mode:** `ModelMode.Budget` — Haiku / gpt-4o-mini / local Ollama. Style
  checks dominate by volume; we trade reasoning depth for cost.

## Scoring rubric

| Score | Meaning |
| ----- | ------- |
| 10    | Lively, personal, concrete. Indistinguishable from a human writer. |
| 7–9   | Acceptable. Minor rough edges that don't warrant a rewrite. |
| 4–6   | Noticeable boilerplate or canceleryat. Retry. |
| 0–3   | "Corporate" voice / classic ChatGPT smell. Always retry. |

Threshold for retry: **< 7** (constant `StyleCheckPassThreshold = 7`).

### Penalize

- Шаблонные обороты: "безусловно", "не побоимся", "в современном мире",
  "революционный", "хотел бы предложить".
- Англицизмы / ChatGPT markers: "delve", "inquire", "furthermore", "leverage",
  "тейкэвэй".
- Канцелярит: "в связи с тем что", "осуществить", "произвести впечатление",
  "имеет место быть".
- Sentences > 30 words (deck style).
- No first-person ("я" / "мы") or no direct question.
- Marketing buzzwords: "уникальное решение", "лидер рынка", "best-in-class".

### Reward

- Specific facts: a number, a name, an industry detail.
- Direct personal address.
- Natural conversational register.
- One clear CTA or question.

## Channel-specific tuning

The user prompt passes a `channel` argument so the model can adjust strictness:

- `email` — admits a slightly more formal register.
- `telegram` — short phrases, conversational, no deck-style paragraphs.
- `reply` — must read as responsive to the incoming message.
- `followup` — terse, non-pushy.

## Retry contract

When `score < 7`:

1. `applyStyleCheck` invokes the supplied `regenFn(ctx, feedback)`.
2. The regen call clones the original `CompletionRequest` and appends
   `StyleRetryHint` (with the LLM's feedback string substituted) to the last
   user message. System prompt is unchanged.
3. The regenerated text is returned to the caller as-is. No second style pass
   — bounded retry budget of **1**.

If style check or regen errors (provider down, malformed JSON, etc.) the
**original draft is returned** and the error is logged via `slog`. Outreach
generation must not block on a style-pass outage.

## Acceptance metrics (from issue #26)

- Style check should catch ≥ 80 % of "corporate" tics on synthetic adversarial
  examples.
- Latency overhead < 2 s (one Budget-mode round-trip; ~ 400 ms p50 on Haiku).
- User can disable via `PUT /api/settings {"ai_style_check_enabled": false}`.
- Tests with mocked LLM cover: high score (no retry), low score (single
  retry), provider error (graceful original return), malformed JSON
  (graceful original return).

## Test coverage

- `internal/ai/style_check_test.go` — pure StyleCheck parsing + channel
  forwarding + ModelMode.Budget assertion.
- `internal/ai/client_test.go` — table-driven wiring for all four Generate\*
  use cases: disabled (1 call), enabled high-score (2 calls), enabled
  low-score (3 calls + regenerated draft).
- `internal/settings/style_check_settings_test.go` — repository default,
  DB-stored override, use case forwarding into the fields map.

## Future work

- LLM-judge benchmarking (promptfoo) on a held-out adversarial set to
  empirically estimate the catch-rate.
- Per-channel threshold overrides (today the constant is global).
- Surfacing the style-check verdict in the UI so SDR can see *why* a
  message was rewritten.
