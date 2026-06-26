# AI model selection — `ModelMode` mapping

Введено в v0.9.0 (PR `feat/ai-model-mode`). Заменяет хардкоды моделей на пер-call-site объявление *намерения* (Plan / Execute / Budget).

---

## Зачем

До v0.9.0 каждый `*ai.AIClient` метод получал модель через одну точку конфигурации (`cfg.AIModel`). Все вызовы (Qualify, DraftReply, GenerateColdMessage, GenerateCallBrief...) шли через **одну и ту же модель**. Для квалификации и для синтеза CallBrief — одна и та же.

Эмпирическое подтверждение проблемы: NGirchev cross-review (habr.com/ru/articles/1034452/) показал, что сильные модели (GPT-5.5, Opus 4.6) без контроля «под какую задачу» создают «архитектурно красивый код, который не запускался», в то время как Codex 5.3 (более «execute»-ориентированный) справлялся лучше для задач исполнения.

Вывод: разделить **plan-mode** (исследование, синтез, длинная цепочка рассуждений) и **execute-mode** (короткий структурированный ответ на известный вход) явно в коде.

---

## Три режима

| Mode | Что это | Когда выбирать | Латентность | Стоимость |
|---|---|---|---|---|
| `ModelModeExecute` | **default**. Короткий структурированный ответ на известный вход. | Лид-квалификация, реплай-драфт, холодное письмо по шаблону, ответ на TG. | низкая | средняя |
| `ModelModePlan` | Синтез из множественных источников, длинное рассуждение, outline. | CallBrief из истории переписки, анализ возражений, outline для коммерческого предложения. | высокая | высокая |
| `ModelModeBudget` | Bulk-классификация, тегирование, style-pass. Качество жертвуется ради цены. | Style-check (issue #26), классификация intent, спам-фильтр. | средняя | низкая |

Zero-value поля `CompletionRequest.Mode` равен `ModelModeExecute` — **дефолт безопасный**: новый AI-вызов без явного режима получит execute-модель, не самую дорогую.

---

## Mapping per провайдер (v0.9.0)

| Provider | Plan | Execute | Budget |
|---|---|---|---|
| Claude | `claude-opus-4-7` | `claude-sonnet-4-6` | `claude-haiku-4-5-20251001` |
| OpenAI | `o1` | `gpt-4o` | `gpt-4o-mini` |
| Ollama | (configured local model) | (configured local model) | (configured local model) |
| Groq / Together (OpenAI-compatible) | (configured override) | (configured override) | (configured override) |

**Точка ротации:** `backend/internal/ai/providers/{claude,openai,ollama}.go` — переменные `claudeModelByMode`, `openaiModelByMode`. Когда выйдет новое поколение — меняется одно место.

### Override через user settings

Если в `user_settings.ai_model` задана конкретная модель (например, `gpt-4o-mini` для экономии) — она используется **для всех режимов**, mode-mapping игнорируется. Эта модель — sentinel «пользователь знает, что делает».

Семантика: `overrideModel != "" → wins`. Хочешь mode-aware mapping — оставь `ai_model` пустым в settings.

### Ollama особенность

Ollama хостится локально. Типично — одна модель в памяти за раз. Переключение между моделями = unload/load = 5-30 сек. Поэтому Ollama-провайдер игнорирует `Mode` и всегда возвращает свою configured-модель. Если хочешь Plan/Execute/Budget разделение через Ollama — поднимай несколько инстансов на разных портах.

---

## Mapping per use case (v0.9.0)

| Use case | Метод | Mode | Почему |
|---|---|---|---|
| Лид-квалификация (структурированный JSON) | `AIClient.Qualify` | Execute | известный вход, schema-constrained ответ |
| Драфт ответа на входящее | `AIClient.DraftReply` | Execute | респонс к известному контексту |
| Followup для stale-лидов | `AIClient.GenerateFollowup` | Execute | короткий шаблонный текст |
| Cold outreach письмо | `AIClient.GenerateColdMessage` | Execute | template-driven, не synthesis |
| Cold outreach в Telegram | `AIClient.GenerateTelegramMessage` | Execute | то же |
| Реплай в TG-диалоге | `AIClient.GenerateTelegramReply` | Execute | респонс на конкретное сообщение |
| Brief для звонка из истории | `AIClient.GenerateCallBrief` | **Plan** | синтез из переписки → outline |

**Будущие use case'ы:**
- Style-check pass (issue #26) → `Budget`
- Анализ возражений в long email-thread → `Plan`
- Outline для КП → `Plan`
- Intent-классификация входящего → `Budget`

---

## Стоимость per request (приблизительно, 2026-05)

Усреднённый Floq-вызов ≈ 2K input tokens + 1K output tokens.

| Mode | Claude | OpenAI |
|---|---|---|
| Plan | ~$0.04-0.08 (Opus 4.7) | ~$0.06-0.12 (o1) |
| Execute | ~$0.008-0.015 (Sonnet 4.6) | ~$0.005-0.010 (gpt-4o) |
| Budget | ~$0.001-0.002 (Haiku 4.5) | ~$0.0001-0.0002 (gpt-4o-mini) |

Проверять актуальные цены на anthropic.com/pricing и platform.openai.com/docs/pricing — цифры выше — порядок величины, не SLA.

---

## Fallback цепочка

В рамках одного провайдера: если configured-модель недоступна (deprecated, региональная блокировка), API возвращает 4xx → `Provider.Complete` пробрасывает `error` → caller получает `"ai qualify: ..."`. **Авто-fallback на другую модель в рамках одного провайдера v0.9.0 не реализован** — это P2 из integration-audit (`docs/integration-audit-2026-05-15.md` §6).

Между провайдерами: смена провайдера — через `cfg.AIProvider` в settings (вручную или через UI). Авто-failover (Anthropic down → OpenAI) — отдельная задача (требует carefully thinking про consistency: разные провайдеры дают разный output для одного и того же prompt).

---

## Тестирование

- `backend/internal/ai/mode_test.go::TestModelMode_String` — enum stringify
- `backend/internal/ai/mode_test.go::TestAIClient_MethodModeMapping` — table-driven, 7 use case'ов × ожидаемый mode
- `backend/internal/ai/providers/mode_test.go::TestClaudeProvider_ModelForMode` — provider mapping
- `backend/internal/ai/providers/mode_test.go::TestClaudeProvider_OverrideModelWinsOverMode` — override precedence
- Аналогично для OpenAI, Ollama

`mockProvider` → `modeRecordingProvider`: записывает `req.Mode` для проверки, что AIClient метод выставляет правильный режим.

---

## Что НЕ покрыто этой задачей

- Сами prompt-templates (см. `backend/internal/ai/prompts.go`) — отдельная задача (issue #26 для style-check)
- Авто-failover между провайдерами — P2 из integration-audit
- A/B-тестинг моделей per use case с автоматическим выбором — будущая задача (требует quality metrics)

---

## Migration notes (v0.8.x → v0.9.0)

**Breaking change в Go API:** `providers.NewClaudeProvider` теперь принимает `(apiKey, overrideModel, httpClient)` — добавился второй параметр `overrideModel`. Все call site'ы внутри проекта обновлены (`cmd/server/helpers.go`, `providers/dynamic.go`).

**Behaviour change:** Ранее Claude всегда использовал `claude-3-7-sonnet-latest` (хардкод в провайдере). После миграции:
- Если `cfg.AIModel == ""` → дефолт стал `claude-sonnet-4-6` (Execute mode)
- Если `cfg.AIModel != ""` → используется как раньше (override winning)
- Для Plan-вызовов (только `GenerateCallBrief` сейчас) → `claude-opus-4-7`, **дороже на ~5x** на этом конкретном вызове

**Action needed:** если пилотный клиент чувствителен к costs — выставить `cfg.AIModel = "claude-haiku-4-5-20251001"` чтобы заставить везде Haiku (override). После cost-стабилизации — снять override и пользоваться mode-aware mapping.
