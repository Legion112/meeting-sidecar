package pipeline_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Legion112/meeting-sidecar/internal/pipeline"
	"github.com/Legion112/meeting-sidecar/internal/ui"
	"github.com/Legion112/meeting-sidecar/internal/vad"
)

type zeroSrc struct {
	n int
}

func (z *zeroSrc) Read(ctx context.Context, dst []int16) (int, error) {
	z.n++
	if z.n < 3 {
		return 0, nil
	}
	<-ctx.Done()
	return 0, ctx.Err()
}
func (z *zeroSrc) Close() error { return nil }

type errSrc struct{}

func (errSrc) Read(ctx context.Context, dst []int16) (int, error) {
	return 0, errors.New("device lost")
}
func (errSrc) Close() error { return nil }

func TestPipelineZeroReadsAndAudioError(t *testing.T) {
	seg := vad.NewSegmenter(vad.DefaultConfig(16000))
	r, err := pipeline.New(pipeline.Deps{
		Source: &zeroSrc{}, Segmenter: seg, Transcriber: &tr{}, Gate: &gate{}, Completer: &comp{}, HUD: ui.NewMemoryHUD(),
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	_ = r.Run(ctx)

	r2, _ := pipeline.New(pipeline.Deps{
		Source: errSrc{}, Segmenter: vad.NewSegmenter(vad.DefaultConfig(16000)),
		Transcriber: &tr{}, Gate: &gate{}, Completer: &comp{}, HUD: ui.NewMemoryHUD(),
	})
	err = r2.Run(context.Background())
	if err == nil {
		t.Fatal("expected audio error")
	}
}

func TestHandleEmptyPCMViaFlushShort(t *testing.T) {
	// Flush with in-speech below min returns nothing; exercise cancel flush path with speech in progress
	cfg := vad.DefaultConfig(16000)
	cfg.EnergyThreshold = 100
	cfg.MinSpeechFrames = 50
	cfg.HangoverFrames = 50
	seg := vad.NewSegmenter(cfg)
	fs := seg.FrameSize()
	pcm := make([]int16, fs)
	for i := range pcm {
		pcm[i] = 3000
	}
	s := &src{chunks: [][]int16{pcm}}
	r, _ := pipeline.New(pipeline.Deps{
		Source: s, Segmenter: seg, Transcriber: &tr{text: "x"}, Gate: &gate{ok: true}, Completer: &comp{ans: "a"}, HUD: ui.NewMemoryHUD(),
	})
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	_ = r.Run(ctx)
}
