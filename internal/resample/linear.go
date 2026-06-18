// Package resample provides sample-rate conversion for AudioSync streams.
//
// Linear is a fractional linear resampler over interleaved S16LE stereo. It
// pulls input frames on demand from a ring buffer and emits output frames at a
// dynamically adjustable ratio, so it serves double duty: fixed conversion
// (e.g. 44.1k -> 48k via the base ratio) and clock-drift trimming (the drift
// controller nudges the ratio around the base each callback).
//
// It runs on the playback realtime thread and is allocation-free. A single
// goroutine drives SetRatio + ReadOutput; the ring's producer is a different
// goroutine (SPSC).
package resample

import "github.com/eplata/audiosync/internal/ring"

// Linear is a stereo S16LE linear resampler bound to one input ring.
type Linear struct {
	rb    *ring.Ring
	ratio float64 // input frames consumed per output frame

	pos                    float64 // fractional position in [0,1) between cur and nxt
	curL, curR, nxtL, nxtR int16
	haveCur                bool
	fb                     [4]byte // one-frame read scratch
}

// New binds a resampler to ring rb with an initial ratio (srcRate/dstRate).
func New(rb *ring.Ring, ratio float64) *Linear {
	return &Linear{rb: rb, ratio: ratio}
}

// SetRatio updates the conversion ratio (input frames per output frame). Call
// from the same goroutine that calls ReadOutput.
func (l *Linear) SetRatio(r float64) {
	if r > 0 {
		l.ratio = r
	}
}

// Reset drops interpolation state so playout re-primes from the ring (used when
// a stream re-primes after a long starvation).
func (l *Linear) Reset() { l.haveCur = false; l.pos = 0 }

// pull reads one input frame from the ring. ok is false on underrun.
func (l *Linear) pull() (lft, rgt int16, ok bool) {
	if l.rb.Len() < 4 {
		return 0, 0, false
	}
	l.rb.Read(l.fb[:])
	return int16(uint16(l.fb[0]) | uint16(l.fb[1])<<8),
		int16(uint16(l.fb[2]) | uint16(l.fb[3])<<8), true
}

func lerp(a, b int16, t float64) int16 {
	return int16(float64(a) + (float64(b)-float64(a))*t)
}

// ReadOutput fills out (interleaved S16LE stereo) with resampled audio. On a
// cold-start underrun it emits silence; mid-stream underruns hold the last
// sample (repeat-frame concealment) so the output never blocks.
func (l *Linear) ReadOutput(out []byte) {
	frames := len(out) / 4
	for f := range frames {
		if !l.haveCur {
			cl, cr, ok := l.pull()
			if !ok {
				// Nothing buffered yet: fill the rest with silence.
				for i := 4 * f; i < len(out); i++ {
					out[i] = 0
				}
				return
			}
			l.curL, l.curR = cl, cr
			if nl, nr, ok2 := l.pull(); ok2 {
				l.nxtL, l.nxtR = nl, nr
			} else {
				l.nxtL, l.nxtR = cl, cr
			}
			l.haveCur = true
			l.pos = 0
		}

		oL := lerp(l.curL, l.nxtL, l.pos)
		oR := lerp(l.curR, l.nxtR, l.pos)
		o := 4 * f
		out[o] = byte(oL)
		out[o+1] = byte(oL >> 8)
		out[o+2] = byte(oR)
		out[o+3] = byte(oR >> 8)

		l.pos += l.ratio
		for l.pos >= 1.0 {
			l.pos -= 1.0
			l.curL, l.curR = l.nxtL, l.nxtR
			if nl, nr, ok := l.pull(); ok {
				l.nxtL, l.nxtR = nl, nr
			} else {
				// Hold last sample (conceal) without advancing past it.
				l.nxtL, l.nxtR = l.curL, l.curR
			}
		}
	}
}
