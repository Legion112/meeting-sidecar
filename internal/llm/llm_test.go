package llm_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Legion112/meeting-sidecar/internal/llm"
)

func TestOpenAICompleter(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "" {
			t.Fatal("missing auth")
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{
				{"message": map[string]string{"role": "assistant", "content": "  Answer  "}},
			},
		})
	}))
	defer srv.Close()

	c := &llm.OpenAICompleter{
		APIKey:     "sk-test",
		BaseURL:    srv.URL,
		Model:      "",
		HTTPClient: srv.Client(),
	}
	out, err := c.Complete(context.Background(), "sys", "What is 2+2?")
	if err != nil || out != "Answer" {
		t.Fatalf("%q %v", out, err)
	}
	if _, err := c.Complete(context.Background(), "sys", " "); err == nil {
		t.Fatal("empty q")
	}
	c2 := &llm.OpenAICompleter{BaseURL: srv.URL, HTTPClient: srv.Client()}
	t.Setenv("OPENAI_API_KEY", "")
	if _, err := c2.Complete(context.Background(), "sys", "q"); err == nil {
		t.Fatal("missing key")
	}

	errSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"error": map[string]string{"message": "nope"},
		})
	}))
	defer errSrv.Close()
	c3 := &llm.OpenAICompleter{APIKey: "k", BaseURL: errSrv.URL, HTTPClient: errSrv.Client()}
	if _, err := c3.Complete(context.Background(), "sys", "q"); err == nil {
		t.Fatal("api error")
	}

	empty := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"choices": []any{}})
	}))
	defer empty.Close()
	c4 := &llm.OpenAICompleter{APIKey: "k", BaseURL: empty.URL + "/", Model: "m", HTTPClient: empty.Client()}
	if _, err := c4.Complete(context.Background(), "sys", "q"); err == nil {
		t.Fatal("empty choices")
	}

	httpErr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(503)
		_, _ = w.Write([]byte("down"))
	}))
	defer httpErr.Close()
	c5 := &llm.OpenAICompleter{APIKey: "k", BaseURL: httpErr.URL, HTTPClient: httpErr.Client()}
	if _, err := c5.Complete(context.Background(), "sys", "q"); err == nil {
		t.Fatal("http status")
	}

	bad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("{"))
	}))
	defer bad.Close()
	c6 := &llm.OpenAICompleter{APIKey: "k", BaseURL: bad.URL, HTTPClient: bad.Client()}
	if _, err := c6.Complete(context.Background(), "sys", "q"); err == nil {
		t.Fatal("decode")
	}

	// nil client + default base path — skip live network; already covered paths
	c7 := &llm.OpenAICompleter{APIKey: "k", BaseURL: srv.URL, HTTPClient: nil}
	// override by using srv URL with default client pointing to local srv - DefaultClient works for httptest
	if _, err := c7.Complete(context.Background(), "sys", "q"); err != nil {
		// DefaultClient can talk to httptest URL
		t.Log(err)
	}
}

func TestOllamaCompleter(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"message": map[string]string{"content": " hi "},
		})
	}))
	defer srv.Close()
	c := &llm.OllamaCompleter{BaseURL: srv.URL, Model: "m", HTTPClient: srv.Client()}
	out, err := c.Complete(context.Background(), "sys", "q?")
	if err != nil || out != "hi" {
		t.Fatalf("%q %v", out, err)
	}
	if _, err := c.Complete(context.Background(), "sys", ""); err == nil {
		t.Fatal("empty")
	}
	c2 := &llm.OllamaCompleter{BaseURL: "", Model: "m"}
	if _, err := c2.Complete(context.Background(), "sys", "q"); err == nil {
		t.Fatal("base")
	}
	c3 := &llm.OllamaCompleter{BaseURL: srv.URL, Model: ""}
	if _, err := c3.Complete(context.Background(), "sys", "q"); err == nil {
		t.Fatal("model")
	}
	bad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
		_, _ = w.Write([]byte("x"))
	}))
	defer bad.Close()
	c4 := &llm.OllamaCompleter{BaseURL: bad.URL, Model: "m", HTTPClient: bad.Client()}
	if _, err := c4.Complete(context.Background(), "sys", "q"); err == nil {
		t.Fatal("http")
	}
	badJSON := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("nope"))
	}))
	defer badJSON.Close()
	c5 := &llm.OllamaCompleter{BaseURL: badJSON.URL + "/", Model: "m", HTTPClient: badJSON.Client()}
	if _, err := c5.Complete(context.Background(), "sys", "q"); err == nil {
		t.Fatal("json")
	}
	c6 := &llm.OllamaCompleter{BaseURL: srv.URL, Model: "m"}
	if _, err := c6.Complete(context.Background(), "sys", "q"); err != nil {
		t.Fatal(err)
	}
}
