package config_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Legion112/meeting-sidecar/internal/config"
)

func TestDefault(t *testing.T) {
	cfg := config.Default()
	if cfg.Audio.SampleRate != 16000 {
		t.Fatalf("sample rate: %d", cfg.Audio.SampleRate)
	}
	if cfg.LLM.OpenAI.Model != "gpt-5.6-luna" {
		t.Fatalf("model: %s", cfg.LLM.OpenAI.Model)
	}
	if !strings.Contains(cfg.Assistant.SystemPrompt, "meeting sidecar") {
		t.Fatal("default system prompt missing")
	}
}

func TestLoadAndValidate(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "c.yaml")
	content := `
audio:
  monitor: "alsa_output.pci-0000_34_00.4.iec958-stereo.monitor"
  sample_rate: 0
stt:
  provider: whisper
detect:
  provider: ollama
  ollama:
    base_url: ""
    model: ""
llm:
  provider: openai
  openai:
    model: ""
  ollama:
    base_url: ""
    model: ""
ui:
  display: 0
assistant:
  language: ""
  system_prompt: ""
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Audio.SampleRate != 16000 {
		t.Fatalf("expected default sample rate, got %d", cfg.Audio.SampleRate)
	}
	if cfg.Assistant.SystemPrompt != config.DefaultSystemPrompt {
		t.Fatal("expected baked-in system prompt")
	}
	if cfg.Detect.Ollama.Model != "qwen2.5:0.5b" {
		t.Fatalf("detect model: %s", cfg.Detect.Ollama.Model)
	}
}

func TestLoadRejectsMic(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.yaml")
	if err := os.WriteFile(path, []byte("audio:\n  monitor: alsa_input.usb-mic\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := config.Load(path)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestLoadMissing(t *testing.T) {
	_, err := config.Load(filepath.Join(t.TempDir(), "nope.yaml"))
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestLoadInvalidYAML(t *testing.T) {
	path := filepath.Join(t.TempDir(), "x.yaml")
	if err := os.WriteFile(path, []byte(":\n:\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := config.Load(path)
	if err == nil {
		t.Fatal("expected parse error")
	}
}

func TestLoadMonitorAll(t *testing.T) {
	path := filepath.Join(t.TempDir(), "all.yaml")
	if err := os.WriteFile(path, []byte("audio:\n  monitor: all\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Audio.Monitor != "all" {
		t.Fatal(cfg.Audio.Monitor)
	}
}

func TestValidateProviders(t *testing.T) {
	cfg := config.Default()
	cfg.STT.Provider = "openai"
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected stt error")
	}
	cfg = config.Default()
	cfg.Detect.Provider = "x"
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected detect error")
	}
	cfg = config.Default()
	cfg.LLM.Provider = "x"
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected llm error")
	}
	cfg = config.Default()
	cfg.UI.Display = -1
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected display error")
	}
}

func TestIsMonitorName(t *testing.T) {
	if !config.IsMonitorName("foo.monitor") {
		t.Fatal("expected true")
	}
	if config.IsMonitorName("mic") {
		t.Fatal("expected false")
	}
}
