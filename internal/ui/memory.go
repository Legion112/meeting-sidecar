package ui

import (
	"sync"
)

const maxCaptionLines = 200

// MemoryHUD is a testable in-memory HUD.
type MemoryHUD struct {
	mu          sync.Mutex
	Status      string
	Captions    []CaptionLine
	MicEnabled  bool
	micOnChange func(enabled bool)
	Last        Suggestion
	Hidden      bool
	Closed      bool
	AudioPushes int
	LastAudioN  int
	runOnce     sync.Once
	runDone     chan struct{}
}

// NewMemoryHUD creates a headless HUD.
func NewMemoryHUD() *MemoryHUD {
	return &MemoryHUD{runDone: make(chan struct{})}
}

// SetStatus implements HUD.
func (h *MemoryHUD) SetStatus(status string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.Status = status
	h.Hidden = false
}

// AppendCaption implements HUD.
func (h *MemoryHUD) AppendCaption(source CaptionSource, text string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if text == "" {
		return
	}
	h.Captions = append(h.Captions, CaptionLine{Source: source, Text: text})
	h.Hidden = false
}

// BindMicCapture implements HUD.
func (h *MemoryHUD) BindMicCapture(initial bool, onChange func(enabled bool)) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.MicEnabled = initial
	h.micOnChange = onChange
}

// ShowSuggestion implements HUD.
func (h *MemoryHUD) ShowSuggestion(s Suggestion) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.Last = s
	h.Hidden = false
}

// PushAudio implements HUD.
func (h *MemoryHUD) PushAudio(samples []int16) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.AudioPushes++
	h.LastAudioN = len(samples)
}

// Hide implements HUD.
func (h *MemoryHUD) Hide() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.Hidden = true
}

// Run waits until Close.
func (h *MemoryHUD) Run() error {
	<-h.runDone
	return nil
}

// Close stops Run.
func (h *MemoryHUD) Close() error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.Closed {
		return nil
	}
	h.Closed = true
	h.runOnce.Do(func() { close(h.runDone) })
	return nil
}
