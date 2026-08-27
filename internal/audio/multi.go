package audio

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

const mixFrameSamples = 160 // 10 ms at 16 kHz

// MultiMonitorSource captures and mixes multiple playback monitors into one PCM stream.
type MultiMonitorSource struct {
	factory    RecordFactory
	sampleRate int
	lister     SinkLister
	interval   time.Duration
	logger     *slog.Logger

	mu      sync.Mutex
	buf     *StreamBuffer
	streams map[string]*monitorStream
	cancel  context.CancelFunc
	done    chan struct{}
}

type monitorStream struct {
	ctrl RecordControl
	ch   chan []int16
}

// OpenMultiMonitors opens one or more monitors, mixes them, and rescans for hotplug.
func OpenMultiMonitors(factory RecordFactory, monitors []string, sampleRate int, lister SinkLister, interval time.Duration, logger *slog.Logger) (*MultiMonitorSource, error) {
	if factory == nil {
		return nil, fmt.Errorf("record factory is nil")
	}
	if sampleRate <= 0 {
		return nil, fmt.Errorf("sample rate must be positive")
	}
	if lister == nil {
		return nil, fmt.Errorf("sink lister is nil")
	}
	if interval <= 0 {
		interval = 2 * time.Second
	}
	if logger == nil {
		logger = slog.Default()
	}
	m := &MultiMonitorSource{
		factory:    factory,
		sampleRate: sampleRate,
		lister:     lister,
		interval:   interval,
		logger:     logger,
		buf:        NewStreamBuffer(),
		streams:    make(map[string]*monitorStream),
		done:       make(chan struct{}),
	}
	for _, mon := range monitors {
		if err := m.attachLocked(mon); err != nil {
			m.mu.Lock()
			for name := range m.streams {
				m.detachLocked(name)
			}
			m.mu.Unlock()
			return nil, err
		}
	}
	if len(m.streams) == 0 {
		return nil, fmt.Errorf("no monitors opened")
	}
	ctx, cancel := context.WithCancel(context.Background())
	m.cancel = cancel
	go m.runMixer(ctx)
	go m.runRescan(ctx)
	return m, nil
}

func (m *MultiMonitorSource) attachLocked(monitor string) error {
	if _, ok := m.streams[monitor]; ok {
		return nil
	}
	if len(monitor) < 8 || monitor[len(monitor)-8:] != ".monitor" {
		return fmt.Errorf("%w: %q", ErrNotMonitor, monitor)
	}
	ch := make(chan []int16, 8)
	ctrl, err := m.factory(monitor, m.sampleRate, func(samples []int16) {
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
		return fmt.Errorf("open pulse monitor %q: %w", monitor, err)
	}
	if ctrl.Start != nil {
		ctrl.Start()
	}
	m.streams[monitor] = &monitorStream{ctrl: ctrl, ch: ch}
	m.logger.Info("attached playback monitor", "monitor", monitor)
	return nil
}

func (m *MultiMonitorSource) detachLocked(monitor string) {
	s, ok := m.streams[monitor]
	if !ok {
		return
	}
	delete(m.streams, monitor)
	if s.ctrl.Stop != nil {
		s.ctrl.Stop()
	}
	close(s.ch)
	m.logger.Info("detached playback monitor", "monitor", monitor)
}

func (m *MultiMonitorSource) runMixer(ctx context.Context) {
	defer close(m.done)
	tick := time.NewTicker(time.Duration(mixFrameSamples) * time.Second / time.Duration(m.sampleRate))
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			m.mixFrame()
		}
	}
}

func (m *MultiMonitorSource) mixFrame() {
	m.mu.Lock()
	streams := make([]*monitorStream, 0, len(m.streams))
	for _, s := range m.streams {
		streams = append(streams, s)
	}
	m.mu.Unlock()

	var chunks [][]int16
	maxLen := 0
	for _, s := range streams {
		var c []int16
		select {
		case c = <-s.ch:
		default:
		}
		if len(c) > maxLen {
			maxLen = len(c)
		}
		chunks = append(chunks, c)
	}
	if maxLen == 0 {
		return
	}
	out := make([]int16, maxLen)
	for _, c := range chunks {
		mixAdd(out, c)
	}
	m.buf.Push(out)
}

func mixAdd(dst []int16, src []int16) {
	n := len(src)
	if len(dst) < n {
		n = len(dst)
	}
	for i := 0; i < n; i++ {
		v := int32(dst[i]) + int32(src[i])
		if v > 32767 {
			v = 32767
		} else if v < -32768 {
			v = -32768
		}
		dst[i] = int16(v)
	}
}

func (m *MultiMonitorSource) runRescan(ctx context.Context) {
	tick := time.NewTicker(m.interval)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			m.rescan()
		}
	}
}

func (m *MultiMonitorSource) rescan() {
	devs, err := m.lister.ListPlaybackMonitors()
	if err != nil {
		m.logger.Warn("monitor rescan failed", "err", err)
		return
	}
	want := make(map[string]struct{}, len(devs))
	for _, d := range devs {
		if d.MonitorName != "" {
			want[d.MonitorName] = struct{}{}
		}
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	for name := range m.streams {
		if _, ok := want[name]; !ok {
			m.detachLocked(name)
		}
	}
	for name := range want {
		if err := m.attachLocked(name); err != nil {
			m.logger.Warn("attach monitor failed", "monitor", name, "err", err)
		}
	}
}

// ActiveMonitors returns the currently open monitor names.
func (m *MultiMonitorSource) ActiveMonitors() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]string, 0, len(m.streams))
	for name := range m.streams {
		out = append(out, name)
	}
	return out
}

// Read implements PCMSource.
func (m *MultiMonitorSource) Read(ctx context.Context, dst []int16) (int, error) {
	return m.buf.Read(ctx, dst)
}

// Close stops capture and rescans.
func (m *MultiMonitorSource) Close() error {
	if m.cancel != nil {
		m.cancel()
	}
	<-m.done
	m.mu.Lock()
	for name := range m.streams {
		m.detachLocked(name)
	}
	m.mu.Unlock()
	return m.buf.Close()
}
