package ui

import (
	"image/color"
	"sync"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
)

const waveformPoints = 256

// waveBuffer holds downsampled peak amplitudes for scrolling waveform display.
type waveBuffer struct {
	mu   sync.Mutex
	data [waveformPoints]float32
	head int
}

func (b *waveBuffer) push(samples []int16) {
	if len(samples) == 0 {
		return
	}
	step := len(samples)/8 + 1
	b.mu.Lock()
	defer b.mu.Unlock()
	for i := 0; i < len(samples); i += step {
		end := i + step
		if end > len(samples) {
			end = len(samples)
		}
		var peak float32
		for j := i; j < end; j++ {
			v := float32(samples[j]) / 32768
			if v < 0 {
				v = -v
			}
			if v > peak {
				peak = v
			}
		}
		b.data[b.head] = peak
		b.head = (b.head + 1) % waveformPoints
	}
}

func (b *waveBuffer) ampAt(x, width int) float32 {
	if width <= 1 {
		width = 1
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	idx := b.head + (x*waveformPoints)/width
	idx %= waveformPoints
	return b.data[idx]
}

func newWaveformRaster(buf *waveBuffer) *canvas.Raster {
	bg := color.NRGBA{R: 24, G: 24, B: 28, A: 255}
	grid := color.NRGBA{R: 48, G: 48, B: 56, A: 255}
	wave := color.NRGBA{R: 80, G: 200, B: 120, A: 255}
	return canvas.NewRasterWithPixels(func(x, y, w, h int) color.Color {
		if w <= 0 || h <= 0 {
			return bg
		}
		mid := h / 2
		if y == mid {
			return grid
		}
		amp := buf.ampAt(x, w)
		bar := int(amp*float32(mid-1) + 0.5)
		if bar < 0 {
			bar = 0
		}
		dy := y - mid
		if dy < 0 {
			dy = -dy
		}
		if dy <= bar {
			return wave
		}
		return bg
	})
}

func sizedWaveformRaster(buf *waveBuffer) fyne.CanvasObject {
	r := newWaveformRaster(buf)
	r.SetMinSize(fyne.NewSize(380, 72))
	return r
}
