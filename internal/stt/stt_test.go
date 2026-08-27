package stt_test

import (
	"testing"

	"github.com/Legion112/meeting-sidecar/internal/stt"
)

func TestTranscriberInterface(t *testing.T) {
	var _ stt.Transcriber
}
