// Package ring implements a lock-free single-producer/single-consumer byte
// ring buffer. It is the spine that decouples a realtime audio thread (the
// producer, e.g. a capture callback) from a non-realtime Go goroutine (the
// consumer, e.g. the network sender) without locks, allocation, or blocking.
//
// Exactly one goroutine may call Write and exactly one (different) goroutine
// may call Read concurrently. Any other sharing is undefined.
package ring

import (
	"sync/atomic"
)

// Ring is an SPSC byte ring buffer with a power-of-two capacity.
type Ring struct {
	buf  []byte
	mask uint64
	// head: total bytes written (producer-owned, read by consumer).
	// tail: total bytes read (consumer-owned, read by producer).
	// Monotonic counters; index = value & mask.
	head atomic.Uint64
	tail atomic.Uint64
}

// New returns a Ring whose capacity is the next power of two >= minCapacity.
func New(minCapacity int) *Ring {
	cap := nextPow2(uint64(minCapacity))
	return &Ring{
		buf:  make([]byte, cap),
		mask: cap - 1,
	}
}

// Capacity returns the usable byte capacity of the ring.
func (r *Ring) Capacity() int { return len(r.buf) }

// Len returns the number of bytes currently available to read.
func (r *Ring) Len() int {
	return int(r.head.Load() - r.tail.Load())
}

// Free returns the number of bytes that can currently be written.
func (r *Ring) Free() int {
	return len(r.buf) - r.Len()
}

// Write copies up to len(p) bytes into the ring and returns the number written.
// It never blocks; if the ring is full it returns a short count (possibly 0).
// Producer-side only.
func (r *Ring) Write(p []byte) int {
	head := r.head.Load()
	tail := r.tail.Load()
	free := len(r.buf) - int(head-tail)
	n := min(len(p), free)
	idx := int(head & r.mask)
	first := min(len(r.buf)-idx, n)
	copy(r.buf[idx:idx+first], p[:first])
	if n > first {
		copy(r.buf[0:n-first], p[first:n])
	}
	r.head.Store(head + uint64(n)) // publish after copy
	return n
}

// Read copies up to len(p) bytes out of the ring and returns the number read.
// It never blocks; if the ring is empty it returns a short count (possibly 0).
// Consumer-side only.
func (r *Ring) Read(p []byte) int {
	tail := r.tail.Load()
	head := r.head.Load()
	avail := int(head - tail)
	n := min(len(p), avail)
	idx := int(tail & r.mask)
	first := min(len(r.buf)-idx, n)
	copy(p[:first], r.buf[idx:idx+first])
	if n > first {
		copy(p[first:n], r.buf[0:n-first])
	}
	r.tail.Store(tail + uint64(n)) // release after copy
	return n
}

// Discard drops up to n bytes from the read side, returning how many were
// dropped. Used by drift compensation / overflow handling. Consumer-side only.
func (r *Ring) Discard(n int) int {
	avail := r.Len()
	if n > avail {
		n = avail
	}
	r.tail.Add(uint64(n))
	return n
}

func nextPow2(v uint64) uint64 {
	if v < 2 {
		return 2
	}
	v--
	v |= v >> 1
	v |= v >> 2
	v |= v >> 4
	v |= v >> 8
	v |= v >> 16
	v |= v >> 32
	v++
	return v
}
