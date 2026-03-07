# gomcp-bridge

`gomcp-bridge` - это подпроект реализующий MCP-сервер `codex-bridge`.
Сервер предоставляет инструменты для генерации `PROMPT.md`, уточняющих вопросов к Codex и автоматического обновления ключевых markdown-файлов проекта.

## Что делает сервер

Сервер поднимается по stdio и регистрирует 3 инструмента:

- `generate_prompt_md` - генерирует детальный `PROMPT.md` по задаче пользователя.
- `ask_codex` - задает последовательные уточняющие вопросы в рамках одной сессии.
- `fill_md_files` - обновляет выбранные markdown-файлы на основе финального результата работы.

## Backends

`gomcp-bridge` поддерживает два backend-режима:

- OpenAI API (`openai.go`) через `/v1/chat/completions`
- Codex CLI sign-in (`openai_singin.go`) через локальный `codex exec` и `resume --last`

Выбор backend идет через `CODEX_BACKEND=auto|api|signin`.

## Быстрый старт

Из корня репозитория:

```powershell
go run ./gomcp-bridge
```

## Переменные окружения

- `CODEX_BACKEND`: `auto|api|signin` (default `auto`)
- `OPENAI_API_KEY`: ключ OpenAI (обязателен для `api`)
- `OPENAI_BASE_URL`: базовый URL OpenAI-compatible API (default `https://api.openai.com/v1`)
- `OPENAI_MODEL`: модель для API backend (default `gpt-5-codex`)
- `CODEX_BIN`: путь к `codex` binary (если не найден в `PATH`)
- `CODEX_SIGNIN_MODEL`: модель для sign-in backend (default `gpt-5-codex`)
- `PROMPT_DIR`: директория записи `PROMPT.md` (default текущая рабочая директория)

## Разработка и тесты

```powershell
go test ./gomcp-bridge/...
```

## Ограничения и поведение

- `ask_codex` работает как одна непрерывная сессия до следующего `generate_prompt_md`.
- `fill_md_files` принимает только JSON-ответ и пишет только файлы из разрешенного списка `targets`.
- При backend `signin` требуется авторизация Codex CLI (`codex login`).

