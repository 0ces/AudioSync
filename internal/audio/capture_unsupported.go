//go:build !darwin && !windows && !linux

package audio

import (
	"fmt"
	"runtime"
)

// NewSystemCapture is unavailable on this platform.
func NewSystemCapture(sampleRate uint32, frameMs int) (CaptureBackend, error) {
	return nil, fmt.Errorf("system audio capture not supported on %s", runtime.GOOS)
}
