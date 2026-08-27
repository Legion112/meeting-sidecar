package audio_test

import (
	"errors"
	"testing"

	"github.com/Legion112/meeting-sidecar/internal/audio"
)

type fakeSourceQuerier struct {
	source string
	err    error
}

func (f fakeSourceQuerier) DefaultSource() (string, error) { return f.source, f.err }

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

func TestOpenInput(t *testing.T) {
	_, err := audio.OpenInput(nil, "mic", 16000)
	if err == nil {
		t.Fatal("nil factory")
	}
	_, err = audio.OpenInput(func(string, int, func([]int16)) (audio.RecordControl, error) {
		return audio.RecordControl{}, nil
	}, "", 16000)
	if err == nil {
		t.Fatal("empty source")
	}
	_, err = audio.OpenInput(func(string, int, func([]int16)) (audio.RecordControl, error) {
		return audio.RecordControl{}, nil
	}, "out.monitor", 16000)
	if err == nil {
		t.Fatal("monitor source")
	}
	var opened bool
	src, err := audio.OpenInput(func(source string, sampleRate int, onSamples func([]int16)) (audio.RecordControl, error) {
		if source != "mic1" || sampleRate != 16000 {
			t.Fatalf("source=%q rate=%d", source, sampleRate)
		}
		opened = true
		return audio.RecordControl{Start: func() {}}, nil
	}, "mic1", 16000)
	if err != nil || !opened {
		t.Fatalf("open: %v opened=%v", err, opened)
	}
	if err := src.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestOpenInputFactoryError(t *testing.T) {
	_, err := audio.OpenInput(func(string, int, func([]int16)) (audio.RecordControl, error) {
		return audio.RecordControl{}, errors.New("pulse")
	}, "mic", 16000)
	if err == nil {
		t.Fatal("expected factory error")
	}
}
