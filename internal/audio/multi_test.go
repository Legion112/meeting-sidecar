package audio_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Legion112/meeting-sidecar/internal/audio"
)

func TestOpenMultiMonitors(t *testing.T) {
	factory := func(monitor string, rate int, onSamples func([]int16)) (audio.RecordControl, error) {
		return audio.RecordControl{
			Start: func() {
				onSamples([]int16{1000, 2000})
			},
			Stop: func() {},
		}, nil
	}
	lister := fakeLister{devs: []audio.MonitorDevice{
		{MonitorName: "a.monitor"},
		{MonitorName: "b.monitor"},
	}}
	src, err := audio.OpenMultiMonitors(factory, []string{"a.monitor", "b.monitor"}, 16000, lister, 50*time.Millisecond, nil)
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(30 * time.Millisecond)
	dst := make([]int16, 160)
	n, err := src.Read(context.Background(), dst)
	if err != nil || n == 0 {
		t.Fatalf("read %d %v", n, err)
	}
	if len(src.ActiveMonitors()) != 2 {
		t.Fatal(src.ActiveMonitors())
	}
	if err := src.Close(); err != nil {
		t.Fatal(err)
	}

	_, err = audio.OpenMultiMonitors(nil, []string{"a.monitor"}, 16000, lister, time.Second, nil)
	if err == nil {
		t.Fatal("nil factory")
	}
	_, err = audio.OpenMultiMonitors(factory, []string{"a.monitor"}, 0, lister, time.Second, nil)
	if err == nil {
		t.Fatal("bad rate")
	}
	_, err = audio.OpenMultiMonitors(factory, []string{"a.monitor"}, 16000, nil, time.Second, nil)
	if err == nil {
		t.Fatal("nil lister")
	}
	badFactory := func(string, int, func([]int16)) (audio.RecordControl, error) {
		return audio.RecordControl{}, errors.New("nope")
	}
	_, err = audio.OpenMultiMonitors(badFactory, []string{"a.monitor"}, 16000, lister, time.Second, nil)
	if err == nil {
		t.Fatal("open err")
	}
	_, err = audio.OpenMultiMonitors(factory, []string{"mic"}, 16000, lister, time.Second, nil)
	if !errors.Is(err, audio.ErrNotMonitor) {
		t.Fatalf("err=%v", err)
	}
}

func TestMultiRescanAttachDetach(t *testing.T) {
	var opens, stops int
	factory := func(monitor string, rate int, onSamples func([]int16)) (audio.RecordControl, error) {
		opens++
		return audio.RecordControl{
			Start: func() { onSamples([]int16{1}) },
			Stop:  func() { stops++ },
		}, nil
	}
	lister := &dynamicLister{
		devs: []audio.MonitorDevice{{MonitorName: "a.monitor"}},
	}
	src, err := audio.OpenMultiMonitors(factory, []string{"a.monitor"}, 16000, lister, 20*time.Millisecond, nil)
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(45 * time.Millisecond)
	lister.devs = []audio.MonitorDevice{
		{MonitorName: "a.monitor"},
		{MonitorName: "b.monitor"},
	}
	time.Sleep(45 * time.Millisecond)
	if opens < 2 {
		t.Fatalf("opens=%d", opens)
	}
	lister.devs = []audio.MonitorDevice{{MonitorName: "b.monitor"}}
	time.Sleep(45 * time.Millisecond)
	if stops < 1 {
		t.Fatalf("stops=%d", stops)
	}
	_ = src.Close()
}

type dynamicLister struct {
	fakeQuerier
	devs []audio.MonitorDevice
}

func (d *dynamicLister) ListPlaybackMonitors() ([]audio.MonitorDevice, error) {
	return d.devs, nil
}

func TestMultiRescanListError(t *testing.T) {
	factory := func(string, int, func([]int16)) (audio.RecordControl, error) {
		return audio.RecordControl{Start: func() {}, Stop: func() {}}, nil
	}
	lister := fakeLister{
		devs:    []audio.MonitorDevice{{MonitorName: "a.monitor"}},
		listErr: errors.New("list fail"),
	}
	src, err := audio.OpenMultiMonitors(factory, []string{"a.monitor"}, 16000, lister, 15*time.Millisecond, nil)
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(25 * time.Millisecond)
	_ = src.Close()
}
