package mixer

import (
	"encoding/binary"
	"testing"

	"github.com/eplata/audiosync/internal/audio"
)

var fmt48 = audio.AudioFormat{SampleRate: 48000, Channels: 2, Sample: audio.FormatS16LE}

// pcmConst builds nSamples interleaved S16LE samples all equal to v.
func pcmConst(nSamples int, v int16) []byte {
	b := make([]byte, nSamples*2)
	for i := range nSamples {
		binary.LittleEndian.PutUint16(b[2*i:], uint16(v))
	}
	return b
}

func sampleAt(b []byte, i int) int16 {
	return int16(binary.LittleEndian.Uint16(b[2*i:]))
}

func TestPrefillGatesPlayout(t *testing.T) {
	m := New(fmt48, fmt48.FrameBytes(100))
	// prefill 10ms, capacity 100ms.
	s := m.AddStream(1, fmt48, 10, 100)

	out := make([]byte, fmt48.FrameBytes(5))
	// Write less than prefill -> still silent.
	s.Write(pcmConst(10, 1000))
	m.PullMix(out)
	if sampleAt(out, 0) != 0 {
		t.Fatalf("expected silence before prefill, got %d", sampleAt(out, 0))
	}

	// Write enough to exceed prefill (10ms@48k stereo = 960 samples).
	s.Write(pcmConst(2000, 1000))
	m.PullMix(out)
	if sampleAt(out, 0) != 1000 {
		t.Fatalf("after prefill got %d want 1000", sampleAt(out, 0))
	}
}

func TestSumsAndClips(t *testing.T) {
	m := New(fmt48, fmt48.FrameBytes(100))
	a := m.AddStream(1, fmt48, 0, 100)
	b := m.AddStream(2, fmt48, 0, 100)

	n := fmt48.FrameBytes(5) / 2 // samples
	a.Write(pcmConst(n, 20000))
	b.Write(pcmConst(n, 20000))

	out := make([]byte, fmt48.FrameBytes(5))
	m.PullMix(out)
	// 20000 + 20000 = 40000 -> clipped to 32767.
	if got := sampleAt(out, 0); got != 32767 {
		t.Fatalf("clip got %d want 32767", got)
	}
}

func TestGainAndMute(t *testing.T) {
	m := New(fmt48, fmt48.FrameBytes(100))
	s := m.AddStream(1, fmt48, 0, 100)
	n := fmt48.FrameBytes(5) / 2
	s.Write(pcmConst(n, 10000))
	s.SetGain(0.5)

	out := make([]byte, fmt48.FrameBytes(5))
	m.PullMix(out)
	if got := sampleAt(out, 0); got != 5000 {
		t.Fatalf("gain 0.5 got %d want 5000", got)
	}

	s.Write(pcmConst(n, 10000))
	s.SetMuted(true)
	m.PullMix(out)
	if got := sampleAt(out, 0); got != 0 {
		t.Fatalf("muted got %d want 0", got)
	}
}

func TestSoloMutesOthers(t *testing.T) {
	m := New(fmt48, fmt48.FrameBytes(100))
	a := m.AddStream(1, fmt48, 0, 100)
	b := m.AddStream(2, fmt48, 0, 100)
	n := fmt48.FrameBytes(5) / 2
	out := make([]byte, fmt48.FrameBytes(5))

	// Both audible: 5000 + 3000 = 8000.
	a.Write(pcmConst(n, 5000))
	b.Write(pcmConst(n, 3000))
	m.PullMix(out)
	if got := sampleAt(out, 0); got != 8000 {
		t.Fatalf("no solo: got %d want 8000", got)
	}

	// Solo A: only A audible.
	a.Write(pcmConst(n, 5000))
	b.Write(pcmConst(n, 3000))
	a.SetSolo(true)
	m.PullMix(out)
	if got := sampleAt(out, 0); got != 5000 {
		t.Fatalf("solo A: got %d want 5000 (B muted)", got)
	}

	// Clear solo: both audible again.
	a.Write(pcmConst(n, 5000))
	b.Write(pcmConst(n, 3000))
	a.SetSolo(false)
	m.PullMix(out)
	if got := sampleAt(out, 0); got != 8000 {
		t.Fatalf("solo cleared: got %d want 8000", got)
	}
}

func TestUnderrunConceals(t *testing.T) {
	m := New(fmt48, fmt48.FrameBytes(100))
	s := m.AddStream(1, fmt48, 0, 100)
	// Drain past available: the resampler conceals by holding the last sample
	// (repeat-frame), so head and underrun tail both read the buffered value —
	// no garbage, no panic.
	s.Write(pcmConst(100, 7777))
	out := make([]byte, fmt48.FrameBytes(5)) // asks for 240 frames, only 50 buffered
	m.PullMix(out)
	if sampleAt(out, 0) != 7777 {
		t.Fatalf("head sample got %d want 7777", sampleAt(out, 0))
	}
	last := len(out)/2 - 1
	if sampleAt(out, last) != 7777 {
		t.Fatalf("underrun tail got %d want 7777 (held concealment)", sampleAt(out, last))
	}
}

// TestColdStartSilence: an unprimed/empty stream must yield pure silence.
func TestColdStartSilence(t *testing.T) {
	m := New(fmt48, fmt48.FrameBytes(100))
	m.AddStream(1, fmt48, 0, 100) // no data written
	out := make([]byte, fmt48.FrameBytes(5))
	for i := range out {
		out[i] = 0xAA // poison to ensure PullMix overwrites
	}
	m.PullMix(out)
	for i := range len(out) / 2 {
		if sampleAt(out, i) != 0 {
			t.Fatalf("cold-start sample %d = %d, want 0 (silence)", i, sampleAt(out, i))
		}
	}
}

func TestPullMixZeroAlloc(t *testing.T) {
	m := New(fmt48, fmt48.FrameBytes(100))
	s := m.AddStream(1, fmt48, 0, 100)
	out := make([]byte, fmt48.FrameBytes(5))
	frame := pcmConst(len(out)/2, 1000) // preallocated; keep the buffer fed
	if allocs := testing.AllocsPerRun(1000, func() {
		s.Write(frame)
		m.PullMix(out)
	}); allocs != 0 {
		t.Fatalf("PullMix allocates %v/op, want 0", allocs)
	}
}
