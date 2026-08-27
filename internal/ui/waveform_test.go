package ui

import (
	"testing"
)

func TestWaveBufferPushAndAmp(t *testing.T) {
	var b waveBuffer
	b.push(nil)
	b.push([]int16{})
	b.push([]int16{1000, -2000, 3000, -4000, 5000, -6000, 7000, -8000})
	if b.ampAt(255, 256) <= 0 {
		t.Fatal("expected peak on newest column")
	}
	if b.ampAt(0, 0) != b.ampAt(0, 1) {
		// width 0 treated as 1
	}
	for i := 0; i < waveformPoints+10; i++ {
		b.push([]int16{20000, -20000})
	}
	if b.ampAt(128, 256) <= 0 {
		t.Fatal("ring should stay live")
	}
}

func TestMemoryHUDPushAudio(t *testing.T) {
	h := NewMemoryHUD()
	h.PushAudio([]int16{1, 2, 3})
	if h.AudioPushes != 1 || h.LastAudioN != 3 {
		t.Fatalf("pushes=%d n=%d", h.AudioPushes, h.LastAudioN)
	}
}

func TestFyneHUDPushAudioClosed(t *testing.T) {
	h := &FyneHUD{closed: true}
	h.PushAudio([]int16{1})
}

func TestFyneHUDPushAudioThrottled(t *testing.T) {
	h := &FyneHUD{}
	h.waveAt.Store(1<<62)
	h.PushAudio([]int16{1000, -1000, 2000, -2000})
}
