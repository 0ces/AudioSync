package drift

import "testing"

func TestZeroErrorNoCorrection(t *testing.T) {
	c := New(1000)
	if got := c.Update(1000); got != 0 {
		t.Fatalf("at target correction=%v want 0", got)
	}
}

func TestSignOfCorrection(t *testing.T) {
	// Overfull buffer -> positive correction (consume faster to drain).
	if got := New(1000).Update(1500); got <= 0 {
		t.Fatalf("overfull correction=%v want >0", got)
	}
	// Underfull buffer -> negative correction (slow down to refill).
	if got := New(1000).Update(500); got >= 0 {
		t.Fatalf("underfull correction=%v want <0", got)
	}
}

func TestCorrectionClamped(t *testing.T) {
	c := New(1000)
	// Drive far past target many times; output must saturate at ±maxCor.
	var last float64
	for range 100 {
		last = c.Update(100000)
	}
	if last > c.maxCor+1e-9 || last < c.maxCor-1e-9 {
		t.Fatalf("saturated correction=%v want %v", last, c.maxCor)
	}
}

func TestDisabledWhenNoTarget(t *testing.T) {
	c := New(0)
	if got := c.Update(5000); got != 0 {
		t.Fatalf("target 0 correction=%v want 0", got)
	}
}

// TestConvergence: a constant overfill should settle to a steady positive
// correction (integral builds, output stays within clamp).
func TestConvergence(t *testing.T) {
	c := New(2000)
	var v float64
	for range 50 {
		v = c.Update(2200) // 10% over target
	}
	if v <= 0 || v > c.maxCor+1e-9 {
		t.Fatalf("converged correction=%v, want (0, %v]", v, c.maxCor)
	}
}
