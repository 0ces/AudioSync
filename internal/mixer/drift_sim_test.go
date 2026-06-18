package mixer

import (
	"math"
	"testing"
)

// TestDriftKeepsBufferBounded simulates a sender whose clock runs faster than
// the receiver's. Without compensation the jitter buffer would grow without
// bound (latency creep) until it overflows. The PI controller must hold the
// fill near its target. This is the closed-loop version of the plan's
// multi-hour soak check, compressed into a fast deterministic simulation.
func TestDriftKeepsBufferBounded(t *testing.T) {
	const (
		framesPerTick = 240    // 5ms @ 48k output
		driftPPM      = 1000.0 // sender 0.1% fast — exaggerated for a quick test
		ticks         = 6000   // ~30s of simulated audio
		prefillMs     = 20
		capacityMs    = 500
	)
	m := New(fmt48, fmt48.FrameBytes(100))
	s := m.AddStream(1, fmt48, prefillMs, capacityMs)

	targetBytes := fmt48.FrameBytes(prefillMs)
	capBytes := fmt48.FrameBytes(capacityMs)
	out := make([]byte, framesPerTick*4)
	frame := pcmConst(2, 1000) // one stereo frame (2 samples)

	writeFrames := func(n int) {
		for range n {
			s.Write(frame)
		}
	}

	// Prime to target.
	writeFrames(targetBytes / 4)

	var acc float64
	var minFill, maxFill = math.MaxInt32, 0
	for tick := range ticks {
		// Producer delivers framesPerTick*(1+drift) input frames per tick.
		acc += framesPerTick * (1 + driftPPM/1e6)
		whole := int(acc)
		acc -= float64(whole)
		writeFrames(whole)

		m.PullMix(out)

		fill := s.rb.Len()
		if fill <= 0 {
			t.Fatalf("tick %d: buffer starved (fill=%d)", tick, fill)
		}
		if fill >= capBytes {
			t.Fatalf("tick %d: buffer overflowed (fill=%d cap=%d)", tick, fill, capBytes)
		}
		// Track steady-state range after warmup.
		if tick > ticks/2 {
			minFill = min(minFill, fill)
			maxFill = max(maxFill, fill)
		}
	}

	// The decisive check: post-warmup the fill must barely move. Active control
	// pins it to a tight band; an UNcompensated buffer would creep monotonically
	// with the drift (here ~1 byte/tick * 3000 ticks ≈ thousands of bytes of
	// spread). A tight range can only happen if the controller is working.
	spread := maxFill - minFill
	if spread > targetBytes/10 {
		t.Fatalf("steady-state fill spread %d bytes too large (range [%d,%d]) — drift not compensated", spread, minFill, maxFill)
	}
	t.Logf("steady-state fill range [%d,%d] (spread %d) bytes around target %d", minFill, maxFill, spread, targetBytes)
}
