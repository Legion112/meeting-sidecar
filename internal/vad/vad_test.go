package vad_test

import (
	"testing"

	"github.com/Legion112/meeting-sidecar/internal/vad"
)

func tone(n int, amp int16) []int16 {
	out := make([]int16, n)
	for i := range out {
		if i%2 == 0 {
			out[i] = amp
		} else {
			out[i] = -amp
		}
	}
	return out
}

func silence(n int) []int16 { return make([]int16, n) }

func TestSegmenterSpeechAndSilence(t *testing.T) {
	cfg := vad.DefaultConfig(16000)
	cfg.EnergyThreshold = 100
	cfg.MinSpeechFrames = 2
	cfg.HangoverFrames = 2
	cfg.MaxSpeechFrames = 100
	s := vad.NewSegmenter(cfg)
	fs := s.FrameSize()
	if fs != 320 {
		t.Fatalf("frame size %d", fs)
	}

	// Pure silence never emits.
	if u := s.Push(silence(fs * 5)); len(u) != 0 {
		t.Fatalf("silence emitted %d", len(u))
	}

	// Short speech below min frames discarded after hangover.
	short := append(tone(fs, 3000), silence(fs*3)...)
	if u := s.Push(short); len(u) != 0 {
		t.Fatalf("short speech emitted")
	}

	// Real utterance: speech then silence hangover.
	pcm := append(tone(fs*3, 3000), silence(fs*3)...)
	u := s.Push(pcm)
	if len(u) != 1 {
		t.Fatalf("want 1 utterance, got %d", len(u))
	}
	if len(u[0].PCM) == 0 {
		t.Fatal("empty pcm")
	}

	// Empty push
	if u := s.Push(nil); u != nil {
		t.Fatal("nil push")
	}
}

func TestFlushAndMaxSpeech(t *testing.T) {
	cfg := vad.DefaultConfig(0)
	cfg.EnergyThreshold = 100
	cfg.MinSpeechFrames = 1
	cfg.HangoverFrames = 50
	cfg.MaxSpeechFrames = 3
	cfg.FrameMs = 20
	s := vad.NewSegmenter(cfg)
	fs := s.FrameSize()

	u := s.Push(tone(fs*3, 4000))
	if len(u) != 1 {
		t.Fatalf("max speech should flush, got %d", len(u))
	}

	s2 := vad.NewSegmenter(cfg)
	_ = s2.Push(tone(fs*2, 4000))
	flushed := s2.Flush()
	if len(flushed) != 1 {
		t.Fatalf("flush got %d", len(flushed))
	}
	if len(s2.Flush()) != 0 {
		t.Fatal("second flush")
	}
}

func TestDefaultConfigZeros(t *testing.T) {
	s := vad.NewSegmenter(vad.Config{})
	if s.FrameSize() <= 0 {
		t.Fatal("frame size")
	}
}
