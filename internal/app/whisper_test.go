package app_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/Legion112/meeting-sidecar/internal/app"
	"github.com/Legion112/meeting-sidecar/internal/config"
	whisperstt "github.com/Legion112/meeting-sidecar/internal/stt/whisper"
)

func TestNewWhisperNilEnsureExistingFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ggml-small.bin")
	if err := os.WriteFile(path, []byte("weights"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := config.Default()
	cfg.STT.ModelPath = path
	tr, err := app.NewWhisperTranscriber(context.Background(), cfg, nil, func(string, string) (whisperstt.Engine, error) {
		return eng{}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	_ = tr.Close()
}

func TestNewWhisperResolveError(t *testing.T) {
	whisperstt.SetUserHomeDirForTest(func() (string, error) { return "", errors.New("nohome") })
	t.Cleanup(whisperstt.ResetUserHomeDirForTest)
	cfg := config.Default()
	cfg.STT.ModelPath = ""
	_, err := app.NewWhisperTranscriber(context.Background(), cfg,
		func(context.Context, string) error { return nil },
		func(string, string) (whisperstt.Engine, error) { return eng{}, nil },
	)
	if err == nil {
		t.Fatal("expected")
	}
}
