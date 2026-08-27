package ui

// Suggestion is one HUD update.
type Suggestion struct {
	Question string
	Answer   string
}

// HUD displays private suggestions to the user.
type HUD interface {
	SetStatus(status string)
	ShowSuggestion(s Suggestion)
	Hide()
	// Run blocks until the UI exits (or returns immediately for headless fakes).
	Run() error
	Close() error
}
