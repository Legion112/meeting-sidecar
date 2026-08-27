package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Legion112/meeting-sidecar/internal/config"
)

func TestResolveSTTModelPath(t *testing.T) {
	dir := t.TempDir()
	home := filepath.Join(dir, "home")
	models := filepath.Join(home, ".local", "share", "meeting-sidecar", "models")
	trans := filepath.Join(home, "github", "transcription", "models")
	if err := os.MkdirAll(models, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(trans, 0o755); err != nil {
		t.Fatal(err)
	}
	turbo := filepath.Join(trans, "ggml-large-v3-turbo.bin")
	if err := os.WriteFile(turbo, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	config.SetUserHomeDirForTest(func() (string, error) { return home, nil })
	t.Cleanup(config.ResetUserHomeDirForTest)

	cfg := config.Default()
	cfg.STT.Model = "large-v3-turbo"
	got, err := cfg.ResolveSTTModelPath()
	if err != nil || got != turbo {
		t.Fatalf("transcription fallback: %q %v", got, err)
	}

	cfg = config.Default()
	cfg.STT.ModelPath = turbo
	got, err = cfg.ResolveSTTModelPath()
	if err != nil || got != turbo {
		t.Fatalf("model_path: %q %v", got, err)
	}

	cfg = config.Default()
	cfg.STT.Model = "large-v3-turbo"
	got, err = cfg.ResolveSTTModelPath()
	if err != nil || got != turbo {
		t.Fatalf("model shorthand: %q %v", got, err)
	}

	cfg = config.Default()
	got, err = cfg.ResolveSTTModelPath()
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(models, "ggml-small.bin")
	if got != want {
		t.Fatalf("default: got %q want %q", got, want)
	}
}

func TestSTTLanguage(t *testing.T) {
	cfg := config.Default()
	cfg.STT.Language = "auto"
	cfg.Assistant.Language = "ru"
	if got := cfg.STTLanguage(); got != "ru" {
		t.Fatalf("assistant fallback: %q", got)
	}

	cfg.STT.Language = "en"
	if got := cfg.STTLanguage(); got != "en" {
		t.Fatalf("explicit stt: %q", got)
	}

	cfg.STT.Language = "auto"
	cfg.Assistant.Language = "auto"
	if got := cfg.STTLanguage(); got != "auto" {
		t.Fatalf("both auto: %q", got)
	}
}

func TestSegmenterConfig(t *testing.T) {
	cfg := config.Default()
	base := cfg.Audio.SegmenterConfig()
	if base.HangoverFrames != 15 || base.MaxSpeechFrames != cfg.Audio.SampleRate/20*15 {
		t.Fatalf("defaults changed: %+v", base)
	}

	cfg.Audio.VAD = config.VADConfig{
		EnergyThreshold: 300,
		HangoverMs:      500,
		MinSpeechMs:     250,
		MaxSpeechSec:    30,
	}
	vad := cfg.Audio.SegmenterConfig()
	if vad.EnergyThreshold != 300 {
		t.Fatalf("threshold: %v", vad.EnergyThreshold)
	}
	if vad.HangoverFrames != 25 {
		t.Fatalf("hangover frames: %d", vad.HangoverFrames)
	}
	if vad.MinSpeechFrames != 13 {
		t.Fatalf("min speech frames: %d", vad.MinSpeechFrames)
	}
	if vad.MaxSpeechFrames != 16000*30/20 {
		t.Fatalf("max speech frames: %d", vad.MaxSpeechFrames)
	}
}
