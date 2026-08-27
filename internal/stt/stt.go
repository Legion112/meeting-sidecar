package stt

import "context"

// Transcriber turns speech PCM into text.
type Transcriber interface {
	Transcribe(ctx context.Context, pcm []int16, sampleRate int) (string, error)
	Close() error
}
