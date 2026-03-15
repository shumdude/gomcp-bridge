# gomcp-bridge - Agent Guide

Local guide for work inside `gomcp-bridge/*`.

## Scope

- Subproject: `gomcp-bridge/`.
- Purpose: two MCP servers with isolated implementations:
  - `codex-bridge` in `gomcp-bridge/codex/`
  - `claude-bridge` in `gomcp-bridge/claude/`

## Structure

```text
gomcp-bridge/
├── codex/
│   ├── main.go                # codex-bridge MCP server (generate_prompt_md / ask_codex / fill_md_files)
│   ├── openai.go              # OpenAI-compatible Chat Completions backend
│   ├── openai_singin.go       # Codex CLI sign-in backend (`codex exec` + resume)
│   └── openai_singin_test.go  # buildExecArgs tests for Codex sign-in backend
├── claude/
│   ├── main.go                # claude-bridge MCP server (generate_prompt_md / ask_claude / fill_md_files)
│   ├── claude_signin.go       # Claude CLI sign-in backend (`claude -p` + session resume)
│   └── claude_signin_test.go  # buildPrintArgs tests for Claude sign-in backend
├── AGENTS.md
└── README.md
```

## Invariants

- For `codex-bridge`, backend selection is done only via `newChatClient()` in `codex/main.go` and `CODEX_BACKEND`:
  - `auto`: `OPENAI_API_KEY` -> API backend, otherwise sign-in backend
  - `api`: always `newOpenAIClient()`
  - `signin`: always `newOpenAISignInClient()`
- `ask_codex` keeps a shared session between calls; reset happens only after `generate_prompt_md`.
- `ask_claude` keeps a shared session between calls; reset happens only after `generate_prompt_md`.
- `ask_codex` and `ask_claude` must be called strictly sequentially (no parallel requests in the same session).
- `fill_md_files` accepts and writes only whitelisted `targets`; absolute paths and root escapes are forbidden.
- Prompts sent to the LLM (`systemPrompt*` and extra context for `ask_codex` / `ask_claude` / `fill_md_files`) must stay in English.
- Do not change the `fill_md_files` response contract: strict JSON with `files[]`.

## Environment Variables

For `codex-bridge`:
- `CODEX_BACKEND`: `auto|api|signin`
- `OPENAI_API_KEY`: required for backend `api`
- `OPENAI_BASE_URL`: optional, default `https://api.openai.com/v1`
- `OPENAI_MODEL`: optional, default `gpt-5-codex`
- `CODEX_BIN`: optional path to `codex` binary
- `CODEX_SIGNIN_MODEL`: optional model for sign-in backend

For `claude-bridge`:
- `CLAUDE_BIN`: optional path to `claude` binary
- `CLAUDE_MODEL`: optional model for Claude backend

For both:
- `PROMPT_DIR`: directory for `PROMPT.md` (default: current directory)

## What to Run After Changes

```powershell
go test ./gomcp-bridge/...
```

If backend selection logic, JSON serialization/parsing, or file writing logic was changed, add or update tests in the same change.
