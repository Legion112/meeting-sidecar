package ui

// Suggestion is one HUD update.
type Suggestion struct {
	Question string
	Answer   string
}

// HUD displays private suggestions to the user.
type HUD interface {
	SetStatus(status string)
	// AppendCaption adds a speech-to-text utterance to the rolling transcript.
	AppendCaption(text string)
	ShowSuggestion(s Suggestion)
	// PushAudio feeds captured PCM for live waveform display (may be called from any goroutine).
	PushAudio(samples []int16)
	Hide()
	// Run blocks until the UI exits (or returns immediately for headless fakes).
	Run() error
	Close() error
}
