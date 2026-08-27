package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	fyneapp "fyne.io/fyne/v2/app"
	"github.com/jfreymuth/pulse"

	"github.com/Legion112/meeting-sidecar/internal/app"
	"github.com/Legion112/meeting-sidecar/internal/audio"
	"github.com/Legion112/meeting-sidecar/internal/config"
	"github.com/Legion112/meeting-sidecar/internal/ui"
)

func main() {
	configPath := flag.String("config", "config.yaml", "path to YAML config")
	listMonitors := flag.Bool("list-monitors", false, "list Pulse playback sinks and exit")
	debug := flag.Bool("debug", false, "log STT transcripts and gate decisions to stderr")
	flag.Parse()

	if *debug {
		slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug})))
	}

	client, err := pulse.NewClient()
	if err != nil {
		fmt.Fprintf(os.Stderr, "pulse: %v\n", err)
		os.Exit(1)
	}
	defer client.Close()

	pq := pulseQuerier{client: client}
	if *listMonitors {
		devs, err := pq.ListPlaybackMonitors()
		if err != nil {
			fmt.Fprintf(os.Stderr, "list monitors: %v\n", err)
			os.Exit(1)
		}
		fmt.Print(audio.FormatMonitorTable(devs))
		return
	}

	cfg, err := loadConfig(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "config: %v\n", err)
		os.Exit(1)
	}

	modelPath, err := cfg.ResolveSTTModelPath()
	if err != nil {
		fmt.Fprintf(os.Stderr, "stt model: %v\n", err)
		os.Exit(1)
	}
	slog.Info("stt", "model", modelPath, "language", cfg.STTLanguage(), "threads", cfg.STT.Threads)

	monitors, err := audio.ResolveMonitors(cfg.Audio.Monitor, pq)
	if err != nil {
		fmt.Fprintf(os.Stderr, "audio: %v\n", err)
		os.Exit(1)
	}

	factory := pulseFactory(client)
	var src audio.PCMSource
	if strings.EqualFold(strings.TrimSpace(cfg.Audio.Monitor), audio.MonitorAll) {
		src, err = audio.OpenMultiMonitors(factory, monitors, cfg.Audio.SampleRate, pq, 2*time.Second, slog.Default())
	} else {
		src, err = audio.OpenMonitor(factory, monitors[0], cfg.Audio.SampleRate)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "audio open: %v\n", err)
		os.Exit(1)
	}
	defer src.Close()

	ctx := context.Background()
	tr, err := app.NewWhisperTranscriber(ctx, cfg, nil, nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "stt: %v\n", err)
		os.Exit(1)
	}
	defer tr.Close()

	gate, err := app.NewGate(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "detect: %v\n", err)
		os.Exit(1)
	}
	completer, err := app.NewCompleter(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "llm: %v\n", err)
		os.Exit(1)
	}

	fyneApp := fyneapp.NewWithID("com.legion112.meeting-sidecar")
	hud, err := ui.NewFyneHUD(ui.FyneOptions{
		Display:     cfg.UI.Display,
		AlwaysOnTop: cfg.UI.AlwaysOnTop,
		App:         fyneApp,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "ui: %v\n", err)
		os.Exit(1)
	}

	if err := app.Run(ctx, app.Deps{
		Config:      cfg,
		Source:      src,
		Transcriber: tr,
		Gate:        gate,
		Completer:   completer,
		HUD:         hud,
		Logger:      slog.Default(),
	}); err != nil {
		fmt.Fprintf(os.Stderr, "run: %v\n", err)
		os.Exit(1)
	}
}

func loadConfig(path string) (config.Config, error) {
	cfg, err := config.Load(path)
	if err == nil {
		return cfg, nil
	}
	if errors.Is(err, os.ErrNotExist) || strings.Contains(err.Error(), "no such file") {
		slog.Warn("config not found; using defaults", "path", path)
		return config.Default(), nil
	}
	return config.Config{}, err
}

type pulseQuerier struct {
	client *pulse.Client
}

func (q pulseQuerier) DefaultSink() (string, error) {
	sink, err := q.client.DefaultSink()
	if err != nil {
		return "", err
	}
	return sink.ID(), nil
}

func (q pulseQuerier) ListPlaybackMonitors() ([]audio.MonitorDevice, error) {
	sinks, err := q.client.ListSinks()
	if err != nil {
		return nil, err
	}
	out := make([]audio.MonitorDevice, 0, len(sinks))
	for _, s := range sinks {
		id := s.ID()
		out = append(out, audio.MonitorDevice{
			SinkName:    id,
			MonitorName: id + ".monitor",
			Description: s.Name(),
		})
	}
	return out, nil
}

func pulseFactory(client *pulse.Client) audio.RecordFactory {
	return func(monitor string, sampleRate int, onSamples func([]int16)) (audio.RecordControl, error) {
		src, err := client.SourceByID(monitor)
		if err != nil {
			return audio.RecordControl{}, fmt.Errorf("lookup monitor %q: %w", monitor, err)
		}
		stream, err := client.NewRecord(
			pulse.Int16Writer(func(buf []int16) (int, error) {
				cp := make([]int16, len(buf))
				copy(cp, buf)
				onSamples(cp)
				return len(buf), nil
			}),
			pulse.RecordSampleRate(sampleRate),
			pulse.RecordMono,
			pulse.RecordSource(src),
			pulse.RecordMediaName("meeting-sidecar"),
		)
		if err != nil {
			return audio.RecordControl{}, err
		}
		return audio.RecordControl{Start: stream.Start, Stop: stream.Stop}, nil
	}
}
