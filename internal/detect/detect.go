package detect

import "context"

// Gate classifies whether transcribed speech is a question.
type Gate interface {
	IsQuestion(ctx context.Context, text string) (bool, error)
}
