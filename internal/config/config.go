package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/Legion112/meeting-sidecar/internal/vad"
	"gopkg.in/yaml.v3"
)

// DefaultSystemPrompt is the baked-in answer prompt when assistant.system_prompt is empty.
const DefaultSystemPrompt = `You are a private meeting sidecar. The user will speak your output; they will not read a report.

Answer the meeting question only.
Be short: at most 3 sentences, or up to 5 tight bullets if the question lists items.
Lead with the answer. No preamble, no restating the question, no "as an AI".
Be helpful and specific. If you are unsure, say so in one clause and give the best cautious answer.
Match the question language (English or Russian).
Do not invent numbers, names, or commitments. Do not ask follow-up questions unless a yes/no would be unsafe.`

// Config is the on-disk YAML configuration.
type Config struct {
	Audio     AudioConfig     `yaml:"audio"`
	STT       STTConfig       `yaml:"stt"`
	Detect    DetectConfig    `yaml:"detect"`
	LLM       LLMConfig       `yaml:"llm"`
	UI        UIConfig        `yaml:"ui"`
	Assistant AssistantConfig `yaml:"assistant"`
}

type AudioConfig struct {
	Monitor          string    `yaml:"monitor"`
	Microphone       bool      `yaml:"microphone"`
	MicrophoneSource string    `yaml:"microphone_source"`
	SampleRate       int       `yaml:"sample_rate"`
	VAD              VADConfig `yaml:"vad"`
}

// VADConfig overrides energy VAD defaults; zero values keep built-in defaults.
type VADConfig struct {
	EnergyThreshold float64 `yaml:"energy_threshold"`
	HangoverMs      int     `yaml:"hangover_ms"`
	MinSpeechMs     int     `yaml:"min_speech_ms"`
	MaxSpeechSec    int     `yaml:"max_speech_sec"`
}

type STTConfig struct {
	Provider      string `yaml:"provider"`
	Model         string `yaml:"model"`
	ModelPath     string `yaml:"model_path"`
	Language      string `yaml:"language"`
	InitialPrompt string `yaml:"initial_prompt"`
	Threads       int    `yaml:"threads"`
}

type DetectConfig struct {
	Provider string       `yaml:"provider"`
	Ollama   OllamaConfig `yaml:"ollama"`
}

type LLMConfig struct {
	Provider string       `yaml:"provider"`
	OpenAI   OpenAIConfig `yaml:"openai"`
	Ollama   OllamaConfig `yaml:"ollama"`
}

type OpenAIConfig struct {
	Model string `yaml:"model"`
}

type OllamaConfig struct {
	BaseURL string `yaml:"base_url"`
	Model   string `yaml:"model"`
}

type UIConfig struct {
	Display      int  `yaml:"display"`
	AlwaysOnTop  bool `yaml:"always_on_top"`
}

type AssistantConfig struct {
	Language     string `yaml:"language"`
	SystemPrompt string `yaml:"system_prompt"`
}

// Default returns a Config with v1 defaults.
func Default() Config {
	return Config{
		Audio: AudioConfig{
			Monitor:    "",
			SampleRate: 16000,
		},
		STT: STTConfig{
			Provider:  "whisper",
			ModelPath: "",
			Language:  "auto",
		},
		Detect: DetectConfig{
			Provider: "ollama",
			Ollama: OllamaConfig{
				BaseURL: "http://127.0.0.1:11434",
				Model:   "qwen2.5:0.5b",
			},
		},
		LLM: LLMConfig{
			Provider: "openai",
			OpenAI: OpenAIConfig{
				Model: "gpt-5.6-luna",
			},
			Ollama: OllamaConfig{
				BaseURL: "http://127.0.0.1:11434",
				Model:   "llama3.2",
			},
		},
		UI: UIConfig{
			Display:     1,
			AlwaysOnTop: true,
		},
		Assistant: AssistantConfig{
			Language:     "auto",
			SystemPrompt: DefaultSystemPrompt,
		},
	}
}

// Load reads YAML from path and merges onto Default().
func Load(path string) (Config, error) {
	cfg := Default()
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("read config: %w", err)
	}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse config: %w", err)
	}
	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	cfg.applyDefaults()
	return cfg, nil
}

