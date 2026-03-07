# gomcp-bridge

`gomcp-bridge` is a subproject in the `gobot` monorepo that implements the `codex-bridge` MCP server.
The server provides tools for generating `PROMPT.md`, asking follow-up questions to Codex, and automatically updating key project markdown files.

## What the Server Does

The server runs over stdio and registers 3 tools:

- `generate_prompt_md` - generates a detailed `PROMPT.md` from a user task.
- `ask_codex` - asks sequential clarifying questions within one session.
- `fill_md_files` - updates selected markdown files based on the final implementation result.

## Backends

`gomcp-bridge` supports two backend modes:

- OpenAI API (`openai.go`) via `/v1/chat/completions`
- Codex CLI sign-in (`openai_singin.go`) via local `codex exec` and `resume --last`

Backend selection is controlled by `CODEX_BACKEND=auto|api|signin`.

## Quick Start

From the repository root:

```powershell
go run ./gomcp-bridge
```

## Environment Variables

- `CODEX_BACKEND`: `auto|api|signin` (default `auto`)
- `OPENAI_API_KEY`: OpenAI API key (required for `api`)
- `OPENAI_BASE_URL`: OpenAI-compatible API base URL (default `https://api.openai.com/v1`)
- `OPENAI_MODEL`: model for API backend (default `gpt-5-codex`)
- `CODEX_BIN`: path to `codex` binary (if not found in `PATH`)
- `CODEX_SIGNIN_MODEL`: model for sign-in backend (default `gpt-5-codex`)
- `PROMPT_DIR`: output directory for `PROMPT.md` (default: current working directory)

## Development and Tests

```powershell
go test ./gomcp-bridge/...
```

## Constraints and Behavior

- `ask_codex` works as one continuous session until the next `generate_prompt_md`.
- `fill_md_files` accepts only JSON output and writes only files from the allowed `targets` list.
- The `signin` backend requires Codex CLI authentication (`codex login`).
