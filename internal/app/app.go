package app

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"sync"
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
	Config        config.Config
	Source        audio.PCMSource
	RecordFactory audio.RecordFactory
	MicSource     string
	Transcriber   stt.Transcriber
	Gate          detect.Gate
	Completer     llm.Completer
	HUD           ui.HUD
	Logger        *slog.Logger
	EnsureModel   func(ctx context.Context, path string) error
	NewEngine     func(modelPath string, opts whisperstt.EngineOptions) (whisperstt.Engine, error)
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
	playbackRunner, err := pipeline.New(pipeline.Deps{
		Source:        d.Source,
		Segmenter:     vad.NewSegmenter(cfg.Audio.SegmenterConfig()),
		Transcriber:   d.Transcriber,
		Gate:          d.Gate,
		Completer:     d.Completer,
		HUD:           d.HUD,
		CaptionSource: ui.CaptionPlayback,
		SampleRate:    cfg.Audio.SampleRate,
		SystemPrompt:  cfg.Assistant.SystemPrompt,
		Logger:        d.Logger,
	})
	if err != nil {
		return err
	}

	runCtx, cancel := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer cancel()

	var micMu sync.Mutex
	var micCancel context.CancelFunc
	var micDone chan struct{}

	stopMic := func() {
		micMu.Lock()
		cancelMic := micCancel
		done := micDone
		micCancel = nil
		micDone = nil
		micMu.Unlock()
		if cancelMic != nil {
			cancelMic()
			<-done
		}
	}
	defer stopMic()

	startMic := func() error {
		if d.RecordFactory == nil || d.MicSource == "" {
			return fmt.Errorf("microphone capture is not configured")
		}
		micMu.Lock()
		defer micMu.Unlock()
		if micCancel != nil {
			return nil
		}
		src, err := audio.OpenInput(d.RecordFactory, d.MicSource, cfg.Audio.SampleRate)
		if err != nil {
			return err
		}
		micRunner, err := pipeline.New(pipeline.Deps{
			Source:        src,
			Segmenter:     vad.NewSegmenter(cfg.Audio.SegmenterConfigMic()),
			Transcriber:   d.Transcriber,
			Gate:          d.Gate,
			Completer:     d.Completer,
			HUD:           d.HUD,
			CaptionSource: ui.CaptionMicrophone,
			SampleRate:    cfg.Audio.SampleRate,
			SystemPrompt:  cfg.Assistant.SystemPrompt,
			Logger:        d.Logger,
		})
		if err != nil {
			_ = src.Close()
			return err
		}
		micCtx, cancel := context.WithCancel(runCtx)
		done := make(chan struct{})
		micCancel = cancel
		micDone = done
		go func() {
			defer close(done)
			defer func() { _ = src.Close() }()
			if err := micRunner.Run(micCtx); err != nil && micCtx.Err() == nil {
				d.Logger.Warn("microphone pipeline", "err", err)
			}
		}()
		return nil
	}

	setMicStatus := func(on bool) {
		if on {
			d.HUD.SetStatus("listening (playback + mic)")
		} else {
			d.HUD.SetStatus("listening")
		}
	}

	if d.RecordFactory != nil && d.MicSource != "" {
		initialMic := cfg.MicEnabled()
		if initialMic {
			if err := startMic(); err != nil {
				d.Logger.Warn("microphone startup", "err", err)
				d.HUD.SetStatus("microphone error: " + err.Error())
				initialMic = false
			}
		}
		d.HUD.BindMicCapture(initialMic, func(on bool) {
			if on {
				if err := startMic(); err != nil {
					d.Logger.Warn("microphone toggle", "enabled", on, "err", err)
					d.HUD.SetStatus("microphone error: " + err.Error())
					return
				}
				d.Logger.Info("microphone pipeline enabled", "source", d.MicSource)
				setMicStatus(true)
				return
			}
			stopMic()
			d.Logger.Info("microphone pipeline disabled")
			setMicStatus(false)
		})
		if initialMic {
			setMicStatus(true)
		}
	}

	errCh := make(chan error, 1)
	go func() {
		err := playbackRunner.Run(runCtx)
		stopMic()
		_ = d.HUD.Close()
		errCh <- err
	}()

	if err := d.HUD.Run(); err != nil {
		cancel()
		<-errCh
		return err
	}
	cancel()
	err = <-errCh
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
func NewWhisperTranscriber(ctx context.Context, cfg config.Config, ensure func(context.Context, string) error, newEngine func(string, whisperstt.EngineOptions) (whisperstt.Engine, error)) (stt.Transcriber, error) {
	if ensure == nil {
		ensure = func(ctx context.Context, path string) error {
			d := &whisperstt.Downloader{}
			return d.EnsureModel(ctx, path)
		}
	}
	if newEngine == nil {
		newEngine = whisperstt.NewNativeEngine
	}
	path, err := cfg.ResolveSTTModelPath()
	if err != nil {
		return nil, err
	}
	if err := ensure(ctx, path); err != nil {
		return nil, err
	}
	opts := whisperstt.EngineOptions{
		Language:      cfg.STTLanguage(),
		InitialPrompt: cfg.STT.InitialPrompt,
		Threads:       uint(cfg.STT.Threads),
	}
	eng, err := newEngine(path, opts)
	if err != nil {
		return nil, err
	}
	return &whisperstt.Client{Engine: eng}, nil
}
