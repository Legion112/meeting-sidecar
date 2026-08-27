package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Legion112/meeting-sidecar/internal/config"
)

func TestApplyDefaultsAllEmpty(t *testing.T) {
	path := filepath.Join(t.TempDir(), "c.yaml")
	yaml := `
audio:
  monitor: ""
  sample_rate: -1
stt:
  provider: ""
  language: ""
detect:
  provider: ""
  ollama:
    base_url: ""
    model: ""
llm:
  provider: ""
  openai:
    model: ""
  ollama:
    base_url: ""
    model: ""
assistant:
  language: ""
  system_prompt: "   "
`
	if err := os.WriteFile(path, []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.STT.Provider != "whisper" || cfg.LLM.Provider != "openai" {
		t.Fatalf("%+v", cfg)
	}
	if cfg.Assistant.SystemPrompt != config.DefaultSystemPrompt {
		t.Fatal("prompt")
	}
}
