//go:build darwin

package audio

/*
#cgo darwin LDFLAGS: -framework CoreAudio -framework AudioToolbox -framework Foundation -framework CoreFoundation
#include "tap_darwin.h"
*/
import "C"

import (
	"fmt"
	"sync"
	"time"
	"unsafe"
)

// systemCapture is the macOS CaptureBackend: a CoreAudio process tap delivering
// interleaved S16LE stereo. The realtime work lives in C (see tap_darwin.m);
// this Go side only polls the C ring on a non-realtime goroutine and forwards
// frames to the sink.
type systemCapture struct {
	frameMs int
	format  AudioFormat

	mu      sync.Mutex
	stop    chan struct{}
	stopped chan struct{}
	running bool
}

// NewSystemCapture creates the macOS system-audio capture backend. The tap's
// native sample rate is discovered at Start; sampleRate is advisory only.
func NewSystemCapture(sampleRate uint32, frameMs int) (CaptureBackend, error) {
	return &systemCapture{frameMs: frameMs}, nil
}

func (c *systemCapture) Format() AudioFormat { return c.format }

func (c *systemCapture) Start(sink FrameSink) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.running {
		return nil
	}

	var rate C.uint32_t
	if rc := C.audiosync_tap_start(&rate); rc != 0 {
		return fmt.Errorf("coreaudio tap start failed (code %d) — needs macOS 14.4+ and audio-recording permission", int(rc))
	}
	c.format = AudioFormat{SampleRate: uint32(rate), Channels: 2, Sample: FormatS16LE}
	c.running = true
	c.stop = make(chan struct{})
	c.stopped = make(chan struct{})

	// Poll buffer: a few frames worth, refilled each iteration.
	bufBytes := max(c.format.FrameBytes(c.frameMs), 1920)
	buf := make([]byte, bufBytes)

	go func() {
		defer close(c.stopped)
		for {
			select {
			case <-c.stop:
				return
			default:
			}
			n := int(C.audiosync_tap_read((*C.uint8_t)(unsafe.Pointer(&buf[0])), C.int(len(buf))))
			if n <= 0 {
				time.Sleep(500 * time.Microsecond)
				continue
			}
			sink(Frames{Data: buf[:n], Format: c.format, HostTS: uint64(time.Now().UnixNano())})
		}
	}()
	return nil
}

func (c *systemCapture) Stop() error {
	c.mu.Lock()
	if !c.running {
		c.mu.Unlock()
		return nil
	}
	c.running = false
	close(c.stop)
	stopped := c.stopped
	c.mu.Unlock()
	<-stopped
	C.audiosync_tap_stop()
	return nil
}

func (c *systemCapture) Close() error { return nil }
