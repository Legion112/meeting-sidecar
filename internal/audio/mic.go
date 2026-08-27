package audio

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
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

// MicMixer wraps playback capture and optionally mixes a Pulse input source (microphone).
type MicMixer struct {
	base       PCMSource
	factory    RecordFactory
	micSource  string
	sampleRate int
	logger     *slog.Logger

	mu      sync.Mutex
	enabled bool
	mic     *inputStream
}

type inputStream struct {
	ctrl RecordControl
	ch   chan []int16
}

// NewMicMixer wraps base playback capture. micSource is a Pulse source id (not a .monitor).
func NewMicMixer(base PCMSource, factory RecordFactory, micSource string, sampleRate int, logger *slog.Logger) (*MicMixer, error) {
	if base == nil {
		return nil, fmt.Errorf("mic mixer: base source is nil")
	}
	if factory == nil {
		return nil, fmt.Errorf("mic mixer: record factory is nil")
	}
	if micSource == "" {
		return nil, fmt.Errorf("mic mixer: source is empty")
	}
	if strings.HasSuffix(micSource, ".monitor") {
		return nil, fmt.Errorf("microphone source cannot be a playback monitor: %q", micSource)
	}
	if sampleRate <= 0 {
		return nil, fmt.Errorf("mic mixer: sample rate must be positive")
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &MicMixer{
		base:       base,
		factory:    factory,
		micSource:  micSource,
		sampleRate: sampleRate,
		logger:     logger,
	}, nil
}

// MicEnabled reports whether microphone capture is active.
func (m *MicMixer) MicEnabled() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.enabled
}

// SetMicEnabled attaches or detaches the microphone stream.
func (m *MicMixer) SetMicEnabled(on bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if on == m.enabled {
		return nil
	}
	if on {
		if err := m.attachLocked(); err != nil {
			return err
		}
		m.enabled = true
		return nil
	}
	m.detachLocked()
	m.enabled = false
	return nil
}

func (m *MicMixer) attachLocked() error {
	if m.mic != nil {
		return nil
	}
	ch := make(chan []int16, 8)
	ctrl, err := m.factory(m.micSource, m.sampleRate, func(samples []int16) {
		cp := make([]int16, len(samples))
		copy(cp, samples)
		select {
		case ch <- cp:
		default:
			select {
			case <-ch:
			default:
			}
			ch <- cp
		}
	})
	if err != nil {
		return fmt.Errorf("open microphone %q: %w", m.micSource, err)
	}
	if ctrl.Start != nil {
		ctrl.Start()
	}
	m.mic = &inputStream{ctrl: ctrl, ch: ch}
	m.logger.Info("attached microphone", "source", m.micSource)
	return nil
}

func (m *MicMixer) detachLocked() {
	if m.mic == nil {
		return
	}
	if m.mic.ctrl.Stop != nil {
		m.mic.ctrl.Stop()
	}
	close(m.mic.ch)
	m.mic = nil
	m.logger.Info("detached microphone")
}

// Read implements PCMSource.
func (m *MicMixer) Read(ctx context.Context, dst []int16) (int, error) {
	n, err := m.base.Read(ctx, dst)
	if err != nil || n == 0 {
		return n, err
	}
	m.mu.Lock()
	mic := m.mic
	m.mu.Unlock()
	if mic == nil {
		return n, err
	}
	var chunk []int16
	select {
	case chunk = <-mic.ch:
	default:
	}
	if len(chunk) > 0 {
		mixAdd(dst[:n], chunk)
	}
	return n, err
}

// Close stops microphone capture and closes the base source.
func (m *MicMixer) Close() error {
	m.mu.Lock()
	m.detachLocked()
	m.enabled = false
	m.mu.Unlock()
	return m.base.Close()
}
