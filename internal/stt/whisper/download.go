package whisper

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
)

// DefaultModelURL is the official ggml-small.bin on Hugging Face.
const DefaultModelURL = "https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-small.bin"

// Downloader fetches model weights over HTTP.
type Downloader struct {
	Client  *http.Client
	URL     string
	DestDir string
}

// EnsureModel downloads the ggml model if dest does not exist.
func (d *Downloader) EnsureModel(ctx context.Context, destPath string) error {
	if destPath == "" {
		return fmt.Errorf("model path is empty")
	}
	if st, err := os.Stat(destPath); err == nil && !st.IsDir() && st.Size() > 0 {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
		return fmt.Errorf("mkdir models: %w", err)
	}

	url := d.URL
	if url == "" {
		url = DefaultModelURL
	}
	client := d.Client
	if client == nil {
		client = http.DefaultClient
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("model request: %w", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("download model: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download model: HTTP %d", resp.StatusCode)
	}

	tmp := destPath + ".partial"
	f, err := os.Create(tmp)
	if err != nil {
		return fmt.Errorf("create model file: %w", err)
	}
	if _, err := io.Copy(f, resp.Body); err != nil {
		_ = f.Close()
		_ = os.Remove(tmp)
		return fmt.Errorf("write model: %w", err)
	}
	_ = f.Close()
	if err := os.Rename(tmp, destPath); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("finalize model: %w", err)
	}
	return nil
}
