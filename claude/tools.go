package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

type toolRegistration struct {
	Tool    mcp.Tool
	Handler server.ToolHandlerFunc
}

const systemPromptWriteCode = `You are a senior software engineer.
Generate implementation code based on the provided prompt.

Rules:
1. Produce concrete, working code.
2. Follow the user's requested language/framework if provided.
3. If no language is specified, pick the most reasonable one for the task.
4. Keep output focused on the code solution.
5. Return only the final code output.`

func buildToolRegistrations(claude chatClient) []toolRegistration {
	var askMu sync.Mutex

	toolGeneratePromptMD := mcp.NewTool("generate_prompt_md",
		mcp.WithDescription(
			"Sends a task description to Claude, which generates a detailed "+
				"PROMPT.md file for Codex CLI. The file is saved in the working directory. "+
				"Returns the PROMPT.md content.",
		),
		mcp.WithString("task",
			mcp.Required(),
			mcp.Description("Free-form task description from the user"),
		),
	)

	handlerGeneratePromptMD := func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		task, err := req.RequireString("task")
		if err != nil || strings.TrimSpace(task) == "" {
			return mcp.NewToolResultError("parameter 'task' is required"), nil
		}

		slog.Info("generate_prompt_md called", "task_len", len(task))

		content, err := claude.chat(ctx, systemPromptMD, task)
		if err != nil {
			return mcp.NewToolResultError("Claude error: " + err.Error()), nil
		}

		dir, err := ensurePromptDir()
		if err != nil {
			return mcp.NewToolResultError("failed to prepare directory for PROMPT.md: " + err.Error()), nil
		}

		path := filepath.Join(dir, "PROMPT.md")
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			return mcp.NewToolResultError("failed to write PROMPT.md: " + err.Error()), nil
		}

		askMu.Lock()
		claude.resetAskSession()
		askMu.Unlock()

		slog.Info("PROMPT.md written", "path", path)
		return mcp.NewToolResultText(
			fmt.Sprintf("PROMPT.md saved: %s\n\n---\n\n%s", path, content),
		), nil
	}

	toolAskClaude := mcp.NewTool("ask_claude",
		mcp.WithDescription(
			"Asks one clarifying question to Claude in the context of the current task. "+
				"Use this tool strictly one question per call, without parallel calls. "+
				"All ask_claude calls after the latest generate_prompt_md must stay in one shared session. "+
				"Returns Claude's answer.",
		),
		mcp.WithString("question",
			mcp.Required(),
			mcp.Description("Specific clarifying question about the task"),
		),
		mcp.WithString("context",
			mcp.Description("Short context: PROMPT.md content or previously received answers (optional)"),
		),
	)

	handlerAskClaude := func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		question, err := req.RequireString("question")
		if err != nil || strings.TrimSpace(question) == "" {
			return mcp.NewToolResultError("parameter 'question' is required"), nil
		}
		extraCtx := req.GetString("context", "")

		userMsg := question
		if strings.TrimSpace(extraCtx) != "" {
			userMsg = "Task context:\n" + extraCtx + "\n\nQuestion:\n" + question
		}

		slog.Info("ask_claude called", "question", question)
		askMu.Lock()
		defer askMu.Unlock()

		answer, err := claude.ask(ctx, systemPromptQA, userMsg)
		if err != nil {
			return mcp.NewToolResultError("Claude error: " + err.Error()), nil
		}

		return mcp.NewToolResultText(answer), nil
	}

	toolWriteCode := mcp.NewTool("write_code",
		mcp.WithDescription(
			"Generates code from a required prompt using Claude.",
		),
		mcp.WithString("prompt",
			mcp.Required(),
			mcp.Description("Required prompt describing what code to generate"),
		),
	)

	handlerWriteCode := func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		prompt, err := req.RequireString("prompt")
		if err != nil || strings.TrimSpace(prompt) == "" {
			return mcp.NewToolResultError("parameter 'prompt' is required"), nil
		}

		slog.Info("write_code called", "prompt_len", len(prompt))

		code, err := claude.chat(ctx, systemPromptWriteCode, prompt)
		if err != nil {
			return mcp.NewToolResultError("Claude error: " + err.Error()), nil
		}

		return mcp.NewToolResultText(code), nil
	}

	toolFillMDFiles := mcp.NewTool("fill_md_files",
		mcp.WithDescription(
			"Updates selected project markdown files based on Codex CLI final implementation result. "+
				"Use after successful tests and before task completion.",
		),
		mcp.WithString("final_result",
			mcp.Required(),
			mcp.Description("Final Codex CLI result: what was implemented, validated, and how it ended"),
		),
		mcp.WithString("targets",
			mcp.Description("Optional comma-separated list of markdown files. Default: MEMORY.md, md_files/maintain/INFO.md, CLAUDE.md, AGENTS.md"),
		),
	)

	handlerFillMDFiles := func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		finalResult, err := req.RequireString("final_result")
		if err != nil || strings.TrimSpace(finalResult) == "" {
			return mcp.NewToolResultError("parameter 'final_result' is required"), nil
		}

		targets := parseTargets(req.GetString("targets", ""))
		filesContext, err := loadMDFilesForPrompt(targets)
		if err != nil {
			return mcp.NewToolResultError("failed to prepare markdown files: " + err.Error()), nil
		}

		userMsg := "Final implementation result:\n" + finalResult + "\n\n" +
			"Update the following markdown files:\n" + strings.Join(targets, "\n") + "\n\n" +
			"Current file contents:\n\n" + filesContext

		raw, err := claude.chat(ctx, systemPromptFillMDFiles, userMsg)
		if err != nil {
			return mcp.NewToolResultError("Claude error: " + err.Error()), nil
		}

		var resp mdFilesResponse
		if err := json.Unmarshal([]byte(extractJSONObject(raw)), &resp); err != nil {
			return mcp.NewToolResultError("failed to parse fill_md_files response: " + err.Error()), nil
		}
		if len(resp.Files) == 0 {
			return mcp.NewToolResultError("fill_md_files returned no files"), nil
		}

		root, err := os.Getwd()
		if err != nil {
			return mcp.NewToolResultError("failed to resolve working directory: " + err.Error()), nil
		}

		allowed := map[string]struct{}{}
		for _, target := range targets {
			allowed[filepath.ToSlash(filepath.Clean(filepath.FromSlash(target)))] = struct{}{}
		}

		var updated []string
		for _, file := range resp.Files {
			rel := filepath.ToSlash(filepath.Clean(filepath.FromSlash(strings.TrimSpace(file.Path))))
			if _, ok := allowed[rel]; !ok {
				return mcp.NewToolResultError("fill_md_files returned unexpected path: " + rel), nil
			}
			if strings.TrimSpace(file.Content) == "" {
				return mcp.NewToolResultError("fill_md_files returned empty content for: " + rel), nil
			}

			abs := filepath.Join(root, filepath.FromSlash(rel))
			if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
				return mcp.NewToolResultError("failed to prepare directory for " + rel + ": " + err.Error()), nil
			}
			if err := os.WriteFile(abs, []byte(file.Content), 0o644); err != nil {
				return mcp.NewToolResultError("failed to write " + rel + ": " + err.Error()), nil
			}
			updated = append(updated, rel)
		}

		slices.Sort(updated)
		return mcp.NewToolResultText("Updated markdown files:\n- " + strings.Join(updated, "\n- ")), nil
	}

	return []toolRegistration{
		{
			Tool:    toolGeneratePromptMD,
			Handler: handlerGeneratePromptMD,
		},
		{
			Tool:    toolAskClaude,
			Handler: handlerAskClaude,
		},
		{
			Tool:    toolWriteCode,
			Handler: handlerWriteCode,
		},
		{
			Tool:    toolFillMDFiles,
			Handler: handlerFillMDFiles,
		},
	}
}
