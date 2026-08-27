package vad

import "testing"

func TestFrameEnergyEmpty(t *testing.T) {
	if frameEnergy(nil) != 0 {
		t.Fatal("empty")
	}
}
