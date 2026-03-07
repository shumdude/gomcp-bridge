package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
)

type openAISignInClient struct {
	bin        string
	model      string
	workdir    string
	sessionDir string
	askStarted bool
	askMu      sync.Mutex
}

func newOpenAISignInClient() (*openAISignInClient, error) {
	bin, err := findCodexBinary()
	if err != nil {
		return nil, err
	}

	workdir, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("get working directory: %w", err)
	}

	client := &openAISignInClient{
		bin:        bin,
		model:      envOr("CODEX_SIGNIN_MODEL", defaultModel),
		workdir:    workdir,
		sessionDir: filepath.Join(workdir, ".codex-bridge", "ask-session"),
	}
	if err := os.MkdirAll(client.sessionDir, 0o755); err != nil {
		return nil, fmt.Errorf("create codex session dir: %w", err)
	}

	if err := client.checkLogin(); err != nil {
		return nil, err
	}

	return client, nil
}

func (c *openAISignInClient) chat(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	return c.runCodex(ctx, c.workdir, false, buildCodexPrompt(systemPrompt, userPrompt))
}

func (c *openAISignInClient) ask(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	c.askMu.Lock()
	defer c.askMu.Unlock()

	answer, err := c.runCodex(ctx, c.sessionDir, c.askStarted, buildCodexPrompt(systemPrompt, userPrompt))
	if err != nil {
		return "", err
	}
	c.askStarted = true
	return answer, nil
}

func (c *openAISignInClient) resetAskSession() {
	c.askMu.Lock()
	defer c.askMu.Unlock()
	c.askStarted = false
}

func (c *openAISignInClient) runCodex(ctx context.Context, cwd string, resume bool, prompt string) (string, error) {
	outputFile, err := os.CreateTemp("", "codex-last-message-*.txt")
	if err != nil {
		return "", fmt.Errorf("create temp output file: %w", err)
	}
	outputPath := outputFile.Name()
	outputFile.Close()
	defer os.Remove(outputPath)

	args := c.buildExecArgs(cwd, resume, outputPath)

	cmd := exec.CommandContext(ctx, c.bin, args...)
	cmd.Stdin = strings.NewReader(prompt)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("codex exec failed: %w: %s", err, strings.TrimSpace(string(output)))
	}

	data, err := os.ReadFile(outputPath)
	if err != nil {
		return "", fmt.Errorf("read codex output: %w", err)
	}

	answer := strings.TrimSpace(string(data))
	if answer == "" {
		return "", fmt.Errorf("codex exec returned empty output")
	}

	return answer, nil
}

func (c *openAISignInClient) buildExecArgs(cwd string, resume bool, outputPath string) []string {
	args := []string{"-C", cwd, "exec", "--color", "never"}
	if resume {
		args = append(args, "resume", "--last", "--skip-git-repo-check", "--output-last-message", outputPath)
	} else {
		args = append(args, "-s", "read-only", "--skip-git-repo-check", "--output-last-message", outputPath)
	}
	if c.model != "" {
		args = append(args, "-m", c.model)
	}
	return append(args, "-")
}

func (c *openAISignInClient) checkLogin() error {
	cmd := exec.Command(c.bin, "login", "status")
	output, err := cmd.CombinedOutput()
	text := strings.TrimSpace(string(output))
	if err != nil {
		if strings.Contains(strings.ToLower(text), "not logged in") {
			return fmt.Errorf("codex CLI is not logged in; run `codex login` or `codex` in a terminal first")
		}
		return fmt.Errorf("codex login status failed: %w: %s", err, text)
	}
	return nil
}

func findCodexBinary() (string, error) {
	if custom := strings.TrimSpace(os.Getenv("CODEX_BIN")); custom != "" {
		return custom, nil
	}

	candidates := []string{"codex.exe", "codex.cmd", "codex"}
	for _, candidate := range candidates {
		if path, err := exec.LookPath(candidate); err == nil {
			return path, nil
		}
	}

	return "", fmt.Errorf("codex CLI not found in PATH; install it or set CODEX_BIN")
}

func buildCodexPrompt(systemPrompt, userPrompt string) string {
	return strings.TrimSpace(
		"You are running as a text-only backend for another MCP server.\n" +
			"Do not edit files, do not run commands, and do not inspect the repository.\n" +
			"Respond only with the final answer to the user prompt.\n\n" +
			"System instructions:\n" + systemPrompt + "\n\n" +
			"User prompt:\n" + userPrompt,
	)
}
