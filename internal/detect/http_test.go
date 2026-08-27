package detect_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Legion112/meeting-sidecar/internal/detect"
)

type errBody struct{}

func (errBody) Read([]byte) (int, error) { return 0, io.ErrUnexpectedEOF }
func (errBody) Close() error             { return nil }

func TestOllamaGateDoAndReadErrors(t *testing.T) {
	closed := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	u := closed.URL
	closed.Close()
	g := &detect.OllamaGate{BaseURL: u, Model: "m", HTTPClient: http.DefaultClient}
	if _, err := g.IsQuestion(context.Background(), "q?"); err == nil {
		t.Fatal("do")
	}

	// NewRequest fails on invalid URL
	g2 := &detect.OllamaGate{BaseURL: "http://[", Model: "m"}
	if _, err := g2.IsQuestion(context.Background(), "q?"); err == nil {
		t.Fatal("bad url")
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		// hijack to return body that errors — use Response with err Body via custom client
	}))
	defer srv.Close()

	g3 := &detect.OllamaGate{
		BaseURL: "http://example.com",
		Model:   "m",
		HTTPClient: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: 200,
				Body:       errBody{},
				Header:     make(http.Header),
				Request:    req,
			}, nil
		})},
	}
	if _, err := g3.IsQuestion(context.Background(), "q?"); err == nil {
		t.Fatal("read body")
	}
	_ = strings.Contains
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
