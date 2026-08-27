package detect_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Legion112/meeting-sidecar/internal/detect"
)

func TestParseQuestionReply(t *testing.T) {
	cases := map[string]bool{
		"":                         false,
		"yes":                      true,
		"YES!":                     true,
		"no":                       false,
		`{"question":true}`:        true,
		`{"question":false}`:       false,
		`{"is_question":"yes"}`:    true,
		`{"isQuestion":false}`:     false,
		"```json\n{\"question\":true}\n```": true,
		"maybe":                    false,
		"true":                     true,
		"question":                 true,
		"not a question":           false,
		"statement":                false,
		"y":                        true,
		"n":                        false,
		"false":                    false,
	}
	for in, want := range cases {
		if got := detect.ParseQuestionReply(in); got != want {
			t.Fatalf("%q: got %v want %v", in, got, want)
		}
	}
}

func TestQuestionPrompt(t *testing.T) {
	sys, user := detect.QuestionPrompt("How are you?")
	if user != "How are you?" || sys == "" {
		t.Fatal("prompt")
	}
}

func TestOllamaGate(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/chat" {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"message": map[string]string{"role": "assistant", "content": `{"question":true}`},
		})
	}))
	defer srv.Close()

	g := &detect.OllamaGate{BaseURL: srv.URL, Model: "m", HTTPClient: srv.Client()}
	ok, err := g.IsQuestion(context.Background(), "What time is it?")
	if err != nil || !ok {
		t.Fatalf("%v %v", ok, err)
	}
	ok, err = g.IsQuestion(context.Background(), "  ")
	if err != nil || ok {
		t.Fatalf("empty: %v %v", ok, err)
	}

	g2 := &detect.OllamaGate{BaseURL: "", Model: "m"}
	if _, err := g2.IsQuestion(context.Background(), "q"); err == nil {
		t.Fatal("empty base")
	}
	g3 := &detect.OllamaGate{BaseURL: srv.URL, Model: ""}
	if _, err := g3.IsQuestion(context.Background(), "q"); err == nil {
		t.Fatal("empty model")
	}

	bad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
		_, _ = w.Write([]byte("fail"))
	}))
	defer bad.Close()
	g4 := &detect.OllamaGate{BaseURL: bad.URL + "/", Model: "m", HTTPClient: bad.Client()}
	if _, err := g4.IsQuestion(context.Background(), "q"); err == nil {
		t.Fatal("http error")
	}

	badJSON := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("not-json"))
	}))
	defer badJSON.Close()
	g5 := &detect.OllamaGate{BaseURL: badJSON.URL, Model: "m", HTTPClient: badJSON.Client()}
	if _, err := g5.IsQuestion(context.Background(), "q"); err == nil {
		t.Fatal("decode error")
	}

	// nil client uses default — point at srv
	g6 := &detect.OllamaGate{BaseURL: srv.URL, Model: "m"}
	ok, err = g6.IsQuestion(context.Background(), "q?")
	if err != nil || !ok {
		t.Fatalf("default client: %v %v", ok, err)
	}
}
