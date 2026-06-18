package resample

import (
	"testing"

	"github.com/eplata/audiosync/internal/ring"
)

// writeFrames pushes n stereo S16 frames where frame i = (i, -i).
func writeFrames(rb *ring.Ring, n int) {
	b := make([]byte, n*4)
	for i := range n {
		l := int16(i)
		r := int16(-i)
		b[4*i] = byte(l)
		b[4*i+1] = byte(l >> 8)
		b[4*i+2] = byte(r)
		b[4*i+3] = byte(r >> 8)
	}
	rb.Write(b)
}

func frameAt(out []byte, i int) (int16, int16) {
	return int16(uint16(out[4*i]) | uint16(out[4*i+1])<<8),
		int16(uint16(out[4*i+2]) | uint16(out[4*i+3])<<8)
}

func TestPassthroughRatio1(t *testing.T) {
	rb := ring.New(4096)
	writeFrames(rb, 300)
	rs := New(rb, 1.0)

	out := make([]byte, 200*4)
	rs.ReadOutput(out)
	for i := range 200 {
		l, r := frameAt(out, i)
		if l != int16(i) || r != int16(-i) {
			t.Fatalf("frame %d = (%d,%d), want (%d,%d)", i, l, r, i, -i)
		}
	}
}

func TestDownsampleByTwo(t *testing.T) {
	rb := ring.New(8192)
	writeFrames(rb, 600)
	rs := New(rb, 2.0)

	out := make([]byte, 200*4)
	rs.ReadOutput(out)
	// At integer ratio 2.0 the interpolation lands on exact input frames 0,2,4...
	for i := range 200 {
		l, _ := frameAt(out, i)
		if l != int16(2*i) {
			t.Fatalf("downsampled frame %d L=%d, want %d", i, l, 2*i)
		}
	}
}

func TestUpsampleInterpolates(t *testing.T) {
	rb := ring.New(4096)
	writeFrames(rb, 300)
	rs := New(rb, 0.5)

	out := make([]byte, 8*4)
	rs.ReadOutput(out)
	// ratio 0.5 -> out0=f0, out1=midpoint(f0,f1), out2=f1, ...
	l0, _ := frameAt(out, 0)
	l1, _ := frameAt(out, 1)
	l2, _ := frameAt(out, 2)
	if l0 != 0 || l2 != 1 {
		t.Fatalf("upsample integer taps: l0=%d l2=%d want 0,1", l0, l2)
	}
	if l1 != 0 { // midpoint of 0 and 1 truncates to 0
		t.Fatalf("upsample midpoint l1=%d want 0", l1)
	}
}

func TestColdStartSilence(t *testing.T) {
	rb := ring.New(64)
	rs := New(rb, 1.0)
	out := make([]byte, 16*4)
	for i := range out {
		out[i] = 0x7F
	}
	rs.ReadOutput(out)
	for i, b := range out {
		if b != 0 {
			t.Fatalf("byte %d = %d, want 0 (silence on empty ring)", i, b)
		}
	}
}

func TestReadOutputZeroAlloc(t *testing.T) {
	rb := ring.New(1 << 16)
	rs := New(rb, 1.0)
	frame := make([]byte, 240*4)
	out := make([]byte, 240*4)
	if allocs := testing.AllocsPerRun(1000, func() {
		rb.Write(frame)
		rs.ReadOutput(out)
	}); allocs != 0 {
		t.Fatalf("ReadOutput allocates %v/op, want 0", allocs)
	}
}
