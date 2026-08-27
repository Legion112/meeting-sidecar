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

type src struct {
	chunks [][]int16
	i      int
	err    error
}

func (s *src) Read(ctx context.Context, dst []int16) (int, error) {
	if s.err != nil && s.i >= len(s.chunks) {
		return 0, s.err
	}
	if s.i >= len(s.chunks) {
		<-ctx.Done()
		return 0, ctx.Err()
	}
	n := copy(dst, s.chunks[s.i])
	s.i++
	return n, nil
}
func (s *src) Close() error { return nil }

type tr struct {
	text string
	err  error
	n    int
}

func (t *tr) Transcribe(ctx context.Context, pcm []int16, sampleRate int) (string, error) {
	t.n++
	return t.text, t.err
}
func (t *tr) Close() error { return nil }

type gate struct {
	ok  bool
	err error
	n   int
}

func (g *gate) IsQuestion(ctx context.Context, text string) (bool, error) {
	g.n++
	return g.ok, g.err
}

type comp struct {
	ans string
	err error
	n   int
}

func (c *comp) Complete(ctx context.Context, systemPrompt, question string) (string, error) {
	c.n++
	return c.ans, c.err
}

func speechFrames(fs, n int, amp int16) []int16 {
	out := make([]int16, fs*n)
	for i := range out {
		if i%2 == 0 {
			out[i] = amp
		} else {
			out[i] = -amp
		}
	}
	return out
}

