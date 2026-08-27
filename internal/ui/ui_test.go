package ui_test

import (
	"testing"
	"time"

	"fyne.io/fyne/v2/test"

	"github.com/Legion112/meeting-sidecar/internal/ui"
)

func TestMemoryHUD(t *testing.T) {
	h := ui.NewMemoryHUD()
	h.SetStatus("ok")
	if h.Status != "ok" {
		t.Fatal(h.Status)
	}
	h.ShowSuggestion(ui.Suggestion{Question: "q", Answer: "a"})
	h.AppendCaption(ui.CaptionPlayback, "hello world")
	h.BindMicCapture(false, func(on bool) { h.MicEnabled = on })
	h.PushAudio([]int16{1, 2})
	if h.Last.Answer != "a" {
		t.Fatal(h.Last)
	}
	if len(h.Captions) != 1 || h.Captions[0].Text != "hello world" || h.Captions[0].Source != ui.CaptionPlayback {
		t.Fatalf("captions: %+v", h.Captions)
	}
	h.Hide()
	if !h.Hidden {
		t.Fatal("hidden")
	}
	go func() {
		time.Sleep(10 * time.Millisecond)
		_ = h.Close()
	}()
	if err := h.Run(); err != nil {
		t.Fatal(err)
	}
	if err := h.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestMemoryHUDCaptionSources(t *testing.T) {
	h := ui.NewMemoryHUD()
	h.AppendCaption(ui.CaptionPlayback, "from speakers")
	h.AppendCaption(ui.CaptionMicrophone, "from mic")
	if len(h.Captions) != 2 {
		t.Fatalf("captions: %+v", h.Captions)
	}
	if h.Captions[0].Source != ui.CaptionPlayback || h.Captions[1].Source != ui.CaptionMicrophone {
		t.Fatalf("sources: %+v", h.Captions)
	}
}

func TestFyneHUD(t *testing.T) {
	a := test.NewApp()
	defer a.Quit()
	h, err := ui.NewFyneHUD(ui.FyneOptions{
		Display:     1,
		AlwaysOnTop: true,
		Title:       "",
		App:         a,
	})
	if err != nil {
		t.Fatal(err)
	}
	h.SetStatus("listening")
	h.PushAudio([]int16{100, -100, 200})
	h.AppendCaption(ui.CaptionMicrophone, "Привет всем")
	h.BindMicCapture(true, nil)
	h.ShowSuggestion(ui.Suggestion{Question: "Q?", Answer: "A"})
	test.Tap(h.HideBtn)
	h.Hide()
	h.SetStatus("x")
	if err := h.Close(); err != nil {
		t.Fatal(err)
	}
	h.SetStatus("ignored")
	h.ShowSuggestion(ui.Suggestion{})
	h.Hide()
	if err := h.Close(); err != nil {
		t.Fatal(err)
	}

	h2, err := ui.NewFyneHUD(ui.FyneOptions{Title: "t", App: a, Display: 0, AlwaysOnTop: false})
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- h2.Run() }()
	time.Sleep(20 * time.Millisecond)
	_ = h2.Close()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("run hang")
	}
}

func TestNewFyneHUDRequiresApp(t *testing.T) {
	if _, err := ui.NewFyneHUD(ui.FyneOptions{}); err == nil {
		t.Fatal("expected error")
	}
}
