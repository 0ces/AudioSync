// CoreAudio process-tap bridge for AudioSync (macOS 14.4+).
//
// Captures system audio output via AudioHardwareCreateProcessTap +
// CATapDescription (global stereo tap) feeding a private aggregate device.
// The realtime IOProc converts Float32 -> interleaved S16LE stereo and writes
// into a lock-free C ring; Go drains it from a non-realtime goroutine, so the
// CoreAudio realtime thread never crosses the cgo boundary.
#ifndef AUDIOSYNC_TAP_DARWIN_H
#define AUDIOSYNC_TAP_DARWIN_H

#include <stdint.h>

// Starts the system-audio tap. On success returns 0 and writes the tap's
// sample rate to *outSampleRate. Negative return is an OSStatus-derived error.
int audiosync_tap_start(uint32_t *outSampleRate);

// Copies up to cap bytes of captured interleaved S16LE stereo into dst.
// Returns the number of bytes copied (0 if none available). Consumer-side;
// call from a single goroutine.
int audiosync_tap_read(uint8_t *dst, int cap);

// Stops the tap and releases all CoreAudio resources. Safe to call once.
void audiosync_tap_stop(void);

#endif // AUDIOSYNC_TAP_DARWIN_H
