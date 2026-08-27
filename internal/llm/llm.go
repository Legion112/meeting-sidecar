package llm

import "context"

// Completer produces a short suggested answer for a meeting question.
type Completer interface {
	Complete(ctx context.Context, systemPrompt, question string) (string, error)
}
