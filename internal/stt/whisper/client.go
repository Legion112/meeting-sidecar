package whisper

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
)

// userHomeDir is swapped in tests.
var userHomeDir = os.UserHomeDir

// SetUserHomeDirForTest overrides home lookup (tests only).
func SetUserHomeDirForTest(fn func() (string, error)) {
	if fn == nil {
		userHomeDir = os.UserHomeDir
		return
	}
	userHomeDir = fn
}

// ResetUserHomeDirForTest restores os.UserHomeDir.
func ResetUserHomeDirForTest() {
	userHomeDir = os.UserHomeDir
}

// Engine runs Whisper inference on float32 mono PCM at 16 kHz.
type Engine interface {
	Transcribe(ctx context.Context, samples []float32) (string, error)
	Close() error
}

// Client is a Transcriber backed by a Whisper Engine.
type Client struct {
	Engine   Engine
	Language string
}

// Transcribe implements stt.Transcriber.
func (c *Client) Transcribe(ctx context.Context, pcm []int16, sampleRate int) (string, error) {
	if c == nil || c.Engine == nil {
		return "", fmt.Errorf("whisper: engine is nil")
	}
	if len(pcm) == 0 {
		return "", fmt.Errorf("whisper: empty pcm")
	}
	if sampleRate <= 0 {
		return "", fmt.Errorf("whisper: invalid sample rate")
	}
	samples := int16ToFloat32(pcm)
	if sampleRate != 16000 {
		samples = resampleLinear(samples, sampleRate, 16000)
	}
	text, err := c.Engine.Transcribe(ctx, samples)
	if err != nil {
		return "", fmt.Errorf("whisper: %w", err)
	}
	return text, nil
}

// Close releases the engine.
func (c *Client) Close() error {
	if c == nil || c.Engine == nil {
		return nil
	}
	return c.Engine.Close()
}

func int16ToFloat32(pcm []int16) []float32 {
	out := make([]float32, len(pcm))
	for i, v := range pcm {
		out[i] = float32(v) / 32768.0
	}
	return out
}

func resampleLinear(in []float32, from, to int) []float32 {
	if from == to || len(in) == 0 {
		return in
	}
	n := int(float64(len(in)) * float64(to) / float64(from))
	if n <= 0 {
		return nil
	}
	out := make([]float32, n)
	for i := 0; i < n; i++ {
		src := float64(i) * float64(from) / float64(to)
		j := int(src)
		frac := float32(src - float64(j))
		if j >= len(in)-1 {
			out[i] = in[len(in)-1]
			continue
		}
		out[i] = in[j]*(1-frac) + in[j+1]*frac
	}
	return out
}

// DefaultModelPath returns ~/.local/share/meeting-sidecar/models/ggml-small.bin
func DefaultModelPath() (string, error) {
	home, err := userHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".local", "share", "meeting-sidecar", "models", "ggml-small.bin"), nil
}

// ResolveModelPath returns configured path or the default.
func ResolveModelPath(configured string) (string, error) {
	if configured != "" {
		return configured, nil
	}
	return DefaultModelPath()
}
