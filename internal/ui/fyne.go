package ui

import (
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

// FyneHUD is an always-on-top overlay window.
type FyneHUD struct {
	app       fyne.App
	win       fyne.Window
	status    *widget.Label
	waveform  fyne.CanvasObject
	waveBuf   waveBuffer
	waveAt    atomic.Int64
	captions  *widget.Label
	captionLines []string
	question  *widget.Label
	answer    *widget.Label
	micCheck  *widget.Check
	HideBtn   *widget.Button
	display   int
	alwaysTop bool
	mu        sync.Mutex
	closed    bool
	running   bool
}

// FyneOptions configures the overlay.
type FyneOptions struct {
	Display     int
	AlwaysOnTop bool
	Title       string
	App         fyne.App // required
}

// NewFyneHUD creates the overlay. opts.App is required (created in cmd or tests).
func NewFyneHUD(opts FyneOptions) (*FyneHUD, error) {
	if opts.App == nil {
		return nil, fmt.Errorf("fyne app is required")
	}
	return newFyneHUD(opts.App, opts), nil
}

func newFyneHUD(a fyne.App, opts FyneOptions) *FyneHUD {
	title := opts.Title
	if title == "" {
		title = "meeting-sidecar"
	}
	w := a.NewWindow(title)
	status := widget.NewLabel(fmt.Sprintf("display %d — place on non-shared monitor", opts.Display))
	question := widget.NewLabel("")
	question.Wrapping = fyne.TextWrapWord
	answer := widget.NewLabel("")
	answer.Wrapping = fyne.TextWrapWord
	captions := widget.NewLabel("Transcript appears here…")
	captions.Wrapping = fyne.TextWrapWord
	captionScroll := container.NewScroll(captions)
	captionScroll.SetMinSize(fyne.NewSize(380, 140))
	h := &FyneHUD{
		app:       a,
		win:       w,
		status:    status,
		captions:  captions,
		question:  question,
		answer:    answer,
		display:   opts.Display,
		alwaysTop: opts.AlwaysOnTop,
	}
	h.HideBtn = widget.NewButton("Hide", func() { h.Hide() })
	micCheck := widget.NewCheck("Capture microphone", nil)
	h.micCheck = micCheck
	wave := sizedWaveformRaster(&h.waveBuf)
	h.waveform = wave
	w.SetContent(container.NewVBox(
		widget.NewLabel("Meeting sidecar (private)"),
		status,
		micCheck,
		widget.NewLabel("Audio level"),
		wave,
		widget.NewSeparator(),
		widget.NewLabel("Transcript"),
		captionScroll,
		widget.NewSeparator(),
		widget.NewLabel("Last question"),
		question,
		widget.NewLabel("Suggested answer"),
		answer,
		h.HideBtn,
	))
	w.Resize(fyne.NewSize(420, 520))
	return h
}

// PushAudio implements HUD.
func (h *FyneHUD) PushAudio(samples []int16) {
	h.mu.Lock()
	if h.closed {
		h.mu.Unlock()
		return
	}
	h.mu.Unlock()
	h.waveBuf.push(samples)
	now := time.Now().UnixMilli()
	last := h.waveAt.Load()
	if now-last < 40 {
		return
	}
	h.waveAt.Store(now)
	fyne.Do(func() {
		h.mu.Lock()
		defer h.mu.Unlock()
		if h.closed || h.waveform == nil {
			return
		}
		h.waveform.Refresh()
	})
}

// SetStatus implements HUD.
func (h *FyneHUD) SetStatus(status string) {
	h.mu.Lock()
	if h.closed {
		h.mu.Unlock()
		return
	}
	h.mu.Unlock()
	fyne.Do(func() {
		h.mu.Lock()
		defer h.mu.Unlock()
		if h.closed {
			return
		}
		h.status.SetText(status)
		h.win.Show()
	})
}

// AppendCaption implements HUD.
func (h *FyneHUD) AppendCaption(text string) {
	text = strings.TrimSpace(text)
	if text == "" {
		return
	}
	h.mu.Lock()
	if h.closed {
		h.mu.Unlock()
		return
	}
	h.mu.Unlock()
	fyne.Do(func() {
		h.mu.Lock()
		defer h.mu.Unlock()
		if h.closed || h.captions == nil {
			return
		}
		h.captionLines = append(h.captionLines, text)
		if len(h.captionLines) > maxCaptionLines {
			h.captionLines = h.captionLines[len(h.captionLines)-maxCaptionLines:]
		}
		h.captions.SetText(strings.Join(h.captionLines, "\n"))
		h.win.Show()
	})
}

// BindMicCapture implements HUD.
func (h *FyneHUD) BindMicCapture(initial bool, onChange func(enabled bool)) {
	h.mu.Lock()
	if h.closed {
		h.mu.Unlock()
		return
	}
	h.mu.Unlock()
	fyne.Do(func() {
		h.mu.Lock()
		defer h.mu.Unlock()
		if h.closed || h.micCheck == nil {
			return
		}
		h.micCheck.SetChecked(initial)
		h.micCheck.OnChanged = func(checked bool) {
			if onChange != nil {
				onChange(checked)
			}
		}
	})
}

// ShowSuggestion implements HUD.
func (h *FyneHUD) ShowSuggestion(s Suggestion) {
	h.mu.Lock()
	if h.closed {
		h.mu.Unlock()
		return
	}
	h.mu.Unlock()
	fyne.Do(func() {
		h.mu.Lock()
		defer h.mu.Unlock()
		if h.closed {
			return
		}
		h.question.SetText(s.Question)
		h.answer.SetText(s.Answer)
		h.win.Show()
		h.win.RequestFocus()
	})
}

// Hide implements HUD.
func (h *FyneHUD) Hide() {
	h.mu.Lock()
	if h.closed {
		h.mu.Unlock()
		return
	}
	h.mu.Unlock()
	fyne.Do(func() {
		h.mu.Lock()
		defer h.mu.Unlock()
		if h.closed {
			return
		}
		h.win.Hide()
	})
}

// Run shows the window. Blocks on the Fyne event loop until Quit (call from main goroutine).
func (h *FyneHUD) Run() error {
	h.mu.Lock()
	h.running = true
	h.mu.Unlock()
	h.win.Show()
	h.app.Run()
	return nil
}

// Close quits the Fyne app.
func (h *FyneHUD) Close() error {
	h.mu.Lock()
	if h.closed {
		h.mu.Unlock()
		return nil
	}
	h.closed = true
	h.mu.Unlock()
	fyne.Do(func() {
		h.app.Quit()
	})
	return nil
}
