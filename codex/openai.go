package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

const defaultModel = "gpt-5-codex"
const defaultBaseURL = "https://api.openai.com/v1"

type openAIClient struct {
	apiKey  string
	baseURL string
	model   string
	http    *http.Client
	askMu   sync.Mutex
	askMsgs []chatMessage
}

func newOpenAIClient() (*openAIClient, error) {
	apiKey := strings.TrimSpace(os.Getenv("OPENAI_API_KEY"))
	if apiKey == "" {
		return nil, fmt.Errorf("OPENAI_API_KEY is not set")
	}

	baseURL := strings.TrimRight(envOr("OPENAI_BASE_URL", defaultBaseURL), "/")
	model := envOr("OPENAI_MODEL", defaultModel)

	return &openAIClient{
		apiKey:  apiKey,
		baseURL: baseURL,
		model:   model,
		http:    &http.Client{Timeout: 120 * time.Second},
	}, nil
}

func envOr(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatRequest struct {
	Model       string        `json:"model"`
	Messages    []chatMessage `json:"messages"`
	Temperature float64       `json:"temperature"`
}

type chatResponse struct {
	Choices []struct {
		Message chatMessage `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

func (c *openAIClient) chat(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	return c.complete(ctx, []chatMessage{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userPrompt},
	})
}

func (c *openAIClient) ask(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	c.askMu.Lock()
	defer c.askMu.Unlock()

	if len(c.askMsgs) == 0 {
		c.askMsgs = append(c.askMsgs, chatMessage{Role: "system", Content: systemPrompt})
	}
	c.askMsgs = append(c.askMsgs, chatMessage{Role: "user", Content: userPrompt})

	answer, err := c.complete(ctx, append([]chatMessage(nil), c.askMsgs...))
	if err != nil {
		c.askMsgs = c.askMsgs[:len(c.askMsgs)-1]
		return "", err
	}

	c.askMsgs = append(c.askMsgs, chatMessage{Role: "assistant", Content: answer})
	return answer, nil
}

func (c *openAIClient) resetAskSession() {
	c.askMu.Lock()
	defer c.askMu.Unlock()
	c.askMsgs = nil
}

func (c *openAIClient) complete(ctx context.Context, messages []chatMessage) (string, error) {
	body := chatRequest{
		Model:       c.model,
		Messages:    messages,
		Temperature: 0.3,
	}
	data, err := json.Marshal(body)
	if err != nil {
		return "", fmt.Errorf("marshal: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/chat/completions", bytes.NewReader(data))
	if err != nil {
		return "", fmt.Errorf("new request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read body: %w", err)
	}

	var result chatResponse
	if err := json.Unmarshal(raw, &result); err != nil {
		return "", fmt.Errorf("unmarshal (status %d): %w\nbody: %s", resp.StatusCode, err, raw)
	}
	if result.Error != nil {
		return "", fmt.Errorf("openai error: %s", result.Error.Message)
	}
	if len(result.Choices) == 0 {
		return "", fmt.Errorf("empty choices (status %d)", resp.StatusCode)
	}
	return result.Choices[0].Message.Content, nil
}
