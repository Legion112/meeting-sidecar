package vad

import (
	"math"
)

// Frame is one analysis window of mono int16 PCM.
type Frame []int16

// Utterance is a contiguous speech segment (silence stripped).
type Utterance struct {
	PCM []int16
}

// Config controls energy VAD behaviour.
type Config struct {
	SampleRate       int
	FrameMs          int
	EnergyThreshold  float64
	HangoverFrames   int
	MinSpeechFrames  int
	MaxSpeechFrames  int
}

// DefaultConfig returns sensible 16 kHz / 20 ms defaults.
func DefaultConfig(sampleRate int) Config {
	if sampleRate <= 0 {
		sampleRate = 16000
	}
	return Config{
		SampleRate:      sampleRate,
		FrameMs:         20,
		EnergyThreshold: 500,
		HangoverFrames:  15, // 300 ms
		MinSpeechFrames: 5,  // 100 ms
		MaxSpeechFrames: sampleRate / 20 * 15, // ~15 s
	}
}

// Segmenter splits a PCM stream into speech utterances; silence is discarded.
type Segmenter struct {
	cfg          Config
	frameSize    int
	pending      []int16
	inSpeech     bool
	hangover     int
	speechFrames []int16
	speechCount  int
}

// NewSegmenter creates a VAD segmenter.
func NewSegmenter(cfg Config) *Segmenter {
	if cfg.FrameMs <= 0 {
		cfg.FrameMs = 20
	}
	if cfg.SampleRate <= 0 {
		cfg.SampleRate = 16000
	}
	if cfg.EnergyThreshold <= 0 {
		cfg.EnergyThreshold = 500
	}
	if cfg.HangoverFrames <= 0 {
		cfg.HangoverFrames = 15
	}
	if cfg.MinSpeechFrames <= 0 {
		cfg.MinSpeechFrames = 5
	}
	if cfg.MaxSpeechFrames <= 0 {
		cfg.MaxSpeechFrames = cfg.SampleRate / 20 * 15
	}
	fs := cfg.SampleRate * cfg.FrameMs / 1000
	if fs <= 0 {
		fs = 320
	}
	return &Segmenter{cfg: cfg, frameSize: fs}
}

// FrameSize returns samples per analysis frame.
func (s *Segmenter) FrameSize() int { return s.frameSize }

// Push appends PCM and returns any completed speech utterances.
// Silence never appears in the returned slice.
func (s *Segmenter) Push(samples []int16) []Utterance {
	if len(samples) == 0 {
		return nil
	}
	s.pending = append(s.pending, samples...)
	var out []Utterance
	for len(s.pending) >= s.frameSize {
		frame := s.pending[:s.frameSize]
		s.pending = s.pending[s.frameSize:]
		if u, ok := s.consumeFrame(frame); ok {
			out = append(out, u)
		}
	}
	return out
}

// Flush ends any in-progress speech as an utterance (if long enough).
func (s *Segmenter) Flush() []Utterance {
	var out []Utterance
	if s.inSpeech && s.speechCount >= s.cfg.MinSpeechFrames {
		pcm := append([]int16(nil), s.speechFrames...)
		out = append(out, Utterance{PCM: pcm})
	}
	s.inSpeech = false
	s.hangover = 0
	s.speechFrames = nil
	s.speechCount = 0
	s.pending = nil
	return out
}

func (s *Segmenter) consumeFrame(frame []int16) (Utterance, bool) {
	speech := frameEnergy(frame) >= s.cfg.EnergyThreshold
	if speech {
		if !s.inSpeech {
			s.inSpeech = true
			s.speechFrames = nil
			s.speechCount = 0
		}
		s.speechFrames = append(s.speechFrames, frame...)
		s.speechCount++
		s.hangover = s.cfg.HangoverFrames
		if s.speechCount >= s.cfg.MaxSpeechFrames {
			u := Utterance{PCM: append([]int16(nil), s.speechFrames...)}
			s.inSpeech = false
			s.hangover = 0
			s.speechFrames = nil
			s.speechCount = 0
			return u, true
		}
		return Utterance{}, false
	}

	// silence / noise frame — never emitted alone
	if !s.inSpeech {
		return Utterance{}, false
	}
	s.speechFrames = append(s.speechFrames, frame...)
	s.hangover--
	if s.hangover > 0 {
		return Utterance{}, false
	}
	if s.speechCount < s.cfg.MinSpeechFrames {
		s.inSpeech = false
		s.speechFrames = nil
		s.speechCount = 0
		return Utterance{}, false
	}
	u := Utterance{PCM: append([]int16(nil), s.speechFrames...)}
	s.inSpeech = false
	s.speechFrames = nil
	s.speechCount = 0
	return u, true
}

func frameEnergy(frame []int16) float64 {
	if len(frame) == 0 {
		return 0
	}
	var sum float64
	for _, s := range frame {
		v := float64(s)
		sum += v * v
	}
	return math.Sqrt(sum / float64(len(frame)))
}
