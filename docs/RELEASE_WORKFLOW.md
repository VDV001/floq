# Release Workflow

Полный путь от фичи до зарелизенного `main`. Делать ровно по шагам.

Базовые правила проекта:
- **Никогда не работать/коммитить прямо в `main`.** Только через ветку → PR.
- **База всегда `main`**, PR `--base main`, слияние — **Squash and merge** (без «Delete branch»).
- **TDD:** на каждое поведенческое изменение — два коммита (`test:` RED → `feat:` GREEN). Рефактор — отдельным `refactor:` без смены поведения.
- **Скрипт `bin/release.sh` — только для выпуска версии**, не для разработки фич.

---

## Фаза 1. Разработка фичи (TDD)

### 1. Ветка от свежего main
```bash
git checkout main && git fetch origin && git reset --hard origin/main
git checkout -b fix/<issue>-<короткое-имя>     # напр. fix/227-ollama-timeout
```

### 2. TDD-цикл — два коммита на каждое поведенческое изменение
```bash
# a) падающий тест → запуск → видишь RED → коммит
go test ./...            # или: npx vitest run <file>   (должен упасть)
git add <тест-файл>
git commit -m "test(scope): add failing test for X"

# b) реализация → запуск → видишь GREEN → коммит
go test ./...            # теперь зелёный
git add <код>
git commit -m "feat(scope): implement X"
```
Правила:
- Смена сигнатуры / перестановка кода без изменения поведения — отдельным `refactor:` (тесты остаются зелёными).
- Перед каждым коммитом: `git branch --show-current` — убедиться, что не на `main`.
- `feat:` вместе с тестами в одном коммите = провал TDD.
- Table-driven tests обязательны при ≥3 вариантах одной проверки.

### 3. Прогнать всё и запушить ветку
```bash
go build ./... && go test ./...                              # backend
cd frontend && npx vitest run && npx tsc --noEmit && cd ..   # frontend
git push -u origin fix/227-ollama-timeout
```

---

## Фаза 2. PR и ревью

### 4. Открыть PR на main
```bash
gh pr create --base main --head fix/227-ollama-timeout \
  --title "fix(...): ... (#227)" --body "...Closes #227"
```
В веб-форме GitHub подставит PR-шаблон (`.github/pull_request_template.md`) — пройти по чеклисту (TDD/DDD/Clean Architecture, прогон тестов, ревью).

### 5. Ревью
Независимая проверка — агент `superpowers:code-reviewer` (оценка 1–10 по осям, файлы+строки). Обязательные правки → чинить по TDD **в той же ветке** → `git push` (PR обновится сам, номер PR для этого не нужен).

### 6. Squash-merge
Кнопка на GitHub → **Squash and merge**. **Не** нажимать «Delete branch».

---

## Фаза 3. Синхронизация после merge

### 7. Обновить локальный main
Squash даёт `main` **новый sha**, которого нет в истории ветки, поэтому `reset`, а **не** `pull` (`pull` попытается склеить старую и новую историю → дубли/конфликты):
```bash
git checkout main && git fetch origin && git reset --hard origin/main
```

На этом фича в `main`. Если релиз не нужен — стоп здесь.

---

## Фаза 4. Релиз (`bin/release.sh`)

Скрипт нужен **только для выпуска версии**. Версия — следующий минор (`0.77.0` → `0.78.0`).

### 8. Bump — скрипт сам делает ветку и PR (в main не пушит)
```bash
bin/release.sh bump 0.78.0 -m "краткое описание релиза (#227)"
```
Создаёт ветку `chore/bump-v0.78.0`, бампает 4 sync-точки (`VERSION`, README-бейдж, `frontend/package.json`, `frontend/package-lock.json`), добавляет строку в `CHANGELOG.md`, коммитит, пушит ветку, открывает PR.
`-m` — строка для CHANGELOG (если пропустить — вставит `TODO`-заглушку и предупредит).

### 9. Смержить bump-PR
Squash-merge, без delete-branch.

### 10. Синхр main и опубликовать — скрипт тегает + release (в main не пушит)
```bash
git checkout main && git fetch origin && git reset --hard origin/main
bin/release.sh publish 0.78.0                 # заметки в $EDITOR (предзаполнены из git-лога)
# либо без редактора:
bin/release.sh publish 0.78.0 --generate-notes
```
Результат: аннотированный тег `v0.78.0` + GitHub Release.

---

## Что делает скрипт, а что нет

| | |
|---|---|
| **Делает** | bump 4 точек + CHANGELOG, ветку+PR (`bump`); тег+push тега+GitHub Release (`publish`) |
| **НЕ делает** | не пишет код, не коммитит фичи, **не пушит в main напрямую** |

Предохранители (обе фазы откажутся, если):
- не на `main` / дерево грязное / локальный `main` рассинхрон с origin;
- тег `vX.Y.Z` уже существует;
- `publish`: `VERSION` ≠ `X.Y.Z` → «сначала смержи bump-PR».

`--dry-run` у `bump` и `publish` — печатает план, ничего не меняя. Прогнать при сомнениях.

---

## Шпаргалка

| Задача | Как |
|---|---|
| Код фичи/фикса | ветка + TDD-коммиты, `go test` / `vitest` |
| Влить в main | PR `--base main` → Squash and merge |
| Выпустить версию | `bin/release.sh bump X.Y.Z` → merge → `publish X.Y.Z` |

Запомнить:
- Перед коммитом — `git branch --show-current` (не `main`).
- После squash-merge — `git reset --hard origin/main`, **не** `git pull`.
- `test:` (RED) и `feat:` (GREEN) — раздельно; рефактор — свой `refactor:`.
