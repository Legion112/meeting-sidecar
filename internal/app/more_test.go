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
	"github.com/Legion112/meeting-sidecar/internal/ui"
)

type quickHUD struct {
	inner *ui.MemoryHUD
	err   error
}

func (q *quickHUD) SetStatus(s string)                      { q.inner.SetStatus(s) }
func (q *quickHUD) AppendCaption(source ui.CaptionSource, text string) { q.inner.AppendCaption(source, text) }
func (q *quickHUD) BindMicCapture(initial bool, fn func(bool)) { q.inner.BindMicCapture(initial, fn) }
func (q *quickHUD) ShowSuggestion(s ui.Suggestion)          { q.inner.ShowSuggestion(s) }
func (q *quickHUD) PushAudio(samples []int16)                 { q.inner.PushAudio(samples) }
func (q *quickHUD) Hide()                                   { q.inner.Hide() }
func (q *quickHUD) Close() error                            { return q.inner.Close() }
func (q *quickHUD) Run() error {
	if q.err != nil {
		return q.err
	}
	return q.inner.Run()
}

type errSrc struct{}

func (errSrc) Read(ctx context.Context, dst []int16) (int, error) {
	return 0, errors.New("audio boom")
}
func (errSrc) Close() error { return nil }

func TestRunPipelineErrorAndHUDError(t *testing.T) {
	hud := ui.NewMemoryHUD()
	err := app.Run(context.Background(), app.Deps{
		Config:      config.Default(),
		Source:      errSrc{},
		Transcriber: tr{},
		Gate:        gate{},
		Completer:   comp{},
		HUD:         hud,
		Logger:      slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	if err == nil {
		t.Fatal("expected pipeline error")
	}

	h2 := &quickHUD{inner: ui.NewMemoryHUD(), err: errors.New("ui quit")}
	s := &src{done: make(chan struct{})}
	err = app.Run(context.Background(), app.Deps{
		Config:      config.Default(),
		Source:      s,
		Transcriber: tr{},
		Gate:        gate{},
		Completer:   comp{},
		HUD:         h2,
		Logger:      slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	if err == nil {
		t.Fatal("expected hud error")
	}
	time.Sleep(10 * time.Millisecond)
}

func TestNewWhisperDefaultEnsure(t *testing.T) {
	// exercise nil ensure branch that constructs Downloader — fail fast with bad path via engine after ensure noop file
	cfg := config.Default()
	cfg.STT.ModelPath = t.TempDir() + "/missing-will-try-download.bin"
	_, err := app.NewWhisperTranscriber(context.Background(), cfg,
		func(context.Context, string) error { return nil },
		nil, // uses NewNativeEngine stub
	)
	if err == nil {
		t.Fatal("stub engine")
	}
}
