# gomcp-bridge

`gomcp-bridge` is a subproject in the `gobot` monorepo with two MCP servers:

- `codex-bridge` in `gomcp-bridge/codex`
- `claude-bridge` in `gomcp-bridge/claude`

Both servers provide tools for generating `PROMPT.md`, asking follow-up questions in a shared session, and updating key project markdown files.

## Repository Layout

```text
gomcp-bridge/
├── codex/
│   ├── main.go
│   ├── openai.go
│   ├── openai_singin.go
│   └── openai_singin_test.go
├── claude/
│   ├── main.go
│   ├── claude_signin.go
│   └── claude_signin_test.go
├── AGENTS.md
└── README.md
```

## codex-bridge Tools

`codex-bridge` runs over stdio and registers 3 tools:

- `generate_prompt_md` - generates a detailed `PROMPT.md` from a user task.
- `ask_codex` - asks sequential clarifying questions within one session.
- `fill_md_files` - updates selected markdown files based on the final implementation result.

## codex-bridge Backends

`codex-bridge` supports two backend modes:

- OpenAI API (`codex/openai.go`) via `/v1/chat/completions`
- Codex CLI sign-in (`codex/openai_singin.go`) via local `codex exec` and `resume --last`

Backend selection is controlled by `CODEX_BACKEND=auto|api|signin`.

## Quick Start (Repository Root)

Run `codex-bridge`:

```powershell
go run ./gomcp-bridge/codex
```

Run `claude-bridge`:

```powershell
go run ./gomcp-bridge/claude
```

## Environment Variables

For `codex-bridge`:

- `CODEX_BACKEND`: `auto|api|signin` (default `auto`)
- `OPENAI_API_KEY`: OpenAI API key (required for `api`)
- `OPENAI_BASE_URL`: OpenAI-compatible API base URL (default `https://api.openai.com/v1`)
- `OPENAI_MODEL`: model for API backend (default `gpt-5-codex`)
- `CODEX_BIN`: path to `codex` binary (if not found in `PATH`)
- `CODEX_SIGNIN_MODEL`: model for sign-in backend (default `gpt-5-codex`)

For `claude-bridge`:

- `CLAUDE_BIN`: path to `claude` binary (if not found in `PATH`)
- `CLAUDE_MODEL`: model for Claude CLI backend (default `sonnet`)

For both:

- `PROMPT_DIR`: output directory for `PROMPT.md` (default: current working directory)

## Development and Tests

```powershell
go test ./gomcp-bridge/...
```

## Constraints and Behavior

- `ask_codex` and `ask_claude` work as one continuous session until the next `generate_prompt_md`.
- `fill_md_files` accepts only JSON output and writes only files from the allowed `targets` list.
- `codex-bridge` sign-in backend requires Codex CLI authentication (`codex login`).
- `claude-bridge` backend requires Claude CLI authentication (`claude auth login`).
