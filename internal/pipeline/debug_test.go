package pipeline_test

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/Legion112/meeting-sidecar/internal/pipeline"
	"github.com/Legion112/meeting-sidecar/internal/ui"
	"github.com/Legion112/meeting-sidecar/internal/vad"
)

func TestPipelineDebugLogsSTT(t *testing.T) {
	cfg := vad.DefaultConfig(16000)
	cfg.EnergyThreshold = 100
	cfg.MinSpeechFrames = 2
	cfg.HangoverFrames = 2
	seg := vad.NewSegmenter(cfg)
	fs := seg.FrameSize()
	pcm := append(speechFrames(fs, 3, 3000), make([]int16, fs*3)...)

	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	r, err := pipeline.New(pipeline.Deps{
		Source: &src{chunks: [][]int16{pcm}}, Segmenter: seg,
		Transcriber: &tr{text: "What is X?"}, Gate: &gate{ok: false}, Completer: &comp{},
		HUD: ui.NewMemoryHUD(), SampleRate: 16000, Logger: logger,
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	_ = r.Run(ctx)

	out := buf.String()
	if !strings.Contains(out, "stt transcript") || !strings.Contains(out, "What is X?") {
		t.Fatalf("missing stt log: %q", out)
	}
	if !strings.Contains(out, "question gate") || !strings.Contains(out, "is_question=false") {
		t.Fatalf("missing gate log: %q", out)
	}
}