func (c *Config) applyDefaults() {
	if c.Audio.SampleRate <= 0 {
		c.Audio.SampleRate = 16000
	}
	if strings.TrimSpace(c.STT.Provider) == "" {
		c.STT.Provider = "whisper"
	}
	if strings.TrimSpace(c.STT.Language) == "" {
		c.STT.Language = "auto"
	}
	if c.STT.Threads <= 0 {
		c.STT.Threads = 4
	}
	if strings.TrimSpace(c.Detect.Provider) == "" {
		c.Detect.Provider = "ollama"
	}
	if strings.TrimSpace(c.Detect.Ollama.BaseURL) == "" {
		c.Detect.Ollama.BaseURL = "http://127.0.0.1:11434"
	}
	if strings.TrimSpace(c.Detect.Ollama.Model) == "" {
		c.Detect.Ollama.Model = "qwen2.5:0.5b"
	}
	if strings.TrimSpace(c.LLM.Provider) == "" {
		c.LLM.Provider = "openai"
	}
	if strings.TrimSpace(c.LLM.OpenAI.Model) == "" {
		c.LLM.OpenAI.Model = "gpt-5.6-luna"
	}
	if strings.TrimSpace(c.LLM.Ollama.BaseURL) == "" {
		c.LLM.Ollama.BaseURL = "http://127.0.0.1:11434"
	}
	if strings.TrimSpace(c.LLM.Ollama.Model) == "" {
		c.LLM.Ollama.Model = "llama3.2"
	}
	if strings.TrimSpace(c.Assistant.Language) == "" {
		c.Assistant.Language = "auto"
	}
	if strings.TrimSpace(c.Assistant.SystemPrompt) == "" {
		c.Assistant.SystemPrompt = DefaultSystemPrompt
	}
}

// MicEnabled reports the initial microphone capture state from config.
func (c Config) MicEnabled() bool {
	return c.Audio.Microphone
}

// Validate checks configuration constraints.
func (c Config) Validate() error {
	mon := strings.TrimSpace(c.Audio.Monitor)
	if mon != "" && !strings.EqualFold(mon, "all") && !IsMonitorName(mon) {
		return fmt.Errorf("audio.monitor %q is not a playback monitor (must end with .monitor or be \"all\")", mon)
	}
	switch strings.ToLower(strings.TrimSpace(c.STT.Provider)) {
	case "", "whisper":
	default:
		return fmt.Errorf("stt.provider %q unsupported (want whisper)", c.STT.Provider)
	}
	switch strings.ToLower(strings.TrimSpace(c.Detect.Provider)) {
	case "", "ollama":
	default:
		return fmt.Errorf("detect.provider %q unsupported (want ollama)", c.Detect.Provider)
	}
	switch strings.ToLower(strings.TrimSpace(c.LLM.Provider)) {
	case "", "openai", "ollama":
	default:
		return fmt.Errorf("llm.provider %q unsupported (want openai or ollama)", c.LLM.Provider)
	}
	if c.UI.Display < 0 {
		return fmt.Errorf("ui.display must be >= 0")
	}
	return nil
}

// IsMonitorName reports whether name looks like a Pulse sink monitor (playback loopback).
func IsMonitorName(name string) bool {
	return strings.HasSuffix(strings.TrimSpace(name), ".monitor")
}

// SegmenterConfig builds a VAD segmenter config from audio settings.
func (a AudioConfig) SegmenterConfig() vad.Config {
	cfg := vad.DefaultConfig(a.SampleRate)
	v := a.VAD
	if v.EnergyThreshold > 0 {
		cfg.EnergyThreshold = v.EnergyThreshold
	}
	frameMs := cfg.FrameMs
	if v.HangoverMs > 0 {
		cfg.HangoverFrames = (v.HangoverMs + frameMs - 1) / frameMs
	}
	if v.MinSpeechMs > 0 {
		cfg.MinSpeechFrames = (v.MinSpeechMs + frameMs - 1) / frameMs
	}
	if v.MaxSpeechSec > 0 {
		cfg.MaxSpeechFrames = a.SampleRate * v.MaxSpeechSec / frameMs
	}
	return cfg
}
