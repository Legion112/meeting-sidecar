package audio

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// ErrNotMonitor is returned when a non-monitor (e.g. microphone) source is requested.
var ErrNotMonitor = errors.New("audio source is not a playback monitor")

// PCMSource streams mono int16 PCM from a playback monitor.
type PCMSource interface {
	Read(ctx context.Context, dst []int16) (int, error)
	Close() error
}

// SinkQuerier resolves the default Pulse sink name.
type SinkQuerier interface {
	DefaultSink() (string, error)
}

// ResolveMonitor returns an explicit monitor name or default_sink+".monitor".
// Non-monitor names are rejected (v1 has no microphone support).
func ResolveMonitor(explicit string, q SinkQuerier) (string, error) {
	name := strings.TrimSpace(explicit)
	if name == "" {
		if q == nil {
			return "", fmt.Errorf("resolve monitor: sink querier is nil")
		}
		sink, err := q.DefaultSink()
		if err != nil {
			return "", fmt.Errorf("resolve default sink: %w", err)
		}
		sink = strings.TrimSpace(sink)
		if sink == "" {
			return "", fmt.Errorf("resolve default sink: empty name")
		}
		name = sink + ".monitor"
	}
	if !strings.HasSuffix(name, ".monitor") {
		return "", fmt.Errorf("%w: %q", ErrNotMonitor, name)
	}
	return name, nil
}
