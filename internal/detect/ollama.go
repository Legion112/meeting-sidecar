package detect

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// OllamaGate uses a small Ollama chat model as the question gate.
type OllamaGate struct {
	BaseURL    string
	Model      string
	HTTPClient *http.Client
}

type ollamaChatRequest struct {
	Model    string          `json:"model"`
	Stream   bool            `json:"stream"`
	Messages []ollamaMessage `json:"messages"`
	Format   string          `json:"format,omitempty"`
}

type ollamaMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ollamaChatResponse struct {
	Message ollamaMessage `json:"message"`
}

// IsQuestion implements Gate.
func (g *OllamaGate) IsQuestion(ctx context.Context, text string) (bool, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return false, nil
	}
	base := strings.TrimRight(g.BaseURL, "/")
	if base == "" {
		return false, fmt.Errorf("ollama base URL is empty")
	}
	model := g.Model
	if model == "" {
		return false, fmt.Errorf("ollama model is empty")
	}
	client := g.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}

	sys, user := QuestionPrompt(text)
	body, _ := json.Marshal(ollamaChatRequest{
		Model:  model,
		Stream: false,
		Format: "json",
		Messages: []ollamaMessage{
			{Role: "system", Content: sys},
			{Role: "user", Content: user},
		},
	})

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, base+"/api/chat", bytes.NewReader(body))
	if err != nil {
		return false, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return false, fmt.Errorf("ollama chat: %w", err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return false, err
	}
	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("ollama chat: HTTP %d: %s", resp.StatusCode, truncate(string(data), 200))
	}
	var parsed ollamaChatResponse
	if err := json.Unmarshal(data, &parsed); err != nil {
		return false, fmt.Errorf("ollama chat decode: %w", err)
	}
	return ParseQuestionReply(parsed.Message.Content), nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
