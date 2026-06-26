# Auto-enrichment фаза 3 — Registry-источник (DaData) за портом `Enricher`

**Дата:** 2026-06-26
**База:** `refactor/clean-architecture` @ v0.56.0, БД миграция 049, свободная — 050
**Парадигма:** TDD + DDD + Clean Architecture (механические гейты)
**Issue:** инкремент 2 фазы 2 (#186 закрыт инкр.1) → создать свежий issue.

> Терминология: «инкремент 2» из плана = здесь зовём **фаза 3** (отдельный релиз, per-feature). Источник — **DaData.ru** (официальный API), НЕ скрейпинг Rusprofile/HH (ToS/anti-bot/SSRF).

---

## 0. Решение об источнике (senior)

**DaData.ru API** вместо скрейпинга реестров:
- `/suggest/party` — fuzzy-матч `название → компания`, сразу отдаёт ИНН/ОГРН/адрес/ОКВЭД/статус. **Решает проблему матча без ИНН** (центральный риск инкр.2).
- Официальный API, фикс. хост (`suggestions.dadata.ru`) + ключ в заголовке `Authorization: Token <key>` → **URL не выводится из недоверенного ввода → нет SSRF-поверхности** фазы 1. ToS соблюдён. Free-tier 10k запросов/день.
- Отвергнуто: скрейп rusprofile/HH (ToS, anti-bot, хрупкий HTML, реальная SSRF); ФНС egrul.nalog.ru (бесплатно/официально, но async/PDF-выдача, матч надёжен только по ИНН).

---

## 1. Цель
Обогатить enrichment-профиль **юридическими реквизитами** (ИНН/ОГРН/адрес/ОКВЭД/офиц.название/статус) сверх website-скрейпинга. Ship dark за флагом + ключом.

## 2. Архитектура (DDD + Clean Arch)

### 2.1 Новый порт `Enricher` (в `enrichment`, DIP у потребителя)
```go
// EnrichQuery — сигналы идентичности из уже собранного.
type EnrichQuery struct {
    INN         string // если удалось извлечь со страницы (точный матч)
    CompanyName string // profile.Title (fuzzy-матч)
}
type Enricher interface {
    // Enrich возвращает реквизиты или miss (found=false) — НЕ угадывает.
    Enrich(ctx context.Context, q EnrichQuery) (domain.LegalDetails, bool, error)
}
```
Ортогонален `Extractor`: Extractor работает над страницей; Enricher сам ходит во внешний реестр по идентичности. DaData-адаптер реализует порт в `cmd/server` (cross-module: `enrichment` не импортит http-DaData-специфику... адаптер в composition root, как `Completer`).

### 2.2 Domain — VO с инвариантами (настоящий DDD)
```go
// INN — ВО с checksum-валидацией (10 цифр ЮЛ / 12 ИП, контрольные разряды).
type INN struct{ value string }
func NewINN(s string) (INN, error)   // ErrInvalidINN при неверной длине/контрольной сумме
// OGRN — аналогично (13/15 цифр + контрольный разряд).
type OGRN struct{ value string }
func NewOGRN(s string) (OGRN, error)

type LegalDetails struct {
    INN      string `json:"inn,omitempty"`
    OGRN     string `json:"ogrn,omitempty"`
    FullName string `json:"fullName,omitempty"` // офиц. с ОПФ
    Address  string `json:"address,omitempty"`
    OKVED    string `json:"okved,omitempty"`
    Status   string `json:"status,omitempty"`   // ACTIVE/LIQUIDATING/...
}
func (d LegalDetails) IsEmpty() bool
```
- `LegalDetails` кладётся в `CompanyProfile` JSONB новым полем `Legal LegalDetails json:"legal,omitempty"` → **backward-compat, миграции НЕ нужно** (как поля инкр.1). Свободная миграция 050 остаётся.
- ИНН/ОГРН checksum — доменный инвариант, живёт ТОЛЬКО в domain (`var ErrInvalidINN`). Извлечённый со страницы или вернувшийся от DaData ИНН валидируется через `NewINN` перед записью.

### 2.3 Матч-стратегия (честная, не угадываем)
В usecase-шаге (после Extract, есть `page` + `profile.Title`):
1. **ИНН со страницы:** regex `\b\d{10}\b|\b\d{12}\b` рядом с «ИНН» в `page` → `NewINN` (checksum) → если валиден, DaData `findById/party` (точный матч, high confidence).
2. **Иначе fuzzy по названию:** DaData `suggest/party` с `profile.Title`. Берём топ-хит ТОЛЬКО при высокой уверенности (единственный результат ИЛИ точное совпадение названия). Несколько неоднозначных → **skip** (ложные реквизиты хуже их отсутствия).
3. Пусто → miss, не ошибка.

### 2.4 Поток (usecase) — новый опциональный шаг
`processOne`: fetch → extract → **[если enricher!=nil] enrich(query из page+profile) → merge LegalDetails в profile** → save. Enricher best-effort: ошибка/miss логируется и проглатывается (graceful degrade, как LLM) — реквизиты не должны ронять website-профиль. Usecase получает опциональную зависимость `enricher Enricher` (nil = выключено).

## 3. Безопасность — threat-model egress (на дизайне)
- **Новый egress?** Да — DaData. Хост **константный и доверенный** (`suggestions.dadata.ru`), НЕ выводится из email/страницы (в отличие от website-fetcher ф.1). → **SSRF-поверхности нет** (нет URL-инъекции; стандартный https-клиент достаточен, guarded-dialer не нужен — обосновать в коде).
- **Недоверенный вход:** company name / извлечённый ИНН уходят в DaData в теле POST (не в host) — это безопасный query. ИНН валидируется checksum'ом до запроса.
- **Секрет:** `DADATA_API_KEY` из env (как `OPENAI_API_KEY`); пустой ключ → enricher выключен даже при флаге (ship dark). Ключ — в заголовке, не логируется.
- **PII:** реквизиты компаний — публичные данные ЕГРЮЛ, не персональные. Адрес ЮЛ публичен.

## 4. Cost / observability
DaData — не AI-вызов → **НЕ шоехорнить в audit_log** (он AI-специфичен: CallMeta/RequestType). Вместо: (a) per-domain rate-limit (переиспользовать `RateLimiter`), (b) флаг-гейт + free-tier квота DaData (свой дашборд), (c) metric-counter вызовов/попаданий (Prometheus, как drops). v1 достаточно.

## 5. Config / flag
- `ENRICHMENT_REGISTRY_ENABLED` (default **off**, ship dark).
- `DADATA_API_KEY` (env, secret). Пустой → enricher = nil (выключено) даже при флаге.
- `ENRICHMENT_REGISTRY_RATE_LIMIT_PER_MIN` (default напр. 30).

## 6. UI
`LegalDetails` едет в `profile.legal` JSON → секция «Реквизиты» в `EnrichmentCard` (ИНН/ОГРН/адрес/ОКВЭД/статус). `EnrichmentProfile` TS-тип + DTO `ProfileResponse` расширяются (optional поля).

## 7. TDD-план (RED→GREEN по слоям)
1. `domain`: `NewINN` checksum (table-driven: валид ЮЛ/ИП, битая контрольная, длина, не-цифры) → feat.
2. `domain`: `NewOGRN` checksum → feat.
3. `domain`: `LegalDetails` + `IsEmpty` + поле в `CompanyProfile` (JSONB round-trip в integration) → feat.
4. `enrichment`: ИНН-extract-from-page helper (regex + checksum) → feat.
5. `enrichment`: матч-стратегия в usecase (fake Enricher): ИНН-точный → findById; иначе fuzzy; неоднозначн.→skip; miss/err→degrade → feat. (новый порт `Enricher`).
6. `cmd/server`: DaDataEnricher адаптер (fake http) — suggest/findById, парс ответа, Token-заголовок, rate-limit, пустой ключ→nil → feat.
7. `dto` + frontend: реквизиты в API + `EnrichmentCard` → feat.
8. config flag + wiring.

## 8. DoD
≥1 рабочий Enricher (DaData) за портом; реквизиты в UI; ship dark (флаг+ключ); матч честный (skip при неоднозначности); threat-model egress задокументирован; rate-limit; code-review все оси ≥8; фазы 1-2 не сломаны (флаг off = байт-в-байт).

## 9. Риски
| Риск | Решение |
|------|---------|
| Ложный матч (не та компания) | ИНН-точный приоритетно; fuzzy только high-confidence; иначе skip |
| Утечка/стоимость DaData | флаг off + пустой ключ→выкл + rate-limit + free-tier квота |
| DaData недоступна | best-effort: miss/err проглатывается, website-профиль цел |
| Backward-compat | `legal` в JSONB omitempty, zero-value для старых строк |
| Cross-module | DaData-адаптер в cmd/server, `enrichment` чист |
