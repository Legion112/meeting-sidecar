package audio_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/Legion112/meeting-sidecar/internal/audio"
)

type fakeLister struct {
	fakeQuerier
	devs    []audio.MonitorDevice
	listErr error
}

func (f fakeLister) ListPlaybackMonitors() ([]audio.MonitorDevice, error) {
	return f.devs, f.listErr
}

func TestResolveMonitors(t *testing.T) {
	all, err := audio.ResolveMonitors("all", fakeLister{devs: []audio.MonitorDevice{
		{SinkName: "spk", MonitorName: "spk.monitor", Description: "Speakers"},
		{SinkName: "hp", MonitorName: "hp.monitor", Description: "Headphones"},
	}})
	if err != nil || len(all) != 2 || all[0] != "spk.monitor" {
		t.Fatalf("%v %v", all, err)
	}
	_, err = audio.ResolveMonitors("all", nil)
	if err == nil {
		t.Fatal("nil lister")
	}
	_, err = audio.ResolveMonitors("all", fakeLister{listErr: errors.New("x")})
	if err == nil {
		t.Fatal("list err")
	}
	_, err = audio.ResolveMonitors("all", fakeLister{})
	if err == nil {
		t.Fatal("empty list")
	}
	_, err = audio.ResolveMonitors("all", fakeLister{devs: []audio.MonitorDevice{{MonitorName: "bad"}}})
	if !errors.Is(err, audio.ErrNotMonitor) {
		t.Fatalf("err=%v", err)
	}

	one, err := audio.ResolveMonitors("", fakeLister{fakeQuerier: fakeQuerier{sink: "spk"}})
	if err != nil || len(one) != 1 || one[0] != "spk.monitor" {
		t.Fatalf("%v %v", one, err)
	}
	one, err = audio.ResolveMonitors("x.monitor", nil)
	if err != nil || one[0] != "x.monitor" {
		t.Fatalf("%v %v", one, err)
	}
}

func TestFormatMonitorTable(t *testing.T) {
	out := audio.FormatMonitorTable(nil)
	if !strings.Contains(out, "No playback") {
		t.Fatal(out)
	}
	out = audio.FormatMonitorTable([]audio.MonitorDevice{
		{SinkName: "a", MonitorName: "a.monitor", Description: "A"},
	})
	if !strings.Contains(out, "a.monitor") {
		t.Fatal(out)
	}
}
