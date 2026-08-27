package whisper_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/Legion112/meeting-sidecar/internal/stt/whisper"
)

type fakeEngine struct {
	text string
	err  error
}

func (f fakeEngine) Transcribe(ctx context.Context, samples []float32) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if f.err != nil {
		return "", f.err
	}
	if len(samples) == 0 {
		return "", errors.New("empty")
	}
	return f.text, nil
}

func (f fakeEngine) Close() error { return nil }

func TestClient(t *testing.T) {
	c := &whisper.Client{Engine: fakeEngine{text: "hello"}, Language: "en"}
	pcm := make([]int16, 1600)
	for i := range pcm {
		pcm[i] = 1000
	}
	text, err := c.Transcribe(context.Background(), pcm, 16000)
	if err != nil || text != "hello" {
		t.Fatalf("%q %v", text, err)
	}
	// resample path
	text, err = c.Transcribe(context.Background(), pcm, 8000)
	if err != nil || text != "hello" {
		t.Fatalf("resample %q %v", text, err)
	}
	if _, err := c.Transcribe(context.Background(), nil, 16000); err == nil {
		t.Fatal("empty pcm")
	}
	if _, err := c.Transcribe(context.Background(), pcm, 0); err == nil {
		t.Fatal("rate")
	}
	var nilC *whisper.Client
	if _, err := nilC.Transcribe(context.Background(), pcm, 16000); err == nil {
		t.Fatal("nil client")
	}
	if err := nilC.Close(); err != nil {
		t.Fatal(err)
	}
	if err := c.Close(); err != nil {
		t.Fatal(err)
	}
	bad := &whisper.Client{Engine: fakeEngine{err: errors.New("x")}}
	if _, err := bad.Transcribe(context.Background(), pcm, 16000); err == nil {
		t.Fatal("engine err")
	}
}

func TestResolveModelPath(t *testing.T) {
	p, err := whisper.ResolveModelPath("/tmp/m.bin")
	if err != nil || p != "/tmp/m.bin" {
		t.Fatal(p, err)
	}
	p, err = whisper.ResolveModelPath("")
	if err != nil || p == "" {
		t.Fatal(err)
	}
	_, err = whisper.DefaultModelPath()
	if err != nil {
		t.Fatal(err)
	}
}

func TestDownloader(t *testing.T) {
	dir := t.TempDir()
	dest := filepath.Join(dir, "models", "ggml-small.bin")
	// existing file
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dest, []byte("abc"), 0o644); err != nil {
		t.Fatal(err)
	}
	d := &whisper.Downloader{}
	if err := d.EnsureModel(context.Background(), dest); err != nil {
		t.Fatal(err)
	}
	if err := d.EnsureModel(context.Background(), ""); err == nil {
		t.Fatal("empty path")
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("modelbytes"))
	}))
	defer srv.Close()
	dest2 := filepath.Join(dir, "m2.bin")
	d2 := &whisper.Downloader{Client: srv.Client(), URL: srv.URL}
	if err := d2.EnsureModel(context.Background(), dest2); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(dest2)
	if err != nil || string(b) != "modelbytes" {
		t.Fatalf("%q %v", b, err)
	}

	bad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
	}))
	defer bad.Close()
	d3 := &whisper.Downloader{Client: bad.Client(), URL: bad.URL}
	if err := d3.EnsureModel(context.Background(), filepath.Join(dir, "m3.bin")); err == nil {
		t.Fatal("404")
	}

	d4 := &whisper.Downloader{Client: srv.Client(), URL: ""}
	// empty URL uses default HF URL — don't hit network; set URL to srv
	d4.URL = srv.URL
	if err := d4.EnsureModel(context.Background(), filepath.Join(dir, "m4.bin")); err != nil {
		t.Fatal(err)
	}

	// nil client
	d5 := &whisper.Downloader{URL: srv.URL}
	if err := d5.EnsureModel(context.Background(), filepath.Join(dir, "m5.bin")); err != nil {
		t.Fatal(err)
	}
}

func TestNewNativeEngineStub(t *testing.T) {
	_, err := whisper.NewNativeEngine("/x", "en")
	if err == nil {
		t.Fatal("expected stub error without whisper tag")
	}
}
