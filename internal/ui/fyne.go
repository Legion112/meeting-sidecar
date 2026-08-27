package ui

import (
	"fmt"
	"sync"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

// FyneHUD is an always-on-top overlay window.
type FyneHUD struct {
	app       fyne.App
	win       fyne.Window
	status    *widget.Label
	question  *widget.Label
	answer    *widget.Label
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
	h := &FyneHUD{
		app:       a,
		win:       w,
		status:    status,
		question:  question,
		answer:    answer,
		display:   opts.Display,
		alwaysTop: opts.AlwaysOnTop,
	}
	h.HideBtn = widget.NewButton("Hide", func() { h.Hide() })
	w.SetContent(container.NewVBox(
		widget.NewLabel("Meeting sidecar (private)"),
		status,
		widget.NewSeparator(),
		widget.NewLabel("Question"),
		question,
		widget.NewLabel("Suggested answer"),
		answer,
		h.HideBtn,
	))
	w.Resize(fyne.NewSize(420, 320))
	return h
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
