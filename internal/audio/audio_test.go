package audio_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Legion112/meeting-sidecar/internal/audio"
)

type fakeQuerier struct {
	sink string
	err  error
}

func (f fakeQuerier) DefaultSink() (string, error) { return f.sink, f.err }

func TestResolveMonitor(t *testing.T) {
	name, err := audio.ResolveMonitor("out.monitor", nil)
	if err != nil || name != "out.monitor" {
		t.Fatalf("got %q %v", name, err)
	}
	_, err = audio.ResolveMonitor("mic", nil)
	if !errors.Is(err, audio.ErrNotMonitor) {
		t.Fatalf("err=%v", err)
	}
	_, err = audio.ResolveMonitor("", nil)
	if err == nil {
		t.Fatal("expected nil querier error")
	}
	_, err = audio.ResolveMonitor("", fakeQuerier{err: errors.New("boom")})
	if err == nil {
		t.Fatal("expected querier error")
	}
	_, err = audio.ResolveMonitor("", fakeQuerier{sink: "  "})
	if err == nil {
		t.Fatal("expected empty sink")
	}
	name, err = audio.ResolveMonitor("", fakeQuerier{sink: "speakers"})
	if err != nil || name != "speakers.monitor" {
		t.Fatalf("got %q %v", name, err)
	}
}

func TestStreamBuffer(t *testing.T) {
	b := audio.NewStreamBuffer()
	n, err := b.Read(context.Background(), nil)
	if n != 0 || err != nil {
		t.Fatalf("empty dst: %d %v", n, err)
	}
	b.Push(nil)
	b.Push([]int16{1, 2, 3})
	dst := make([]int16, 2)
	n, err = b.Read(context.Background(), dst)
	if err != nil || n != 2 || dst[0] != 1 {
		t.Fatalf("read: %d %v %v", n, err, dst)
	}
	n, err = b.Read(context.Background(), dst)
	if err != nil || n != 1 || dst[0] != 3 {
		t.Fatalf("read rest: %d %v", n, err)
	}
	if err := b.Close(); err != nil {
		t.Fatal(err)
	}
	if err := b.Close(); err != nil {
		t.Fatal(err)
	}
	b.Push([]int16{9}) // ignored
	_, err = b.Read(context.Background(), dst)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("closed read: %v", err)
	}
}

func TestStreamBufferFailAndCancel(t *testing.T) {
	b := audio.NewStreamBuffer()
	boom := errors.New("boom")
	b.Fail(boom)
	_, err := b.Read(context.Background(), make([]int16, 1))
	if !errors.Is(err, boom) {
		t.Fatalf("fail: %v", err)
	}
	b2 := audio.NewStreamBuffer()
	b2.Fail(boom)
	b2.Fail(boom) // ignored after? actually Fail after fail still sets - closed check only
	_ = b2.Close()
	b2.Fail(boom) // closed

	b3 := audio.NewStreamBuffer()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = b3.Read(ctx, make([]int16, 1))
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("cancel: %v", err)
	}

	b4 := audio.NewStreamBuffer()
	ctx, cancel = context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	_, err = b4.Read(ctx, make([]int16, 1))
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("timeout: %v", err)
	}
}

func TestOpenMonitor(t *testing.T) {
	_, err := audio.OpenMonitor(nil, "x.monitor", 16000)
	if err == nil {
		t.Fatal("nil factory")
	}
	_, err = audio.OpenMonitor(func(string, int, func([]int16)) (audio.RecordControl, error) {
		return audio.RecordControl{}, nil
	}, "x.monitor", 0)
	if err == nil {
		t.Fatal("bad rate")
	}
	_, err = audio.OpenMonitor(func(string, int, func([]int16)) (audio.RecordControl, error) {
		return audio.RecordControl{}, nil
	}, "mic", 16000)
	if !errors.Is(err, audio.ErrNotMonitor) {
		t.Fatalf("err=%v", err)
	}
	_, err = audio.OpenMonitor(func(string, int, func([]int16)) (audio.RecordControl, error) {
		return audio.RecordControl{}, errors.New("nope")
	}, "x.monitor", 16000)
	if err == nil {
		t.Fatal("factory error")
	}

	var pushed bool
	src, err := audio.OpenMonitor(func(monitor string, rate int, onSamples func([]int16)) (audio.RecordControl, error) {
		if monitor != "a.monitor" || rate != 16000 {
			t.Fatalf("args %s %d", monitor, rate)
		}
		return audio.RecordControl{
			Start: func() { onSamples([]int16{7, 8}); pushed = true },
			Stop:  func() {},
		}, nil
	}, "a.monitor", 16000)
	if err != nil {
		t.Fatal(err)
	}
	if !pushed {
		t.Fatal("expected start push")
	}
	dst := make([]int16, 2)
	n, err := src.Read(context.Background(), dst)
	if err != nil || n != 2 || dst[0] != 7 {
		t.Fatalf("read %d %v %v", n, err, dst)
	}
	if err := src.Close(); err != nil {
		t.Fatal(err)
	}
}
