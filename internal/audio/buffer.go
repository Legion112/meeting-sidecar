package audio

import (
	"context"
	"sync"
)

// StreamBuffer is a thread-safe PCM queue used by Pulse and tests.
type StreamBuffer struct {
	mu     sync.Mutex
	cond   *sync.Cond
	buf    []int16
	closed bool
	err    error
}

// NewStreamBuffer creates an empty PCM buffer.
func NewStreamBuffer() *StreamBuffer {
	s := &StreamBuffer{}
	s.cond = sync.NewCond(&s.mu)
	return s
}

// Push appends samples for readers.
func (s *StreamBuffer) Push(samples []int16) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed || s.err != nil {
		return
	}
	if len(samples) == 0 {
		return
	}
	s.buf = append(s.buf, samples...)
	s.cond.Broadcast()
}

// Read copies PCM into dst, blocking until data, close, error, or ctx cancel.
func (s *StreamBuffer) Read(ctx context.Context, dst []int16) (int, error) {
	if len(dst) == 0 {
		return 0, nil
	}

	stop := context.AfterFunc(ctx, func() {
		s.cond.Broadcast()
	})
	defer stop()

	s.mu.Lock()
	defer s.mu.Unlock()

	for len(s.buf) == 0 && !s.closed && s.err == nil {
		if err := ctx.Err(); err != nil {
			return 0, err
		}
		s.cond.Wait()
	}

	if s.err != nil {
		return 0, s.err
	}
	if len(s.buf) == 0 {
		// Closed or cancelled with nothing left to read.
		if err := ctx.Err(); err != nil {
			return 0, err
		}
		return 0, context.Canceled
	}
	n := copy(dst, s.buf)
	s.buf = s.buf[n:]
	return n, nil
}

// Close marks the buffer closed and wakes readers.
func (s *StreamBuffer) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil
	}
	s.closed = true
	s.cond.Broadcast()
	return nil
}

// Fail sets a permanent error and wakes readers.
func (s *StreamBuffer) Fail(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return
	}
	s.err = err
	s.cond.Broadcast()
}
