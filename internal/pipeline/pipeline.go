package pipeline

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/Legion112/meeting-sidecar/internal/audio"
	"github.com/Legion112/meeting-sidecar/internal/detect"
	"github.com/Legion112/meeting-sidecar/internal/llm"
	"github.com/Legion112/meeting-sidecar/internal/stt"
	"github.com/Legion112/meeting-sidecar/internal/ui"
	"github.com/Legion112/meeting-sidecar/internal/vad"
)

// Deps wires pipeline stages.
type Deps struct {
	Source       audio.PCMSource
	Segmenter    *vad.Segmenter
	Transcriber  stt.Transcriber
	Gate         detect.Gate
	Completer    llm.Completer
	HUD          ui.HUD
	SampleRate   int
	SystemPrompt string
	Logger       *slog.Logger
}

// Runner executes capture → VAD → STT → gate → LLM → HUD.
type Runner struct {
	deps Deps
}

// New creates a Runner.
func New(d Deps) (*Runner, error) {
	if d.Source == nil {
		return nil, fmt.Errorf("pipeline: source is nil")
	}
	if d.Segmenter == nil {
		return nil, fmt.Errorf("pipeline: segmenter is nil")
	}
	if d.Transcriber == nil {
		return nil, fmt.Errorf("pipeline: transcriber is nil")
	}
	if d.Gate == nil {
		return nil, fmt.Errorf("pipeline: gate is nil")
	}
	if d.Completer == nil {
		return nil, fmt.Errorf("pipeline: completer is nil")
	}
	if d.HUD == nil {
		return nil, fmt.Errorf("pipeline: hud is nil")
	}
	if d.SampleRate <= 0 {
		d.SampleRate = 16000
	}
	if d.Logger == nil {
		d.Logger = slog.Default()
	}
	return &Runner{deps: d}, nil
}

// Run reads PCM until ctx is cancelled.
func (r *Runner) Run(ctx context.Context) error {
	d := r.deps
	d.HUD.SetStatus("listening")
	frame := make([]int16, d.Segmenter.FrameSize()*16)
	for {
		n, err := d.Source.Read(ctx, frame)
		if err != nil {
			r.flush(ctx)
			if ctx.Err() != nil {
				return ctx.Err()
			}
			d.HUD.SetStatus("audio error: " + err.Error())
			return err
		}
		if n == 0 {
			continue
		}
		d.HUD.PushAudio(frame[:n])
		for _, u := range d.Segmenter.Push(frame[:n]) {
			r.handleOrWarn(ctx, u.PCM)
		}
	}
}

func (r *Runner) flush(ctx context.Context) {
	for _, u := range r.deps.Segmenter.Flush() {
		r.handleOrWarn(ctx, u.PCM)
	}
}

func (r *Runner) handleOrWarn(ctx context.Context, pcm []int16) {
	if err := r.handleUtterance(ctx, pcm); err != nil {
		r.deps.Logger.Warn("utterance", "err", err)
		r.deps.HUD.SetStatus("error: " + err.Error())
	}
}

func (r *Runner) handleUtterance(ctx context.Context, pcm []int16) error {
	d := r.deps
	d.HUD.SetStatus("transcribing")
	text, err := d.Transcriber.Transcribe(ctx, pcm, d.SampleRate)
	if err != nil {
		return fmt.Errorf("stt: %w", err)
	}
	if text == "" {
		d.HUD.SetStatus("listening")
		return nil
	}
	d.HUD.SetStatus("detecting")
	ok, err := d.Gate.IsQuestion(ctx, text)
	if err != nil {
		return fmt.Errorf("detect: %w", err)
	}
	if !ok {
		d.HUD.SetStatus("listening")
		return nil
	}
	d.HUD.SetStatus("answering")
	answer, err := d.Completer.Complete(ctx, d.SystemPrompt, text)
	if err != nil {
		return fmt.Errorf("llm: %w", err)
	}
	d.HUD.ShowSuggestion(ui.Suggestion{Question: text, Answer: answer})
	d.HUD.SetStatus("listening")
	return nil
}
