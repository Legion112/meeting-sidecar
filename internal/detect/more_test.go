package detect_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Legion112/meeting-sidecar/internal/detect"
)

func TestOllamaGateReadBodyAndTruncate(t *testing.T) {
	long := strings.Repeat("e", 300)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(502)
		_, _ = w.Write([]byte(long))
	}))
	defer srv.Close()
	g := &detect.OllamaGate{BaseURL: srv.URL, Model: "m", HTTPClient: srv.Client()}
	_, err := g.IsQuestion(context.Background(), "q?")
	if err == nil || !strings.Contains(err.Error(), "HTTP 502") {
		t.Fatalf("%v", err)
	}

	// successful with is_question string false
	okSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"message": map[string]string{"content": `{"is_question":"no"}`},
		})
	}))
	defer okSrv.Close()
	g2 := &detect.OllamaGate{BaseURL: okSrv.URL, Model: "m", HTTPClient: okSrv.Client()}
	ok, err := g2.IsQuestion(context.Background(), "hello")
	if err != nil || ok {
		t.Fatalf("%v %v", ok, err)
	}
}

func TestParseMore(t *testing.T) {
	if detect.ParseQuestionReply(`{"question":"true"}`) {
		// string "true" via parseYesNo in JSON branch - question key string
	}
	// actually JSON bool string goes through string case -> parseYesNo("true") -> true
	if !detect.ParseQuestionReply(`{"question":"true"}`) {
		t.Fatal("string true")
	}
	if detect.ParseQuestionReply("yes please") != true {
		t.Fatal("prefix yes")
	}
}
