package app_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/Legion112/meeting-sidecar/internal/app"
	"github.com/Legion112/meeting-sidecar/internal/config"
	whisperstt "github.com/Legion112/meeting-sidecar/internal/stt/whisper"
	"github.com/Legion112/meeting-sidecar/internal/ui"
)

type src struct {
	done chan struct{}
}

func (s *src) Read(ctx context.Context, dst []int16) (int, error) {
	select {
	case <-ctx.Done():
		return 0, ctx.Err()
	case <-s.done:
		return 0, context.Canceled
	}
}
func (s *src) Close() error { return nil }

type tr struct{}

func (tr) Transcribe(context.Context, []int16, int) (string, error) { return "", nil }
func (tr) Close() error                                             { return nil }

type gate struct{}

func (gate) IsQuestion(context.Context, string) (bool, error) { return false, nil }

type comp struct{}

func (comp) Complete(context.Context, string, string) (string, error) { return "", nil }

type eng struct{}

func (eng) Transcribe(context.Context, []float32) (string, error) { return "t", nil }
func (eng) Close() error                                          { return nil }

func TestRunValidation(t *testing.T) {
	ctx := context.Background()
	cfg := config.Default()
	hud := ui.NewMemoryHUD()
	if err := app.Run(ctx, app.Deps{Config: cfg}); err == nil {
		t.Fatal("source")
	}
	if err := app.Run(ctx, app.Deps{Config: cfg, Source: &src{done: make(chan struct{})}}); err == nil {
		t.Fatal("tr")
	}
	if err := app.Run(ctx, app.Deps{Config: cfg, Source: &src{done: make(chan struct{})}, Transcriber: tr{}}); err == nil {
		t.Fatal("gate")
	}
	if err := app.Run(ctx, app.Deps{Config: cfg, Source: &src{done: make(chan struct{})}, Transcriber: tr{}, Gate: gate{}}); err == nil {
		t.Fatal("comp")
	}
	if err := app.Run(ctx, app.Deps{Config: cfg, Source: &src{done: make(chan struct{})}, Transcriber: tr{}, Gate: gate{}, Completer: comp{}}); err == nil {
		t.Fatal("hud")
	}
	_ = hud
}

func TestRunCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	hud := ui.NewMemoryHUD()
	s := &src{done: make(chan struct{})}
	go func() {
		time.Sleep(30 * time.Millisecond)
		cancel()
	}()
	err := app.Run(ctx, app.Deps{
		Config:      config.Default(),
		Source:      s,
		Transcriber: tr{},
		Gate:        gate{},
		Completer:   comp{},
		HUD:         hud,
		Logger:      slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestFactories(t *testing.T) {
	cfg := config.Default()
	c, err := app.NewCompleter(cfg)
	if err != nil || c == nil {
		t.Fatal(err)
	}
	cfg.LLM.Provider = "ollama"
	c, err = app.NewCompleter(cfg)
	if err != nil || c == nil {
		t.Fatal(err)
	}
	cfg.LLM.Provider = "nope"
	if _, err := app.NewCompleter(cfg); err == nil {
		t.Fatal("bad llm")
	}
	cfg = config.Default()
	g, err := app.NewGate(cfg)
	if err != nil || g == nil {
		t.Fatal(err)
	}
	cfg.Detect.Provider = "x"
	if _, err := app.NewGate(cfg); err == nil {
		t.Fatal("bad gate")
	}

	trr, err := app.NewWhisperTranscriber(context.Background(), config.Default(),
		func(context.Context, string) error { return nil },
		func(string, string) (whisperstt.Engine, error) { return eng{}, nil },
	)
	if err != nil || trr == nil {
		t.Fatal(err)
	}
	_ = trr.Close()

	_, err = app.NewWhisperTranscriber(context.Background(), config.Default(),
		func(context.Context, string) error { return errors.New("dl") },
		nil,
	)
	if err == nil {
		t.Fatal("ensure err")
	}
	_, err = app.NewWhisperTranscriber(context.Background(), config.Default(),
		func(context.Context, string) error { return nil },
		func(string, string) (whisperstt.Engine, error) { return nil, errors.New("eng") },
	)
	if err == nil {
		t.Fatal("engine err")
	}

	// nil newEngine uses NewNativeEngine — fails without a real ggml model file
	_, err = app.NewWhisperTranscriber(context.Background(), config.Default(),
		func(ctx context.Context, path string) error { return nil },
		nil,
	)
	if err == nil {
		t.Fatal("expected native engine failure without model")
	}
}
