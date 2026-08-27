package whisper

import (
	"context"
	"errors"
	"io"
	"testing"
	"time"

	wpkg "github.com/ggerganov/whisper.cpp/bindings/go/pkg/whisper"
)

type fakeModel struct {
	ctx    wpkg.Context
	ctxErr error
}

func (m fakeModel) Close() error                      { return nil }
func (m fakeModel) IsMultilingual() bool              { return true }
func (m fakeModel) Languages() []string               { return []string{"en"} }
func (m fakeModel) NewContext() (wpkg.Context, error) { return m.ctx, m.ctxErr }

type fakeContext struct {
	langErr error
	procErr error
	segs    []wpkg.Segment
	i       int
}

func (c *fakeContext) SetLanguage(string) error       { return c.langErr }
func (c *fakeContext) SetTranslate(bool)              {}
func (c *fakeContext) IsMultilingual() bool           { return true }
func (c *fakeContext) Language() string               { return "en" }
func (c *fakeContext) DetectedLanguage() string       { return "en" }
func (c *fakeContext) SetOffset(time.Duration)        {}
func (c *fakeContext) SetDuration(time.Duration)      {}
func (c *fakeContext) SetThreads(uint)                {}
func (c *fakeContext) SetSplitOnWord(bool)            {}
func (c *fakeContext) SetTokenThreshold(float32)      {}
func (c *fakeContext) SetTokenSumThreshold(float32)   {}
func (c *fakeContext) SetMaxSegmentLength(uint)       {}
func (c *fakeContext) SetTokenTimestamps(bool)        {}
func (c *fakeContext) SetMaxTokensPerSegment(uint)    {}
func (c *fakeContext) SetAudioCtx(uint)               {}
func (c *fakeContext) SetMaxContext(int)              {}
func (c *fakeContext) SetBeamSize(int)                {}
func (c *fakeContext) SetEntropyThold(float32)        {}
func (c *fakeContext) SetInitialPrompt(string)        {}
func (c *fakeContext) SetTemperature(float32)         {}
func (c *fakeContext) SetTemperatureFallback(float32) {}
func (c *fakeContext) SetVAD(bool)                    {}
func (c *fakeContext) SetVADModelPath(string)         {}
func (c *fakeContext) SetVADThreshold(float32)        {}
func (c *fakeContext) SetVADMinSpeechMs(int)          {}
func (c *fakeContext) SetVADMinSilenceMs(int)         {}
func (c *fakeContext) SetVADMaxSpeechSec(float32)     {}
func (c *fakeContext) SetVADSpeechPadMs(int)          {}
func (c *fakeContext) SetVADSamplesOverlap(float32)   {}
func (c *fakeContext) Process([]float32, wpkg.EncoderBeginCallback, wpkg.SegmentCallback, wpkg.ProgressCallback) error {
	return c.procErr
}
func (c *fakeContext) NextSegment() (wpkg.Segment, error) {
	if c.i >= len(c.segs) {
		return wpkg.Segment{}, io.EOF
	}
	s := c.segs[c.i]
	c.i++
	return s, nil
}
func (c *fakeContext) IsBEG(wpkg.Token) bool          { return false }
func (c *fakeContext) IsSOT(wpkg.Token) bool          { return false }
func (c *fakeContext) IsEOT(wpkg.Token) bool          { return false }
func (c *fakeContext) IsPREV(wpkg.Token) bool         { return false }
func (c *fakeContext) IsSOLM(wpkg.Token) bool         { return false }
func (c *fakeContext) IsNOT(wpkg.Token) bool          { return false }
func (c *fakeContext) IsLANG(wpkg.Token, string) bool { return false }
func (c *fakeContext) IsText(wpkg.Token) bool         { return false }
func (c *fakeContext) PrintTimings()                  {}
func (c *fakeContext) ResetTimings()                  {}
func (c *fakeContext) SystemInfo() string             { return "" }

func TestNativeEngineWithFakeModel(t *testing.T) {
	old := loadModel
	t.Cleanup(func() { loadModel = old })

	loadModel = func(path string) (wpkg.Model, error) {
		return fakeModel{ctx: &fakeContext{
			segs: []wpkg.Segment{{Text: " hi"}, {Text: " there"}},
		}}, nil
	}
	eng, err := NewNativeEngine("/fake.bin", "en")
	if err != nil {
		t.Fatal(err)
	}
	text, err := eng.Transcribe(context.Background(), []float32{0.1})
	if err != nil || text != "hi there" {
		t.Fatalf("%q %v", text, err)
	}
	if err := eng.Close(); err != nil {
		t.Fatal(err)
	}

	// language auto skips SetLanguage
	loadModel = func(string) (wpkg.Model, error) {
		return fakeModel{ctx: &fakeContext{segs: []wpkg.Segment{{Text: "ok"}}}}, nil
	}
	eng2, err := NewNativeEngine("/fake.bin", "auto")
	if err != nil {
		t.Fatal(err)
	}
	text, err = eng2.Transcribe(context.Background(), []float32{0.1})
	if err != nil || text != "ok" {
		t.Fatalf("%q %v", text, err)
	}

	// empty language also skips
	eng3 := &NativeEngine{model: fakeModel{ctx: &fakeContext{segs: nil}}, lang: ""}
	text, err = eng3.Transcribe(context.Background(), nil)
	if err != nil || text != "" {
		t.Fatalf("%q %v", text, err)
	}

	// NewContext error
	eng4 := &NativeEngine{model: fakeModel{ctxErr: errors.New("ctx")}}
	if _, err := eng4.Transcribe(context.Background(), nil); err == nil {
		t.Fatal("want ctx err")
	}

	// Process error
	eng5 := &NativeEngine{model: fakeModel{ctx: &fakeContext{procErr: errors.New("proc")}}}
	if _, err := eng5.Transcribe(context.Background(), nil); err == nil {
		t.Fatal("want proc err")
	}

	// cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := eng.Transcribe(ctx, nil); err == nil {
		t.Fatal("want canceled")
	}

	// nil engine / nil model
	var nilE *NativeEngine
	if _, err := nilE.Transcribe(context.Background(), nil); err == nil {
		t.Fatal("nil eng")
	}
	if err := nilE.Close(); err != nil {
		t.Fatal(err)
	}
	empty := &NativeEngine{}
	if _, err := empty.Transcribe(context.Background(), nil); err == nil {
		t.Fatal("nil model")
	}
	if err := empty.Close(); err != nil {
		t.Fatal(err)
	}

	// SetLanguage error is ignored
	eng6 := &NativeEngine{model: fakeModel{ctx: &fakeContext{
		langErr: errors.New("lang"),
		segs:    []wpkg.Segment{{Text: "x"}},
	}}, lang: "en"}
	text, err = eng6.Transcribe(context.Background(), nil)
	if err != nil || text != "x" {
		t.Fatalf("%q %v", text, err)
	}

	if _, err := NewNativeEngine("", "en"); err == nil {
		t.Fatal("empty path")
	}
}
