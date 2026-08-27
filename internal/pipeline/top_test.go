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

type speechThenZero struct {
	pcm  []int16
	once bool
}

func (s *speechThenZero) Read(ctx context.Context, dst []int16) (int, error) {
	if !s.once {
		s.once = true
		return copy(dst, s.pcm), nil
	}
	select {
	case <-ctx.Done():
		return 0, ctx.Err()
	case <-time.After(2 * time.Millisecond):
		return 0, nil
	}
}
func (s *speechThenZero) Close() error { return nil }

func TestCancelAtLoopTopFlushes(t *testing.T) {
	cfg := vad.DefaultConfig(16000)
	cfg.EnergyThreshold = 100
	cfg.MinSpeechFrames = 2
	cfg.HangoverFrames = 200
	seg := vad.NewSegmenter(cfg)
	fs := seg.FrameSize()
	pcm := speechFrames(fs, 3, 3000)
	trr := &tr{err: errors.New("fail")}
	r, _ := pipeline.New(pipeline.Deps{
		Source: &speechThenZero{pcm: pcm}, Segmenter: seg, Transcriber: trr,
		Gate: &gate{}, Completer: &comp{}, HUD: ui.NewMemoryHUD(),
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(40 * time.Millisecond)
		cancel()
	}()
	_ = r.Run(ctx)
}

func TestPushUtteranceErrorContinues(t *testing.T) {
	cfg := vad.DefaultConfig(16000)
	cfg.EnergyThreshold = 100
	cfg.MinSpeechFrames = 2
	cfg.HangoverFrames = 2
	seg := vad.NewSegmenter(cfg)
	fs := seg.FrameSize()
	pcm := append(speechFrames(fs, 3, 3000), make([]int16, fs*4)...)
	done := make(chan struct{})
	src := &src{chunks: [][]int16{pcm}}
	go func() {
		time.Sleep(300 * time.Millisecond)
		close(done)
	}()
	r, _ := pipeline.New(pipeline.Deps{
		Source: src, Segmenter: seg, Transcriber: &tr{err: errors.New("x")},
		Gate: &gate{}, Completer: &comp{}, HUD: ui.NewMemoryHUD(),
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	ctx, cancel := context.WithTimeout(context.Background(), 400*time.Millisecond)
	defer cancel()
	_ = r.Run(ctx)
	<-done
}
