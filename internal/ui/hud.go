package ui

// Suggestion is one HUD update.
type Suggestion struct {
	Question string
	Answer   string
}

// CaptionSource identifies which audio pipeline produced a transcript line.
type CaptionSource int

const (
	CaptionPlayback CaptionSource = iota
	CaptionMicrophone
)

// CaptionLine is one transcript utterance with its source.
type CaptionLine struct {
	Source CaptionSource
	Text   string
}

// HUD displays private suggestions to the user.
type HUD interface {
	SetStatus(status string)
	// AppendCaption adds a speech-to-text utterance to the rolling transcript.
	AppendCaption(source CaptionSource, text string)
	ShowSuggestion(s Suggestion)
	// BindMicCapture wires the microphone checkbox. onChange is called when toggled.
	BindMicCapture(initial bool, onChange func(enabled bool))
	// PushAudio feeds captured PCM for live waveform display (may be called from any goroutine).
	PushAudio(samples []int16)
	Hide()
	// Run blocks until the UI exits (or returns immediately for headless fakes).
	Run() error
	Close() error
}
