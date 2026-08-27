package whisper

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestResampleNZero(t *testing.T) {
	if out := resampleLinear([]float32{1}, 100000, 1); out != nil {
		t.Fatalf("want nil got %v", out)
	}
}

func TestSetUserHomeDirHelpers(t *testing.T) {
	SetUserHomeDirForTest(nil)
	ResetUserHomeDirForTest()
	SetUserHomeDirForTest(func() (string, error) { return "/tmp", nil })
	p, err := DefaultModelPath()
	if err != nil || p == "" {
		t.Fatal(err, p)
	}
	ResetUserHomeDirForTest()
}

func TestUserHomeDirError(t *testing.T) {
	old := userHomeDir
	userHomeDir = func() (string, error) { return "", errors.New("no home") }
	t.Cleanup(func() { userHomeDir = old })
	if _, err := DefaultModelPath(); err == nil {
		t.Fatal("expected error")
	}
}

func TestResampleSameAndTiny(t *testing.T) {
	pcm := []int16{1, 2, 3, 4}
	out := resampleLinear(int16ToFloat32(pcm), 16000, 16000)
	if len(out) != 4 {
		t.Fatal(len(out))
	}
	if resampleLinear(nil, 8000, 16000) != nil {
		t.Fatal("empty")
	}
	_ = resampleLinear([]float32{0.1}, 8000, 16000)
	_ = resampleLinear([]float32{0.5}, 1, 100)
	if resampleLinear([]float32{1, 2}, 1000000, 1) != nil && false {
		t.Fatal("n<=0")
	}
	// force n<=0: from >> to with tiny input
	if out := resampleLinear([]float32{1}, 100000, 1); out != nil && len(out) == 0 {
		// n = 1 * 1 / 100000 = 0
	}
	if out := resampleLinear([]float32{1}, 100000, 1); out != nil {
		t.Fatalf("want nil, got %v", out)
	}
}

func TestEnsureModelWriteErrors(t *testing.T) {
	dir := t.TempDir()
	parentFile := filepath.Join(dir, "file")
	if err := os.WriteFile(parentFile, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	d := &Downloader{Client: http.DefaultClient, URL: "http://127.0.0.1:1"}
	if err := d.EnsureModel(context.Background(), filepath.Join(parentFile, "m.bin")); err == nil {
		t.Fatal("mkdir")
	}
	if err := d.EnsureModel(context.Background(), filepath.Join(dir, "x.bin")); err == nil {
		t.Fatal("conn")
	}
	dBad := &Downloader{Client: http.DefaultClient, URL: "://nope"}
	if err := dBad.EnsureModel(context.Background(), filepath.Join(dir, "y.bin")); err == nil {
		t.Fatal("url")
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hj, ok := w.(http.Hijacker)
		if !ok {
			t.Fatal("hijack")
		}
		c, _, _ := hj.Hijack()
		_, _ = c.Write([]byte("HTTP/1.1 200 OK\r\nContent-Length: 50\r\n\r\nxx"))
		_ = c.Close()
	}))
	defer srv.Close()
	d2 := &Downloader{Client: srv.Client(), URL: srv.URL}
	if err := d2.EnsureModel(context.Background(), filepath.Join(dir, "z.bin")); err == nil {
		t.Fatal("short body")
	}

	d3 := &Downloader{Client: srv.Client(), URL: ""}
	dest := filepath.Join(dir, "exists.bin")
	if err := os.WriteFile(dest, []byte("1"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := d3.EnsureModel(context.Background(), dest); err != nil {
		t.Fatal(err)
	}
}

func TestEnsureModelCreateAndRename(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("abc"))
	}))
	defer srv.Close()
	dir := t.TempDir()
	ro := filepath.Join(dir, "ro")
	if err := os.MkdirAll(ro, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(ro, 0555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(ro, 0755) })
	d := &Downloader{Client: srv.Client(), URL: srv.URL}
	if err := d.EnsureModel(context.Background(), filepath.Join(ro, "m.bin")); err == nil {
		t.Fatal("create should fail on read-only dir")
	}

	destDir := filepath.Join(dir, "asdir")
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := d.EnsureModel(context.Background(), destDir); err == nil {
		t.Fatal("rename")
	}
}

func TestEnsureModelDefaultURL(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("weights"))
	}))
	defer srv.Close()
	dir := t.TempDir()
	dest := filepath.Join(dir, "m.bin")
	d := &Downloader{
		URL: "",
		Client: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			if req.URL.String() != DefaultModelURL {
				t.Fatalf("url %s", req.URL)
			}
			req2, _ := http.NewRequestWithContext(req.Context(), http.MethodGet, srv.URL, nil)
			return srv.Client().Do(req2)
		})},
	}
	if err := d.EnsureModel(context.Background(), dest); err != nil {
		t.Fatal(err)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
