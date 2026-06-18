package audio

import (
	"math"
	"sync"
	"time"
)

// ToneCapture is a synthetic CaptureBackend that emits a continuous sine tone.
// It exists so the full network+playback pipeline can be exercised end-to-end
// on a single machine before the real OS loopback backends land. It paces frame
// delivery in real time using a ticker, mimicking how a real capture callback
// would push frames.
type ToneCapture struct {
	format  AudioFormat
	freqHz  float64
	frameMs int
	amp     float64

	mu      sync.Mutex
	stop    chan struct{}
	stopped chan struct{}
	running bool
	phase   float64
}

// NewToneCapture creates a tone source at freqHz, delivering frames of frameMs
// milliseconds. Format is forced to S16LE stereo at the given sample rate.
func NewToneCapture(sampleRate uint32, freqHz float64, frameMs int) *ToneCapture {
	return &ToneCapture{
		format:  AudioFormat{SampleRate: sampleRate, Channels: 2, Sample: FormatS16LE},
		freqHz:  freqHz,
		frameMs: frameMs,
		amp:     0.25, // -12 dBFS, comfortable test level
	}
}

func (t *ToneCapture) Format() AudioFormat { return t.format }

// Start spawns a goroutine that generates and delivers frames in real time.
func (t *ToneCapture) Start(sink FrameSink) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.running {
		return nil
	}
	t.running = true
	t.stop = make(chan struct{})
	t.stopped = make(chan struct{})

	frameBytes := t.format.FrameBytes(t.frameMs)
	samplesPerChan := frameBytes / t.format.BytesPerFrame()
	interval := time.Duration(t.frameMs) * time.Millisecond
	step := 2 * math.Pi * t.freqHz / float64(t.format.SampleRate)
	buf := make([]byte, frameBytes)

	go func() {
		defer close(t.stopped)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-t.stop:
				return
			case <-ticker.C:
				for i := range samplesPerChan {
					v := int16(t.amp * math.Sin(t.phase) * math.MaxInt16)
					t.phase += step
					if t.phase >= 2*math.Pi {
						t.phase -= 2 * math.Pi
					}
					off := i * t.format.BytesPerFrame()
					// stereo: same value on both channels (little-endian)
					buf[off] = byte(v)
					buf[off+1] = byte(v >> 8)
					buf[off+2] = byte(v)
					buf[off+3] = byte(v >> 8)
				}
				sink(Frames{Data: buf, Format: t.format, HostTS: uint64(time.Now().UnixNano())})
			}
		}
	}()
	return nil
}

// Stop halts frame delivery and waits for the goroutine to exit.
func (t *ToneCapture) Stop() error {
	t.mu.Lock()
	if !t.running {
		t.mu.Unlock()
		return nil
	}
	t.running = false
	close(t.stop)
	stopped := t.stopped
	t.mu.Unlock()
	<-stopped
	return nil
}

// Close is a no-op for the tone source.
func (t *ToneCapture) Close() error { return nil }
