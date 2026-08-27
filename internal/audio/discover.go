package audio

import (
	"fmt"
	"strings"
)

// MonitorAll selects every playback sink monitor (see ResolveMonitors).
const MonitorAll = "all"

// MonitorDevice describes one Pulse sink and its loopback monitor source.
type MonitorDevice struct {
	SinkName    string
	MonitorName string
	Description string
}

// SinkLister lists Pulse playback sinks for monitor discovery.
type SinkLister interface {
	SinkQuerier
	ListPlaybackMonitors() ([]MonitorDevice, error)
}

// ResolveMonitors maps config audio.monitor to one or more monitor source names.
func ResolveMonitors(explicit string, lister SinkLister) ([]string, error) {
	name := strings.TrimSpace(explicit)
	if strings.EqualFold(name, MonitorAll) {
		if lister == nil {
			return nil, fmt.Errorf("resolve monitors: sink lister is nil")
		}
		devs, err := lister.ListPlaybackMonitors()
		if err != nil {
			return nil, fmt.Errorf("list playback monitors: %w", err)
		}
		if len(devs) == 0 {
			return nil, fmt.Errorf("list playback monitors: none found")
		}
		out := make([]string, 0, len(devs))
		for _, d := range devs {
			mon := strings.TrimSpace(d.MonitorName)
			if mon == "" {
				continue
			}
			if !strings.HasSuffix(mon, ".monitor") {
				return nil, fmt.Errorf("%w: %q", ErrNotMonitor, mon)
			}
			out = append(out, mon)
		}
		if len(out) == 0 {
			return nil, fmt.Errorf("list playback monitors: no .monitor sources")
		}
		return out, nil
	}
	one, err := ResolveMonitor(name, lister)
	if err != nil {
		return nil, err
	}
	return []string{one}, nil
}

// FormatMonitorTable renders sink discovery rows for CLI output.
func FormatMonitorTable(devs []MonitorDevice) string {
	if len(devs) == 0 {
		return "No playback sinks found.\n"
	}
	sinkW, monW, descW := len("SINK"), len("MONITOR"), len("DESCRIPTION")
	for _, d := range devs {
		if len(d.SinkName) > sinkW {
			sinkW = len(d.SinkName)
		}
		if len(d.MonitorName) > monW {
			monW = len(d.MonitorName)
		}
		if len(d.Description) > descW {
			descW = len(d.Description)
		}
	}
	pad := func(s string, w int) string {
		if len(s) >= w {
			return s
		}
		return s + strings.Repeat(" ", w-len(s))
	}
	var b strings.Builder
	b.WriteString(pad("SINK", sinkW))
	b.WriteString("  ")
	b.WriteString(pad("MONITOR", monW))
	b.WriteString("  ")
	b.WriteString(pad("DESCRIPTION", descW))
	b.WriteByte('\n')
	for _, d := range devs {
		b.WriteString(pad(d.SinkName, sinkW))
		b.WriteString("  ")
		b.WriteString(pad(d.MonitorName, monW))
		b.WriteString("  ")
		b.WriteString(d.Description)
		b.WriteByte('\n')
	}
	return b.String()
}
