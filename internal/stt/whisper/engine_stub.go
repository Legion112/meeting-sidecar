//go:build !whisper

package whisper

import (
	"fmt"
)

// NewNativeEngine is unavailable unless built with -tags whisper and libwhisper linked.
func NewNativeEngine(modelPath, language string) (Engine, error) {
	_ = modelPath
	_ = language
	return nil, fmt.Errorf("whisper native engine requires build tag 'whisper' and libwhisper (see README)")
}
