package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// OllamaCompleter calls Ollama /api/chat for answers.
type OllamaCompleter struct {
	BaseURL    string
	Model      string
	HTTPClient *http.Client
}

type ollamaReq struct {
	Model    string         `json:"model"`
	Stream   bool           `json:"stream"`
	Messages []ollamaMsg    `json:"messages"`
}

type ollamaMsg struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ollamaResp struct {
	Message ollamaMsg `json:"message"`
}

// Complete implements Completer.
func (c *OllamaCompleter) Complete(ctx context.Context, systemPrompt, question string) (string, error) {
	question = strings.TrimSpace(question)
	if question == "" {
		return "", fmt.Errorf("ollama: empty question")
	}
	base := strings.TrimRight(c.BaseURL, "/")
	if base == "" {
		return "", fmt.Errorf("ollama: base URL is empty")
	}
	model := c.Model
	if model == "" {
		return "", fmt.Errorf("ollama: model is empty")
	}
	client := c.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}

	body, _ := json.Marshal(ollamaReq{
		Model:  model,
		Stream: false,
		Messages: []ollamaMsg{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: question},
		},
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, base+"/api/chat", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("ollama chat: %w", err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("ollama chat: HTTP %d: %s", resp.StatusCode, truncate(string(data), 200))
	}
	var parsed ollamaResp
	if err := json.Unmarshal(data, &parsed); err != nil {
		return "", fmt.Errorf("ollama decode: %w", err)
	}
	return strings.TrimSpace(parsed.Message.Content), nil
}
