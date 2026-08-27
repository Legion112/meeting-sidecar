package audio

import (
	"context"
	"fmt"
)

// RecordControl starts and stops an underlying capture stream.
type RecordControl struct {
	Start func()
	Stop  func()
}

// RecordFactory opens a capture stream that delivers int16 mono samples via onSamples.
type RecordFactory func(monitor string, sampleRate int, onSamples func([]int16)) (RecordControl, error)

// PulseSource captures mono int16 PCM using an injected RecordFactory.
type PulseSource struct {
	ctrl RecordControl
	buf  *StreamBuffer
}

// OpenMonitor validates monitor/rate and opens capture via factory.
func OpenMonitor(factory RecordFactory, monitor string, sampleRate int) (*PulseSource, error) {
	if factory == nil {
		return nil, fmt.Errorf("record factory is nil")
	}
	if sampleRate <= 0 {
		return nil, fmt.Errorf("sample rate must be positive")
	}
	if len(monitor) < 8 || monitor[len(monitor)-8:] != ".monitor" {
		return nil, fmt.Errorf("%w: %q", ErrNotMonitor, monitor)
	}

	buf := NewStreamBuffer()
	ctrl, err := factory(monitor, sampleRate, func(samples []int16) {
		cp := make([]int16, len(samples))
		copy(cp, samples)
		buf.Push(cp)
	})
	if err != nil {
		return nil, fmt.Errorf("open pulse monitor %q: %w", monitor, err)
	}
	if ctrl.Start != nil {
		ctrl.Start()
	}
	return &PulseSource{ctrl: ctrl, buf: buf}, nil
}

// Read implements PCMSource.
func (p *PulseSource) Read(ctx context.Context, dst []int16) (int, error) {
	return p.buf.Read(ctx, dst)
}

// Close stops capture and closes the buffer.
func (p *PulseSource) Close() error {
	if p.ctrl.Stop != nil {
		p.ctrl.Stop()
	}
	return p.buf.Close()
}
