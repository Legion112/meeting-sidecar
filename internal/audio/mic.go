package audio

import (
	"fmt"
	"strings"
)

// SourceQuerier resolves the default Pulse input source.
type SourceQuerier interface {
	DefaultSource() (string, error)
}

// ResolveMicrophone returns an explicit Pulse source id or the default input device.
func ResolveMicrophone(explicit string, q SourceQuerier) (string, error) {
	name := strings.TrimSpace(explicit)
	if name != "" {
		if strings.HasSuffix(name, ".monitor") {
			return "", fmt.Errorf("microphone source %q is a playback monitor, not an input", name)
		}
		return name, nil
	}
	if q == nil {
		return "", fmt.Errorf("resolve microphone: source querier is nil")
	}
	src, err := q.DefaultSource()
	if err != nil {
		return "", fmt.Errorf("resolve default source: %w", err)
	}
	src = strings.TrimSpace(src)
	if src == "" {
		return "", fmt.Errorf("resolve default source: empty name")
	}
	return src, nil
}

// OpenInput opens a Pulse input source (microphone) for mono int16 PCM capture.
func OpenInput(factory RecordFactory, source string, sampleRate int) (*PulseSource, error) {
	if factory == nil {
		return nil, fmt.Errorf("record factory is nil")
	}
	if sampleRate <= 0 {
		return nil, fmt.Errorf("sample rate must be positive")
	}
	source = strings.TrimSpace(source)
	if source == "" {
		return nil, fmt.Errorf("input source is empty")
	}
	if strings.HasSuffix(source, ".monitor") {
		return nil, fmt.Errorf("input source %q is a playback monitor, not an input", source)
	}

	buf := NewStreamBuffer()
	ctrl, err := factory(source, sampleRate, func(samples []int16) {
		cp := make([]int16, len(samples))
		copy(cp, samples)
		buf.Push(cp)
	})
	if err != nil {
		return nil, fmt.Errorf("open pulse input %q: %w", source, err)
	}
	if ctrl.Start != nil {
		ctrl.Start()
	}
	return &PulseSource{ctrl: ctrl, buf: buf}, nil
}
