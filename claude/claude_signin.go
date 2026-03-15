package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
)

const defaultClaudeModel = "sonnet"

type claudeSignInClient struct {
	bin          string
	model        string
	askSessionID string
	askStarted   bool
	askMu        sync.Mutex
}

type claudePrintResponse struct {
	Type      string `json:"type"`
	Subtype   string `json:"subtype"`
	IsError   bool   `json:"is_error"`
	Result    string `json:"result"`
	SessionID string `json:"session_id"`
}

func newClaudeSignInClient() (*claudeSignInClient, error) {
	bin, err := findClaudeBinary()
	if err != nil {
		return nil, err
	}

	client := &claudeSignInClient{
		bin:   bin,
		model: envOr("CLAUDE_MODEL", defaultClaudeModel),
	}

	if err := client.checkLogin(); err != nil {
		return nil, err
	}

	return client, nil
}

func (c *claudeSignInClient) chat(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	answer, _, err := c.runClaude(ctx, c.buildPrintArgs(""), buildClaudePrompt(systemPrompt, userPrompt))
	if err != nil {
		return "", err
	}
	return answer, nil
}

func (c *claudeSignInClient) ask(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	c.askMu.Lock()
	defer c.askMu.Unlock()

	sessionID := ""
	if c.askStarted {
		sessionID = c.askSessionID
	}

	answer, nextSessionID, err := c.runClaude(ctx, c.buildPrintArgs(sessionID), buildClaudePrompt(systemPrompt, userPrompt))
	if err != nil {
		return "", err
	}

	c.askStarted = true
	if strings.TrimSpace(nextSessionID) != "" {
		c.askSessionID = strings.TrimSpace(nextSessionID)
	}
	return answer, nil
}

func (c *claudeSignInClient) resetAskSession() {
	c.askMu.Lock()
	defer c.askMu.Unlock()
	c.askStarted = false
	c.askSessionID = ""
}

func (c *claudeSignInClient) runClaude(ctx context.Context, args []string, prompt string) (string, string, error) {
	cmd := exec.CommandContext(ctx, c.bin, args...)
	cmd.Stdin = strings.NewReader(prompt)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		out := strings.TrimSpace(stdout.String())
		errOut := strings.TrimSpace(stderr.String())
		return "", "", fmt.Errorf("claude command failed: %w: %s", err, strings.TrimSpace(strings.Join([]string{errOut, out}, "\n")))
	}

	raw := strings.TrimSpace(stdout.String())
	if raw == "" {
		return "", "", fmt.Errorf("claude command returned empty output")
	}

	var resp claudePrintResponse
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		return "", "", fmt.Errorf("parse claude JSON output: %w", err)
	}

	if resp.IsError {
		msg := strings.TrimSpace(resp.Result)
		if msg == "" {
			msg = "unknown Claude error"
		}
		return "", "", fmt.Errorf("%s", msg)
	}

	answer := strings.TrimSpace(resp.Result)
	if answer == "" {
		return "", "", fmt.Errorf("claude returned empty result")
	}

	return answer, strings.TrimSpace(resp.SessionID), nil
}

func (c *claudeSignInClient) buildPrintArgs(resumeSessionID string) []string {
	args := []string{
		"-p",
		"--output-format", "json",
		"--input-format", "text",
	}
	if c.model != "" {
		args = append(args, "--model", c.model)
	}
	if strings.TrimSpace(resumeSessionID) != "" {
		args = append(args, "-r", strings.TrimSpace(resumeSessionID))
	}
	return args
}

func (c *claudeSignInClient) checkLogin() error {
	cmd := exec.Command(c.bin, "auth", "status", "--text")
	output, err := cmd.CombinedOutput()
	text := strings.TrimSpace(string(output))
	lower := strings.ToLower(text)

	if strings.Contains(lower, "not logged in") || strings.Contains(lower, "not authenticated") || strings.Contains(lower, "login required") {
		return fmt.Errorf("claude CLI is not logged in; run `claude auth login` first")
	}
	if err != nil {
		return fmt.Errorf("claude auth status failed: %w: %s", err, text)
	}
	return nil
}

func findClaudeBinary() (string, error) {
	if custom := strings.TrimSpace(os.Getenv("CLAUDE_BIN")); custom != "" {
		return custom, nil
	}

	candidates := []string{"claude.exe", "claude.cmd", "claude"}
	for _, candidate := range candidates {
		if path, err := exec.LookPath(candidate); err == nil {
			return path, nil
		}
	}

	return "", fmt.Errorf("claude CLI not found in PATH; install it or set CLAUDE_BIN")
}

func buildClaudePrompt(systemPrompt, userPrompt string) string {
	return strings.TrimSpace(
		"You are running as a text-only backend for another MCP server.\n" +
			"Do not edit files, do not run commands, and do not inspect the repository.\n" +
			"Respond only with the final answer to the user prompt.\n\n" +
			"System instructions:\n" + systemPrompt + "\n\n" +
			"User prompt:\n" + userPrompt,
	)
}

func envOr(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}
