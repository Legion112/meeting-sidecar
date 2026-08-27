package llm_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Legion112/meeting-sidecar/internal/llm"
)

func TestTruncatePaths(t *testing.T) {
	long := strings.Repeat("z", 250)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
		_, _ = w.Write([]byte(long))
	}))
	defer srv.Close()
	c := &llm.OpenAICompleter{APIKey: "k", BaseURL: srv.URL, HTTPClient: srv.Client()}
	_, err := c.Complete(context.Background(), "s", "q")
	if err == nil || !strings.Contains(err.Error(), "HTTP 500") {
		t.Fatal(err)
	}
	c2 := &llm.OllamaCompleter{BaseURL: srv.URL, Model: "m", HTTPClient: srv.Client()}
	_, err = c2.Complete(context.Background(), "s", "q")
	if err == nil {
		t.Fatal("expected")
	}

	// Do() network error
	closed := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	u := closed.URL
	closed.Close()
	c3 := &llm.OpenAICompleter{APIKey: "k", BaseURL: u, HTTPClient: http.DefaultClient}
	if _, err := c3.Complete(context.Background(), "s", "q"); err == nil {
		t.Fatal("do err")
	}
	c4 := &llm.OllamaCompleter{BaseURL: u, Model: "m", HTTPClient: http.DefaultClient}
	if _, err := c4.Complete(context.Background(), "s", "q"); err == nil {
		t.Fatal("ollama do")
	}
}
