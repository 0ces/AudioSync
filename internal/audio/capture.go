package audio

// Frames is one buffer of interleaved PCM delivered by a capture backend.
// Data is borrowed: backends may reuse the underlying array after the sink
// returns, so a sink that needs to retain bytes must copy them (typically by
// writing into a ring buffer).
type Frames struct {
	Data   []byte
	Format AudioFormat
	HostTS uint64 // capture timestamp in nanoseconds, for drift/diagnostics
}

// FrameSink receives captured audio. On real backends it is invoked from a
// realtime/C-owned audio thread, so it MUST NOT block, allocate, log, or call
// into the Go scheduler beyond a single non-blocking ring push.
type FrameSink func(Frames)

// CaptureBackend produces system-audio frames for a sender. Concrete backends:
// WASAPI loopback (Windows), Pulse/PipeWire monitor (Linux), CoreAudio process
// tap (macOS), plus the cross-platform tone generator used for testing.
type CaptureBackend interface {
	// Start begins delivering frames to sink. Non-blocking; returns once the
	// capture thread is running.
	Start(sink FrameSink) error
	// Stop halts delivery. Safe to call once after Start.
	Stop() error
	// Format reports the actual negotiated format (valid after Start).
	Format() AudioFormat
	// Close releases backend resources.
	Close() error
}
