// Package drift compensates for clock drift between a sender and the receiver.
//
// Two machines never share an audio clock exactly: a sender nominally at 48000
// Hz might effectively run at 48001 Hz, so a fixed-size jitter buffer slowly
// fills or drains until it overflows (latency creep) or starves (dropouts). The
// Controller is a PI loop on the jitter-buffer fill level: it outputs a small
// correction added to the resampler's base ratio, speeding up or slowing down
// consumption to hold the buffer near a target fill.
//
// Output is a fractional correction (e.g. +0.001 = consume 0.1% faster). The
// caller applies it as ratio = baseRatio * (1 + correction).
package drift

// Controller is a PI controller over jitter-buffer fill (in bytes).
type Controller struct {
	target float64 // desired fill in bytes
	kp, ki float64
	maxCor float64 // clamp on |correction|

	integral float64
}

// New creates a controller targeting targetBytes of buffered audio. Gains are
// deliberately gentle so corrections stay sub-audible (no pitch warble).
func New(targetBytes int) *Controller {
	return &Controller{
		target: float64(targetBytes),
		kp:     0.10,
		ki:     0.002,
		maxCor: 0.02, // ±2% headroom: covers drift plus buffer settling
	}
}

// Update consumes the current fill level and returns the ratio correction.
// Positive correction drains the buffer (consume faster); negative fills it.
func (c *Controller) Update(fillBytes int) float64 {
	if c.target <= 0 {
		return 0 // control disabled (e.g. prefill 0 in tests)
	}
	// Normalized error: >0 means too much buffered -> drain faster.
	err := (float64(fillBytes) - c.target) / c.target

	c.integral += err
	// Anti-windup: clamp the integral to what could saturate the output.
	iLimit := c.maxCor / c.ki
	c.integral = clamp(c.integral, -iLimit, iLimit)

	return clamp(c.kp*err+c.ki*c.integral, -c.maxCor, c.maxCor)
}

func clamp(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
