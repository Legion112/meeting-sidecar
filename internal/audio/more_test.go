package audio_test

import (
	"context"
	"testing"
	"time"

	"github.com/Legion112/meeting-sidecar/internal/audio"
)

func TestOpenMonitorNilStartStop(t *testing.T) {
	src, err := audio.OpenMonitor(func(string, int, func([]int16)) (audio.RecordControl, error) {
		return audio.RecordControl{}, nil
	}, "x.monitor", 16000)
	if err != nil {
		t.Fatal(err)
	}
	if err := src.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestBufferCancelAfterClose(t *testing.T) {
	b := audio.NewStreamBuffer()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		_, err := b.Read(ctx, make([]int16, 1))
		done <- err
	}()
	time.Sleep(10 * time.Millisecond)
	cancel()
	_ = b.Close()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("expected err")
		}
	case <-time.After(time.Second):
		t.Fatal("timeout")
	}
}

func TestBufferPushWhileWaiting(t *testing.T) {
	b := audio.NewStreamBuffer()
	done := make(chan error, 1)
	go func() {
		dst := make([]int16, 1)
		_, err := b.Read(context.Background(), dst)
		done <- err
	}()
	time.Sleep(20 * time.Millisecond)
	b.Push([]int16{42})
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout")
	}
}