func TestPipelineQuestionPath(t *testing.T) {
	cfg := vad.DefaultConfig(16000)
	cfg.EnergyThreshold = 100
	cfg.MinSpeechFrames = 2
	cfg.HangoverFrames = 2
	seg := vad.NewSegmenter(cfg)
	fs := seg.FrameSize()

	pcm := append(speechFrames(fs, 3, 3000), make([]int16, fs*3)...)
	s := &src{chunks: [][]int16{pcm}}
	trr := &tr{text: "What is X?"}
	g := &gate{ok: true}
	c := &comp{ans: "Y"}
	hud := ui.NewMemoryHUD()

	r, err := pipeline.New(pipeline.Deps{
		Source: s, Segmenter: seg, Transcriber: trr, Gate: g, Completer: c, HUD: hud,
		SampleRate: 16000, SystemPrompt: "sys", Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	_ = r.Run(ctx)
	if trr.n == 0 || g.n == 0 || c.n == 0 {
		t.Fatalf("calls stt=%d gate=%d llm=%d", trr.n, g.n, c.n)
	}
	if hud.Last.Answer != "Y" {
		t.Fatalf("hud %+v", hud.Last)
	}
	if len(hud.Captions) != 1 || hud.Captions[0] != "What is X?" {
		t.Fatalf("captions: %+v", hud.Captions)
	}
	if hud.AudioPushes == 0 {
		t.Fatal("expected audio waveform pushes")
	}
}

func TestPipelineSkipsNonQuestion(t *testing.T) {
	cfg := vad.DefaultConfig(16000)
	cfg.EnergyThreshold = 100
	cfg.MinSpeechFrames = 2
	cfg.HangoverFrames = 2
	seg := vad.NewSegmenter(cfg)
	fs := seg.FrameSize()
	pcm := append(speechFrames(fs, 3, 3000), make([]int16, fs*3)...)
	s := &src{chunks: [][]int16{pcm}}
	trr := &tr{text: "okay"}
	g := &gate{ok: false}
	c := &comp{ans: "no"}
	hud := ui.NewMemoryHUD()
	r, err := pipeline.New(pipeline.Deps{
		Source: s, Segmenter: seg, Transcriber: trr, Gate: g, Completer: c, HUD: hud, SampleRate: 0,
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()
	_ = r.Run(ctx)
	if c.n != 0 {
		t.Fatal("llm should not run")
	}
	if len(hud.Captions) != 1 || hud.Captions[0] != "okay" {
		t.Fatalf("captions: %+v", hud.Captions)
	}
}

func TestPipelineSilenceNeverSTT(t *testing.T) {
	cfg := vad.DefaultConfig(16000)
	seg := vad.NewSegmenter(cfg)
	fs := seg.FrameSize()
	s := &src{chunks: [][]int16{make([]int16, fs*4)}}
	trr := &tr{text: "x"}
	r, err := pipeline.New(pipeline.Deps{
		Source: s, Segmenter: seg, Transcriber: trr, Gate: &gate{}, Completer: &comp{}, HUD: ui.NewMemoryHUD(),
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 80*time.Millisecond)
	defer cancel()
	_ = r.Run(ctx)
	if trr.n != 0 {
		t.Fatal("stt on silence")
	}
}

func TestNewValidation(t *testing.T) {
	seg := vad.NewSegmenter(vad.DefaultConfig(16000))
	hud := ui.NewMemoryHUD()
	cases := []pipeline.Deps{
		{Segmenter: seg, Transcriber: &tr{}, Gate: &gate{}, Completer: &comp{}, HUD: hud},
		{Source: &src{}, Transcriber: &tr{}, Gate: &gate{}, Completer: &comp{}, HUD: hud},
		{Source: &src{}, Segmenter: seg, Gate: &gate{}, Completer: &comp{}, HUD: hud},
		{Source: &src{}, Segmenter: seg, Transcriber: &tr{}, Completer: &comp{}, HUD: hud},
		{Source: &src{}, Segmenter: seg, Transcriber: &tr{}, Gate: &gate{}, HUD: hud},
		{Source: &src{}, Segmenter: seg, Transcriber: &tr{}, Gate: &gate{}, Completer: &comp{}},
	}
	for i, d := range cases {
		if _, err := pipeline.New(d); err == nil {
			t.Fatalf("case %d", i)
		}
	}
}

func TestPipelineErrors(t *testing.T) {
	cfg := vad.DefaultConfig(16000)
	cfg.EnergyThreshold = 100
	cfg.MinSpeechFrames = 2
	cfg.HangoverFrames = 2
	seg := vad.NewSegmenter(cfg)
	fs := seg.FrameSize()
	pcm := append(speechFrames(fs, 3, 3000), make([]int16, fs*3)...)

	s := &src{chunks: [][]int16{pcm}, err: errors.New("eof")}
	// after chunks, returns err - but first processes speech
	trr := &tr{err: errors.New("stt")}
	r, _ := pipeline.New(pipeline.Deps{
		Source: s, Segmenter: seg, Transcriber: trr, Gate: &gate{ok: true}, Completer: &comp{}, HUD: ui.NewMemoryHUD(),
	})
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	_ = r.Run(ctx)

	seg2 := vad.NewSegmenter(cfg)
	s2 := &src{chunks: [][]int16{pcm}}
	r2, _ := pipeline.New(pipeline.Deps{
		Source: s2, Segmenter: seg2, Transcriber: &tr{text: "q?"}, Gate: &gate{err: errors.New("g")}, Completer: &comp{}, HUD: ui.NewMemoryHUD(),
	})
	ctx2, cancel2 := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel2()
	_ = r2.Run(ctx2)

	seg3 := vad.NewSegmenter(cfg)
	s3 := &src{chunks: [][]int16{pcm}}
	r3, _ := pipeline.New(pipeline.Deps{
		Source: s3, Segmenter: seg3, Transcriber: &tr{text: "q?"}, Gate: &gate{ok: true}, Completer: &comp{err: errors.New("l")}, HUD: ui.NewMemoryHUD(),
	})
	ctx3, cancel3 := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel3()
	_ = r3.Run(ctx3)

	seg4 := vad.NewSegmenter(cfg)
	s4 := &src{chunks: [][]int16{pcm}}
	r4, _ := pipeline.New(pipeline.Deps{
		Source: s4, Segmenter: seg4, Transcriber: &tr{text: ""}, Gate: &gate{ok: true}, Completer: &comp{}, HUD: ui.NewMemoryHUD(),
	})
	ctx4, cancel4 := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel4()
	_ = r4.Run(ctx4)
}
