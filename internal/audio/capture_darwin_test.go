//go:build darwin

package audio

import (
	"os"
	"sync/atomic"
	"testing"
	"time"
)

// TestSystemCaptureHW exercises the real CoreAudio process tap. It requires a
// macOS 14.4+ machine and audio-recording permission, so it is gated behind
// AUDIOSYNC_HW_TEST=1 and skipped in normal runs/CI.
//
// It asserts that frames are delivered (the tap delivers buffers even when the
// system is silent). To verify *content*, play audio while it runs and watch
// the reported non-zero byte count grow.
func TestSystemCaptureHW(t *testing.T) {
	if os.Getenv("AUDIOSYNC_HW_TEST") != "1" {
		t.Skip("set AUDIOSYNC_HW_TEST=1 to run the CoreAudio tap hardware test")
	}
	backend, err := NewSystemCapture(48000, 5)
	if err != nil {
		t.Fatalf("NewSystemCapture: %v", err)
	}
	defer backend.Close()

	var bytesSeen, framesSeen, nonZero int64
	if err := backend.Start(func(f Frames) {
		atomic.AddInt64(&bytesSeen, int64(len(f.Data)))
		atomic.AddInt64(&framesSeen, 1)
		for _, b := range f.Data {
			if b != 0 {
				atomic.AddInt64(&nonZero, 1)
				break
			}
		}
	}); err != nil {
		t.Fatalf("Start: %v", err)
	}

	time.Sleep(800 * time.Millisecond)
	if err := backend.Stop(); err != nil {
		t.Fatalf("Stop: %v", err)
	}

	f := backend.Format()
	t.Logf("tap format: %d Hz, %d ch; frames=%d bytes=%d nonZeroFrames=%d",
		f.SampleRate, f.Channels, framesSeen, bytesSeen, nonZero)
	if framesSeen == 0 || bytesSeen == 0 {
		t.Fatalf("no frames delivered — tap not capturing (permission denied or API unavailable?)")
	}
}
