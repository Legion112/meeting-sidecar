package audio_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Legion112/meeting-sidecar/internal/audio"
)

type fakeSourceQuerier struct {
	source string
	err    error
}

func (f fakeSourceQuerier) DefaultSource() (string, error) { return f.source, f.err }

type pushSrc struct {
	chunks [][]int16
	i      int
}

func (s *pushSrc) Read(ctx context.Context, dst []int16) (int, error) {
	if s.i >= len(s.chunks) {
		<-ctx.Done()
		return 0, ctx.Err()
	}
	n := copy(dst, s.chunks[s.i])
	s.i++
	return n, nil
}
func (s *pushSrc) Close() error { return nil }

func TestResolveMicrophone(t *testing.T) {
	name, err := audio.ResolveMicrophone("alsa_input.usb", nil)
	if err != nil || name != "alsa_input.usb" {
		t.Fatalf("got %q %v", name, err)
	}
	_, err = audio.ResolveMicrophone("out.monitor", nil)
	if err == nil {
		t.Fatal("expected monitor rejection")
	}
	_, err = audio.ResolveMicrophone("", nil)
	if err == nil {
		t.Fatal("expected nil querier error")
	}
	name, err = audio.ResolveMicrophone("", fakeSourceQuerier{source: "default-mic"})
	if err != nil || name != "default-mic" {
		t.Fatalf("got %q %v", name, err)
	}
}

func TestMicMixerToggle(t *testing.T) {
	base := &pushSrc{chunks: [][]int16{{100, 200}, {300, 400}}}
	var opened, started, stopped int
	factory := func(source string, sampleRate int, onSamples func([]int16)) (audio.RecordControl, error) {
		if source != "mic1" {
			t.Fatalf("source %q", source)
		}
		opened++
		return audio.RecordControl{
			Start: func() { started++ },
			Stop:  func() { stopped++ },
		}, nil
	}
	m, err := audio.NewMicMixer(base, factory, "mic1", 16000, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()

	n, err := m.Read(context.Background(), make([]int16, 2))
	if err != nil || n != 2 {
		t.Fatalf("base read: %d %v", n, err)
	}
	if opened != 0 {
		t.Fatal("mic should be closed initially")
	}

	if err := m.SetMicEnabled(true); err != nil {
		t.Fatal(err)
	}
	if !m.MicEnabled() || opened != 1 || started != 1 {
		t.Fatalf("enabled: opened=%d started=%d", opened, started)
	}

	mixFactory := func(source string, sampleRate int, onSamples func([]int16)) (audio.RecordControl, error) {
		onSamples([]int16{50, 50})
		return audio.RecordControl{}, nil
	}
	m2, err := audio.NewMicMixer(&pushSrc{chunks: [][]int16{{100, 200}}}, mixFactory, "mic1", 16000, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer m2.Close()
	if err := m2.SetMicEnabled(true); err != nil {
		t.Fatal(err)
	}
	dst := make([]int16, 2)
	n, err = m2.Read(context.Background(), dst)
	if err != nil || n != 2 {
		t.Fatal(err)
	}
	if dst[0] != 150 || dst[1] != 250 {
		t.Fatalf("mixed %v", dst)
	}

	if err := m.SetMicEnabled(false); err != nil {
		t.Fatal(err)
	}
	if m.MicEnabled() || stopped != 1 {
		t.Fatalf("disabled stopped=%d", stopped)
	}
	if err := m.SetMicEnabled(false); err != nil {
		t.Fatal("idempotent disable")
	}
}

func TestMicMixerValidation(t *testing.T) {
	base := &pushSrc{}
	factory := func(string, int, func([]int16)) (audio.RecordControl, error) {
		return audio.RecordControl{}, nil
	}
	_, err := audio.NewMicMixer(nil, factory, "mic", 16000, nil)
	if err == nil {
		t.Fatal("nil base")
	}
	_, err = audio.NewMicMixer(base, nil, "mic", 16000, nil)
	if err == nil {
		t.Fatal("nil factory")
	}
	_, err = audio.NewMicMixer(base, factory, "", 16000, nil)
	if err == nil {
		t.Fatal("empty source")
	}
	_, err = audio.NewMicMixer(base, factory, "x.monitor", 16000, nil)
	if err == nil {
		t.Fatal("monitor source")
	}
}

func TestMicMixerFactoryError(t *testing.T) {
	base := &pushSrc{}
	factory := func(string, int, func([]int16)) (audio.RecordControl, error) {
		return audio.RecordControl{}, errors.New("pulse")
	}
	m, err := audio.NewMicMixer(base, factory, "mic", 16000, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := m.SetMicEnabled(true); err == nil {
		t.Fatal("expected open error")
	}
}

func TestMicMixerClose(t *testing.T) {
	m, err := audio.NewMicMixer(&pushSrc{}, func(string, int, func([]int16)) (audio.RecordControl, error) {
		return audio.RecordControl{Start: func() {}, Stop: func() {}}, nil
	}, "mic", 16000, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := m.SetMicEnabled(true); err != nil {
		t.Fatal(err)
	}
	if err := m.Close(); err != nil {
		t.Fatal(err)
	}
	if m.MicEnabled() {
		t.Fatal("mic should be off after close")
	}
}

func TestMicMixerReadWaits(t *testing.T) {
	base := &pushSrc{chunks: nil}
	m, err := audio.NewMicMixer(base, func(string, int, func([]int16)) (audio.RecordControl, error) {
		return audio.RecordControl{}, nil
	}, "mic", 16000, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	_, err = m.Read(ctx, make([]int16, 8))
	if err == nil {
		t.Fatal("expected timeout")
	}
}
