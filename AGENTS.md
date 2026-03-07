# gomcp-bridge - Agent Guide

Local guide for work inside `gomcp-bridge/*`.

## Scope

- Subproject: `gomcp-bridge/`.
- Purpose: MCP server `codex-bridge` that proxies tasks to an LLM via:
  - OpenAI API backend (`openai.go`)
  - Codex CLI sign-in backend (`openai_singin.go`)

## Structure

```text
gomcp-bridge/
├── main.go                # MCP server, tools: generate_prompt_md / ask_codex / fill_md_files
├── openai.go              # Chat Completions client for OpenAI-compatible API
├── openai_singin.go       # Backend via `codex exec` + session resume
└── openai_singin_test.go  # buildExecArgs tests for sign-in backend
```

## Invariants

- Backend selection is done only via `newChatClient()` and `CODEX_BACKEND`:
  - `auto`: `OPENAI_API_KEY` -> API backend, otherwise sign-in backend
  - `api`: always `newOpenAIClient()`
  - `signin`: always `newOpenAISignInClient()`
- `ask_codex` keeps a shared session between calls; reset happens only after `generate_prompt_md`.
- `ask_codex` must be called strictly sequentially (no parallel requests in the same session).
- `fill_md_files` accepts and writes only whitelisted `targets`; absolute paths and root escapes are forbidden.
- Prompts sent to the LLM (`systemPrompt*` and extra context for `ask_codex`/`fill_md_files`) must stay in English.
- Do not change the `fill_md_files` response contract: strict JSON with `files[]`.

## Environment Variables

- `CODEX_BACKEND`: `auto|api|signin`
- `OPENAI_API_KEY`: required for backend `api`
- `OPENAI_BASE_URL`: optional, default `https://api.openai.com/v1`
- `OPENAI_MODEL`: optional, default `gpt-5-codex`
- `CODEX_BIN`: optional path to `codex` binary
- `CODEX_SIGNIN_MODEL`: optional model for sign-in backend
- `PROMPT_DIR`: directory for `PROMPT.md` (default: current directory)

## What to Run After Changes

```powershell
go test ./gomcp-bridge/...
```

If backend selection logic, JSON serialization/parsing, or file writing logic was changed, add or update tests in the same change.

