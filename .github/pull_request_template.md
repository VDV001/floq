## Что сделано

<!-- Кратко: что и зачем -->

## Архитектурные гейты (TDD + DDD + Clean Architecture)

**TDD**
- [ ] Каждое поведенческое изменение — **два коммита** подряд: сначала `test(scope): add failing test for X` (красный), потом `feat(scope): implement X` (зелёный).
- [ ] Новый код не покрыт "задним числом" — если так, честный commit `test: backfill coverage for X`, не `feat`.
- [ ] Table-driven tests использованы при ≥3 вариантах одной проверки.

**DDD**
- [ ] Новые entity/VO в `domain/` создаются через конструктор `NewXxx(...) (*Xxx, error)` с валидацией инвариантов.
- [ ] Прямое создание `&domain.X{...}` вне `domain/` отсутствует.
- [ ] Бизнес-валидация живёт в `domain/`, не дублируется в usecase/parser/handler.
- [ ] Доменные ошибки — типизированные `var ErrXxx = errors.New(...)`, `errors.Is` работает.
- [ ] Нет мёртвого кода в `domain/` (событий/интерфейсов без подключения).

**Clean Architecture**
- [ ] Handler thin: парсинг → usecase → маппинг. Нет `uuid.New()` / `time.Now()` / прямых вызовов repo / ownership-проверок.
- [ ] Нет cross-module импортов (`modules/X` → `modules/Y`). Межмодульная связь — через адаптеры в `main.go`.
- [ ] UI-строки живут в `handler/messages` или `llm/responses`, не в usecase.
- [ ] Фоновые goroutines принимают `context.Context` и останавливаются по cancel.

## Проверка

- [ ] `cd backend && go test ./...` — зелёный
- [ ] `cd backend && go build ./cmd/server/` — компилируется
- [ ] `cd frontend && npm run lint && npm run build` — зелёный

## Независимое ревью (обязательно для фичевых PR)

- [ ] Запущен `superpowers:code-reviewer` с жёстким промптом ("оценка 1-10 по TDD/DDD/Clean, без комплиментов, файлы+строки")
- [ ] Каждая ось **≥8/10**
- [ ] Отчёт ревью приложен комментарием к PR

## Тест-план

- [ ] ...
