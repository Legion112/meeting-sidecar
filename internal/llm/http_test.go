package llm_test

import (
	"context"
	"io"
	"net/http"
	"testing"

	"github.com/Legion112/meeting-sidecar/internal/llm"
)

type errBody struct{}

func (errBody) Read([]byte) (int, error) { return 0, io.ErrUnexpectedEOF }
func (errBody) Close() error             { return nil }

type rtFunc func(*http.Request) (*http.Response, error)

func (f rtFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestOpenAIDefaultBaseAndReadErr(t *testing.T) {
	c := &llm.OpenAICompleter{
		APIKey:  "k",
		BaseURL: "",
		HTTPClient: &http.Client{Transport: rtFunc(func(req *http.Request) (*http.Response, error) {
			if req.URL.Host != "api.openai.com" {
				t.Fatalf("host %s", req.URL.Host)
			}
			return &http.Response{StatusCode: 200, Body: errBody{}, Header: make(http.Header), Request: req}, nil
		})},
	}
	if _, err := c.Complete(context.Background(), "s", "q"); err == nil {
		t.Fatal("read")
	}

	c2 := &llm.OpenAICompleter{APIKey: "k", BaseURL: "http://[", HTTPClient: http.DefaultClient}
	if _, err := c2.Complete(context.Background(), "s", "q"); err == nil {
		t.Fatal("bad url")
	}

	c3 := &llm.OllamaCompleter{BaseURL: "http://[", Model: "m"}
	if _, err := c3.Complete(context.Background(), "s", "q"); err == nil {
		t.Fatal("ollama bad url")
	}
	c4 := &llm.OllamaCompleter{
		BaseURL: "http://x", Model: "m",
		HTTPClient: &http.Client{Transport: rtFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: 200, Body: errBody{}, Header: make(http.Header), Request: req}, nil
		})},
	}
	if _, err := c4.Complete(context.Background(), "s", "q"); err == nil {
		t.Fatal("ollama read")
	}
}
