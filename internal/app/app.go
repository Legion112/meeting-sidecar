package app

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/Legion112/meeting-sidecar/internal/audio"
	"github.com/Legion112/meeting-sidecar/internal/config"
	"github.com/Legion112/meeting-sidecar/internal/detect"
	"github.com/Legion112/meeting-sidecar/internal/llm"
	"github.com/Legion112/meeting-sidecar/internal/pipeline"
	"github.com/Legion112/meeting-sidecar/internal/stt"
	whisperstt "github.com/Legion112/meeting-sidecar/internal/stt/whisper"
	"github.com/Legion112/meeting-sidecar/internal/ui"
	"github.com/Legion112/meeting-sidecar/internal/vad"
)

// Deps holds injectable runtime dependencies.
type Deps struct {
	Config      config.Config
	Source      audio.PCMSource
	Transcriber stt.Transcriber
	Gate        detect.Gate
	Completer   llm.Completer
	HUD         ui.HUD
	Logger      *slog.Logger
	EnsureModel func(ctx context.Context, path string) error
	NewEngine   func(modelPath, language string) (whisperstt.Engine, error)
}

// Run starts the meeting-sidecar pipeline until interrupted.
// HUD.Run blocks on the calling goroutine; use the main goroutine when the HUD is Fyne.
func Run(ctx context.Context, d Deps) error {
	if d.Logger == nil {
		d.Logger = slog.Default()
	}
	if d.Source == nil {
		return fmt.Errorf("app: audio source is nil")
	}
	if d.Transcriber == nil {
		return fmt.Errorf("app: transcriber is nil")
	}
	if d.Gate == nil {
		return fmt.Errorf("app: gate is nil")
	}
	if d.Completer == nil {
		return fmt.Errorf("app: completer is nil")
	}
	if d.HUD == nil {
		return fmt.Errorf("app: hud is nil")
	}

	cfg := d.Config
	seg := vad.NewSegmenter(vad.DefaultConfig(cfg.Audio.SampleRate))
	runner, _ := pipeline.New(pipeline.Deps{
		Source:       d.Source,
		Segmenter:    seg,
		Transcriber:  d.Transcriber,
		Gate:         d.Gate,
		Completer:    d.Completer,
		HUD:          d.HUD,
		SampleRate:   cfg.Audio.SampleRate,
		SystemPrompt: cfg.Assistant.SystemPrompt,
		Logger:       d.Logger,
	})

	runCtx, cancel := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		err := runner.Run(runCtx)
		_ = d.HUD.Close()
		errCh <- err
	}()

	if err := d.HUD.Run(); err != nil {
		cancel()
		<-errCh
		return err
	}
	cancel()
	err := <-errCh
	if err == context.Canceled || err == context.DeadlineExceeded {
		return nil
	}
	return err
}

// NewCompleter builds the configured answer Completer.
func NewCompleter(cfg config.Config) (llm.Completer, error) {
	switch strings.ToLower(strings.TrimSpace(cfg.LLM.Provider)) {
	case "", "openai":
		return &llm.OpenAICompleter{Model: cfg.LLM.OpenAI.Model}, nil
	case "ollama":
		return &llm.OllamaCompleter{
			BaseURL: cfg.LLM.Ollama.BaseURL,
			Model:   cfg.LLM.Ollama.Model,
		}, nil
	default:
		return nil, fmt.Errorf("unsupported llm.provider %q", cfg.LLM.Provider)
	}
}

// NewGate builds the question gate.
func NewGate(cfg config.Config) (detect.Gate, error) {
	switch strings.ToLower(strings.TrimSpace(cfg.Detect.Provider)) {
	case "", "ollama":
		return &detect.OllamaGate{
			BaseURL: cfg.Detect.Ollama.BaseURL,
			Model:   cfg.Detect.Ollama.Model,
		}, nil
	default:
		return nil, fmt.Errorf("unsupported detect.provider %q", cfg.Detect.Provider)
	}
}

// NewWhisperTranscriber downloads the model if needed and opens the engine.
func NewWhisperTranscriber(ctx context.Context, cfg config.Config, ensure func(context.Context, string) error, newEngine func(string, string) (whisperstt.Engine, error)) (stt.Transcriber, error) {
	if ensure == nil {
		ensure = func(ctx context.Context, path string) error {
			d := &whisperstt.Downloader{}
			return d.EnsureModel(ctx, path)
		}
	}
	if newEngine == nil {
		newEngine = whisperstt.NewNativeEngine
	}
	path, err := whisperstt.ResolveModelPath(cfg.STT.ModelPath)
	if err != nil {
		return nil, err
	}
	if err := ensure(ctx, path); err != nil {
		return nil, err
	}
	eng, err := newEngine(path, cfg.STT.Language)
	if err != nil {
		return nil, err
	}
	return &whisperstt.Client{Engine: eng, Language: cfg.STT.Language}, nil
}
