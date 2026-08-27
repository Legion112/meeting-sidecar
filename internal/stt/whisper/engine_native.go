package whisper

import (
	"context"
	"fmt"
	"strings"

	wpkg "github.com/ggerganov/whisper.cpp/bindings/go/pkg/whisper"
)

// loadModel is swapped in tests so NativeEngine can be covered without GPU weights.
var loadModel = wpkg.New

// NativeEngine wraps whisper.cpp Go bindings (CUDA-linked libwhisper on this host).
type NativeEngine struct {
	model wpkg.Model
	lang  string
}

// NewNativeEngine loads a ggml model from path.
func NewNativeEngine(modelPath, language string) (Engine, error) {
	if modelPath == "" {
		return nil, fmt.Errorf("whisper model path is empty")
	}
	model, err := loadModel(modelPath)
	if err != nil {
		return nil, fmt.Errorf("load whisper model: %w", err)
	}
	return &NativeEngine{model: model, lang: language}, nil
}

// Transcribe implements Engine.
func (e *NativeEngine) Transcribe(ctx context.Context, samples []float32) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if e == nil || e.model == nil {
		return "", fmt.Errorf("whisper engine is nil")
	}
	ctxw, err := e.model.NewContext()
	if err != nil {
		return "", err
	}
	if e.lang != "" && e.lang != "auto" {
		_ = ctxw.SetLanguage(e.lang)
	}
	if err := ctxw.Process(samples, nil, nil, nil); err != nil {
		return "", err
	}
	var b strings.Builder
	for {
		seg, err := ctxw.NextSegment()
		if err != nil {
			break
		}
		b.WriteString(seg.Text)
	}
	return strings.TrimSpace(b.String()), nil
}

// Close implements Engine.
func (e *NativeEngine) Close() error {
	if e == nil || e.model == nil {
		return nil
	}
	e.model.Close()
	return nil
}
