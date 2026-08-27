package vad_test

import (
	"testing"

	"github.com/Legion112/meeting-sidecar/internal/vad"
)

func TestNewSegmenterClamps(t *testing.T) {
	s := vad.NewSegmenter(vad.Config{
		SampleRate: 1, FrameMs: 1, EnergyThreshold: -1,
		HangoverFrames: -1, MinSpeechFrames: -1, MaxSpeechFrames: -1,
	})
	if s.FrameSize() <= 0 {
		t.Fatal("frame")
	}
	// Flush with speech below min
	cfg := vad.DefaultConfig(16000)
	cfg.EnergyThreshold = 10
	cfg.MinSpeechFrames = 100
	s2 := vad.NewSegmenter(cfg)
	fs := s2.FrameSize()
	tone := make([]int16, fs)
	for i := range tone {
		tone[i] = 2000
	}
	_ = s2.Push(tone)
	if u := s2.Flush(); len(u) != 0 {
		t.Fatal("short flush")
	}
}
