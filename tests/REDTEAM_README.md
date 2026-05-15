# Floq inbox redteam corpus

Канонический набор сценариев атак для проверки `InputFirewall`. Используется CI-gate'ом `.github/workflows/redteam.yml` для блокировки merge при регрессии.

---

## Запуск локально

```bash
cd backend
go test -count=1 -v ./internal/ai/security/... -run Redteam
```

Все сценарии должны пройти. Если что-то падает — либо новый pattern сломал существующий сценарий, либо изменилась нумерация (см. ниже).

## Запуск в CI

`.github/workflows/redteam.yml` запускается на каждый PR в main и на push в feat/fix-ветки. Падение → блок merge.

## Дополнительно: promptfoo (LLM-judge)

`tests/redteam.yaml` совместим с [promptfoo](https://promptfoo.dev) format. Для запуска через promptfoo с реальным LLM-judge'ем (для adversarial-testing второй фазы):

```bash
npx promptfoo redteam run --config tests/redteam.yaml --ci
```

**Pilot v0.10.0 не использует promptfoo** — runner работает через `go test` против `InputFirewall`. promptfoo добавляется во второй итерации, когда нужно тестировать full LLM ответ (jailbreak пройдённый firewall'ом → проверка что Claude/GPT не выдал секреты).

---

## Структура корпуса

30 сценариев, 3 класса по 10:

| Класс | ID | Описание | Expected verdict |
|---|---|---|---|
| C1 (jailbreak) | C1.01–C1.10 | Prompt injection / role override / encoded payloads | block |
| C2 (data exfiltration) | C2.01–C2.10 | System prompt extraction (7), data forwarding (3) | block × 7, warn × 3 |
| C3 (social engineering) | C3.01–C3.10 | Scam relay, phishing, **negative controls** | block × 4, warn × 2, info × 4 |

**Negative controls (C3.01–C3.04, C3.10)** — реальные benign-сообщения от потенциальных лидов. Проверяют, что firewall **НЕ ложно-срабатывает** на нормальный inbound. Это критически важно: жёсткий firewall, режущий 30% реальных лидов, бесполезен.

---

## Как добавить новый сценарий

1. Открой `tests/redteam.yaml`
2. Добавь блок в секцию правильного класса (или начни новый класс при необходимости):
   ```yaml
   - description: "C1.11 краткое описание атаки"
     vars:
       payload: "текст атаки"
       language: en | ru | mixed
       expected_verdict: block | warn | info
       expected_pattern: имя_pattern_из_input_firewall.go  # optional
   ```
3. Запусти `go test -count=1 -run Redteam ./internal/ai/security/...` — должно пройти
4. Если не проходит:
   - Сценарий правильно классифицирован, а firewall пропускает → нужно добавить/расширить pattern в `input_firewall.go` (TDD-парой: failing test + fix)
   - Сценарий неверно сформулирован → исправить в YAML

### Правило: каждый новый pattern → 2 сценария

При добавлении нового регекса в `InputFirewall.patterns` обязательно:
- **Один positive** сценарий (block/warn) — проверяет что pattern срабатывает
- **Один negative-control** сценарий в C3 (info) — реальный benign-текст, который похоже на pattern, но не должен блокироваться

Это держит false-positive rate видимым.

---

## Метрики (pilot v0.10.0)

После 4 недель в проде:
- Attack pass rate: % реальных attacks, прошедших firewall (manual review всех Block-/Warn-flagged inbound) — target ≤10%
- False positive rate: % real leads (выборка 100 случайных Allowed-info) ошибочно классифицированных как block/warn — target 0%
- Latency overhead: p99 InputFirewall.Scan() — target <5ms

Отчёт о метриках — в `docs/security-pilot-results.md` после первой недели.

---

## Ссылки

- `docs/security-model.md` — threat model, MITRE/OWASP mapping
- `backend/internal/ai/security/input_firewall.go` — implementation
- `backend/internal/ai/security/redteam_test.go` — runner
- KB `agent-security-defaults v1.0`
- Doubletapp Meridian (50% benchmark): habr.com/ru/companies/doubletapp/articles/1034976/
