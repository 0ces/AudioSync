package ring

import (
	"bytes"
	"sync"
	"testing"
)

func TestCapacityRoundsToPow2(t *testing.T) {
	for _, tc := range []struct{ in, want int }{
		{1, 2}, {2, 2}, {3, 4}, {1000, 1024}, {1024, 1024}, {1025, 2048},
	} {
		if got := New(tc.in).Capacity(); got != tc.want {
			t.Errorf("New(%d).Capacity()=%d want %d", tc.in, got, tc.want)
		}
	}
}

func TestWriteReadRoundtrip(t *testing.T) {
	r := New(16)
	src := []byte("abcdefgh")
	if n := r.Write(src); n != 8 {
		t.Fatalf("Write=%d want 8", n)
	}
	if r.Len() != 8 || r.Free() != 8 {
		t.Fatalf("Len=%d Free=%d want 8/8", r.Len(), r.Free())
	}
	dst := make([]byte, 8)
	if n := r.Read(dst); n != 8 || !bytes.Equal(dst, src) {
		t.Fatalf("Read=%d dst=%q", n, dst)
	}
	if r.Len() != 0 {
		t.Fatalf("Len=%d want 0", r.Len())
	}
}

func TestWraparound(t *testing.T) {
	r := New(8) // capacity 8
	// Fill, drain part, write across the wrap boundary.
	r.Write([]byte("12345678"))
	tmp := make([]byte, 5)
	r.Read(tmp) // tail now at 5
	if n := r.Write([]byte("ABCDE")); n != 5 {
		t.Fatalf("wrap Write=%d want 5", n)
	}
	out := make([]byte, 8)
	got := r.Read(out)
	if want := "678ABCDE"; string(out[:got]) != want {
		t.Fatalf("Read=%q want %q", out[:got], want)
	}
}

func TestWriteFullShortCount(t *testing.T) {
	r := New(4)
	if n := r.Write([]byte("123456")); n != 4 {
		t.Fatalf("Write into full-cap=%d want 4", n)
	}
	if n := r.Write([]byte("x")); n != 0 {
		t.Fatalf("Write when full=%d want 0", n)
	}
}

func TestDiscard(t *testing.T) {
	r := New(8)
	r.Write([]byte("12345678"))
	if d := r.Discard(3); d != 3 || r.Len() != 5 {
		t.Fatalf("Discard=%d Len=%d", d, r.Len())
	}
	if d := r.Discard(100); d != 5 || r.Len() != 0 {
		t.Fatalf("Discard overflow=%d Len=%d", d, r.Len())
	}
}

func TestZeroAllocHotPath(t *testing.T) {
	r := New(4096)
	w := make([]byte, 960)
	rd := make([]byte, 960)
	if allocs := testing.AllocsPerRun(1000, func() {
		r.Write(w)
		r.Read(rd)
	}); allocs != 0 {
		t.Fatalf("Write+Read allocates %v/op, want 0", allocs)
	}
}

// TestSPSCConcurrent streams a byte sequence through the ring across two
// goroutines and verifies nothing is lost or corrupted.
func TestSPSCConcurrent(t *testing.T) {
	r := New(1024)
	const total = 1 << 20
	var wg sync.WaitGroup
	wg.Add(2)

	go func() { // producer
		defer wg.Done()
		buf := make([]byte, 0, 64)
		var i int
		for i < total {
			buf = buf[:0]
			for len(buf) < 64 && i < total {
				buf = append(buf, byte(i))
				i++
			}
			for off := 0; off < len(buf); {
				off += r.Write(buf[off:])
			}
		}
	}()

	go func() { // consumer
		defer wg.Done()
		dst := make([]byte, 128)
		var i int
		for i < total {
			n := r.Read(dst)
			for j := range n {
				if dst[j] != byte(i) {
					t.Errorf("at %d got %d want %d", i, dst[j], byte(i))
					return
				}
				i++
			}
		}
	}()

	wg.Wait()
}
