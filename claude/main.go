package main

import (
	"context"
	"fmt"
	"github.com/mark3labs/mcp-go/server"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

const systemPromptMD = `You are a technical architect and lead software engineer.
Your task is to produce a detailed PROMPT.md file for another AI agent (Codex CLI),
which will implement the task.

PROMPT.md must not be a generic task description; it must be an actionable instruction set for Codex CLI.
It must explicitly require the following workflow:

1. Before any code changes, read all relevant markdown instructions and project reference files.
2. Mandatory starting files: AGENTS.md, MEMORY.md, and md_files/maintain/INFO.md.
3. If working inside a subproject, also read local AGENTS.md and other relevant .md files in affected directories.
4. Only after reading documentation, move to questions, analysis, and implementation.
5. All ask_claude calls after the current generate_prompt_md must stay within one shared Claude-bridge session.
6. Ask questions through ask_claude strictly sequentially: one question per call, only after receiving the previous answer, with no parallel calls.
7. After implementation, run all relevant tests and checks for the affected part of the project.
8. If tests were not run, failed, or ended with errors, the task is not complete.
9. Before final completion, call the fill_md_files tool and update key markdown files based on completed work.
10. After task completion, rename the current PROMPT.md by appending date and time in YYYY-MM-DD_HH-mm-ss format and move it to md_files/prompts/old.
11. If md_files/prompts/old does not exist, create this directory first.

PROMPT.md must include:

1. Short task description (1-2 paragraphs)
2. Goals and expected outcome
3. Mandatory preparation checklist before implementation
4. Technical stack and constraints
5. File/package structure (if applicable)
6. Key implementation requirements
7. Clarifying questions that must be answered before coding
8. Input/output examples (if applicable)
9. What NOT to do (anti-patterns, constraints)
10. Mandatory checks and tests before task completion
11. Final post-completion actions, including fill_md_files and PROMPT.md archival

Return only the PROMPT.md content with no extra text.
Use Markdown formatting.
Write concrete instructions and checklists so Codex CLI can execute them without guessing.`

const systemPromptQA = `You are a technical expert and lead software engineer.
You help Codex CLI gather all required implementation details.
Answer questions precisely, concretely, and directly.
If a question is ambiguous, provide the most reasonable option and explain your choice.`

const systemPromptFillMDFiles = `You are a technical writer and lead software engineer.
Your task is to update project markdown files based on already completed Codex CLI work.

Input you receive:
1. Final implementation result
2. List of markdown files and their current contents

Return only a JSON object without markdown wrappers and without explanations in this format:
{
  "files": [
    {
      "path": "relative/path/to/file.md",
      "content": "full new file content"
    }
  ]
}

Rules:
1. Return full updated text for each provided file.
2. Preserve each file's structure and intent.
3. Update only what is actually implied by the final implementation result.
4. Do not invent facts not present in the input.
5. For MEMORY.md, record short stable conclusions and operational notes.
6. For INFO.md, record detailed technical behavior and current implementation details.
7. For AGENTS.md and CLAUDE.md, update only instructions and workflow rules if the final result truly changes them.
8. Do not add extra files that were not requested.
9. The JSON must be valid.`

type mdFileUpdate struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

type mdFilesResponse struct {
	Files []mdFileUpdate `json:"files"`
}

type chatClient interface {
	chat(ctx context.Context, systemPrompt, userPrompt string) (string, error)
	ask(ctx context.Context, systemPrompt, userPrompt string) (string, error)
	resetAskSession()
}

func promptDir() string {
	if d := os.Getenv("PROMPT_DIR"); d != "" {
		return d
	}
	dir, _ := os.Getwd()
	return dir
}

func ensurePromptDir() (string, error) {
	dir := strings.TrimSpace(promptDir())
	if dir == "" {
		return "", fmt.Errorf("PROMPT_DIR resolved to empty path")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("create prompt dir: %w", err)
	}
	return dir, nil
}

func defaultMDFilesTargets() []string {
	return []string{
		"MEMORY.md",
		"md_files/maintain/INFO.md",
		"CLAUDE.md",
		"AGENTS.md",
	}
}

func parseTargets(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return defaultMDFilesTargets()
	}

	seen := map[string]struct{}{}
	var out []string
	for _, part := range strings.Split(raw, ",") {
		target := filepath.ToSlash(strings.TrimSpace(part))
		if target == "" {
			continue
		}
		if _, ok := seen[target]; ok {
			continue
		}
		seen[target] = struct{}{}
		out = append(out, target)
	}
	if len(out) == 0 {
		return defaultMDFilesTargets()
	}
	slices.Sort(out)
	return out
}

func loadMDFilesForPrompt(targets []string) (string, error) {
	root, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("get working directory: %w", err)
	}

	var sections []string
	for _, target := range targets {
		clean := filepath.Clean(filepath.FromSlash(target))
		if filepath.IsAbs(clean) {
			return "", fmt.Errorf("absolute paths are not allowed: %s", target)
		}
		absPath := filepath.Join(root, clean)
		relPath, err := filepath.Rel(root, absPath)
		if err != nil {
			return "", fmt.Errorf("resolve path %s: %w", target, err)
		}
		relPath = filepath.ToSlash(relPath)
		if strings.HasPrefix(relPath, "../") || relPath == ".." {
			return "", fmt.Errorf("path escapes repository root: %s", target)
		}

		data, err := os.ReadFile(absPath)
		if err != nil {
			return "", fmt.Errorf("read %s: %w", relPath, err)
		}

		sections = append(sections, fmt.Sprintf("FILE: %s\n<<<CONTENT\n%s\nCONTENT>>>", relPath, string(data)))
	}

	return strings.Join(sections, "\n\n"), nil
}

func extractJSONObject(raw string) string {
	trimmed := strings.TrimSpace(raw)
	trimmed = strings.TrimPrefix(trimmed, "```json")
	trimmed = strings.TrimPrefix(trimmed, "```")
	trimmed = strings.TrimSuffix(trimmed, "```")
	return strings.TrimSpace(trimmed)
}

func main() {
	claude, err := newClaudeSignInClient()
	if err != nil {
		slog.Error("chat client init failed", "err", err)
		os.Exit(1)
	}

	s := server.NewMCPServer("claude-bridge", "1.0.0",
		server.WithToolCapabilities(false),
	)

	for _, tool := range buildToolRegistrations(claude) {
		s.AddTool(tool.Tool, tool.Handler)
	}

	slog.Info("claude-bridge MCP server starting (stdio)")
	if err := server.ServeStdio(s); err != nil {
		slog.Error("server error", "err", err)
		os.Exit(1)
	}
}
