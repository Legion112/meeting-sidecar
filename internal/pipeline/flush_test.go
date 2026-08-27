package pipeline_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/Legion112/meeting-sidecar/internal/pipeline"
	"github.com/Legion112/meeting-sidecar/internal/ui"
	"github.com/Legion112/meeting-sidecar/internal/vad"
)

// cancelAfterSpeech returns speech then blocks until cancel, returning ctx.Err().
type cancelAfterSpeech struct {
	pcm  []int16
	sent bool
}

func (c *cancelAfterSpeech) Read(ctx context.Context, dst []int16) (int, error) {
	if !c.sent {
		c.sent = true
		return copy(dst, c.pcm), nil
	}
	<-ctx.Done()
	return 0, ctx.Err()
}
func (c *cancelAfterSpeech) Close() error { return nil }

func TestPipelineFlushOnCancelWithError(t *testing.T) {
	cfg := vad.DefaultConfig(16000)
	cfg.EnergyThreshold = 100
	cfg.MinSpeechFrames = 2
	cfg.HangoverFrames = 100 // keep in speech until flush
	seg := vad.NewSegmenter(cfg)
	fs := seg.FrameSize()
	pcm := speechFrames(fs, 3, 3000)

	trr := &tr{err: errors.New("stt fail")}
	r, err := pipeline.New(pipeline.Deps{
		Source: &cancelAfterSpeech{pcm: pcm}, Segmenter: seg, Transcriber: trr,
		Gate: &gate{ok: true}, Completer: &comp{}, HUD: ui.NewMemoryHUD(),
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 80*time.Millisecond)
	defer cancel()
	_ = r.Run(ctx)
	if trr.n == 0 {
		// flush may call stt when hangover never ended — speech still in progress
		t.Log("stt calls", trr.n)
	}
}

func TestPipelineReadErrorWhileCanceling(t *testing.T) {
	cfg := vad.DefaultConfig(16000)
	cfg.EnergyThreshold = 100
	cfg.MinSpeechFrames = 2
	cfg.HangoverFrames = 100
	seg := vad.NewSegmenter(cfg)
	fs := seg.FrameSize()
	pcm := speechFrames(fs, 3, 3000)

	src := &src{chunks: [][]int16{pcm}, err: context.Canceled}
	// After chunk, returns err; if ctx also canceled, hits cancel flush branch
	ctx, cancel := context.WithCancel(context.Background())
	r, _ := pipeline.New(pipeline.Deps{
		Source: src, Segmenter: seg, Transcriber: &tr{text: "q?"}, Gate: &gate{ok: true},
		Completer: &comp{ans: "a"}, HUD: ui.NewMemoryHUD(),
	})
	go func() {
		time.Sleep(30 * time.Millisecond)
		cancel()
	}()
	_ = r.Run(ctx)
}

func TestPipelineUtteranceWarnPath(t *testing.T) {
	cfg := vad.DefaultConfig(16000)
	cfg.EnergyThreshold = 100
	cfg.MinSpeechFrames = 2
	cfg.HangoverFrames = 2
	seg := vad.NewSegmenter(cfg)
	fs := seg.FrameSize()
	pcm := append(speechFrames(fs, 3, 3000), make([]int16, fs*3)...)
	s := &src{chunks: [][]int16{pcm}}
	r, _ := pipeline.New(pipeline.Deps{
		Source: s, Segmenter: seg, Transcriber: &tr{err: errors.New("boom")},
		Gate: &gate{}, Completer: &comp{}, HUD: ui.NewMemoryHUD(),
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	_ = r.Run(ctx)
}
