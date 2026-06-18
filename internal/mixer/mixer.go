// Package mixer sums N per-sender PCM streams into one output buffer. Each
// stream carries its own jitter buffer (a ring with a prefill threshold). The
// mixer's PullMix method is invoked from the playback realtime thread and must
// stay allocation-free.
//
// Phase 1 assumes every stream and the output share one canonical format:
// 48kHz stereo S16LE. Per-stream resampling/drift compensation is layered in
// later (see internal/drift).
package mixer

import (
	"math"
	"sync"
	"sync/atomic"
	"time"

	"github.com/eplata/audiosync/internal/audio"
	"github.com/eplata/audiosync/internal/drift"
	"github.com/eplata/audiosync/internal/resample"
	"github.com/eplata/audiosync/internal/ring"
)

// Stream is one sender's jitter-buffered audio feeding the mix. Source audio
// arrives at the sender's rate; a per-stream resampler converts it to the
// mixer's output rate, and a drift controller trims the ratio off the buffer
// fill level so latency stays bounded.
type Stream struct {
	ID           uint32
	format       audio.AudioFormat
	rb           *ring.Ring
	prefillBytes int

	baseRatio float64 // srcRate / dstRate
	rs        *resample.Linear
	ctrl      *drift.Controller

	primed atomic.Bool
	muted  atomic.Bool
	solo   atomic.Bool
	gain   atomic.Uint32 // float32 bits; 1.0 == unity

	// Telemetry sampled by the UI (written on the audio/net threads).
	peakL  atomic.Uint32 // float32 bits, 0..1 post-gain peak this callback
	peakR  atomic.Uint32 // float32 bits
	corr   atomic.Uint32 // float32 bits, last drift correction
	lastNS atomic.Int64  // wall-clock ns of last received packet
}

// Write enqueues received PCM for this stream. Called from the network
// goroutine (single producer). Returns bytes accepted (short on overflow).
func (s *Stream) Write(pcm []byte) int {
	s.lastNS.Store(time.Now().UnixNano())
	// On overflow, drop oldest to keep latency bounded rather than newest.
	if need := len(pcm) - s.rb.Free(); need > 0 {
		s.rb.Discard(need)
	}
	return s.rb.Write(pcm)
}

// SetGain sets the linear gain (0..n). SetMuted toggles mute. Both are safe
// from any goroutine.
func (s *Stream) SetGain(g float32)  { s.gain.Store(math.Float32bits(g)) }
func (s *Stream) SetMuted(m bool)    { s.muted.Store(m) }
func (s *Stream) SetSolo(v bool)     { s.solo.Store(v) }
func (s *Stream) gainValue() float32 { return math.Float32frombits(s.gain.Load()) }

// Mixer holds the active streams and the scratch buffers PullMix reuses.
type Mixer struct {
	format  audio.AudioFormat
	frameMs int

	mu   sync.Mutex // guards the streams map + snapshot rebuild (rare ops)
	byID map[uint32]*Stream
	snap atomic.Pointer[[]*Stream] // lock-free read for the audio thread

	masterGain atomic.Uint32 // float32 bits; applied to the final mix

	// Scratch reused by PullMix (audio thread only). Sized for a generous
	// maximum callback length so the hot path never allocates.
	scratch []byte
	accum   []int32
}

// SetMasterGain sets the linear gain applied to the summed mix (1.0 == unity).
func (m *Mixer) SetMasterGain(g float32) { m.masterGain.Store(math.Float32bits(g)) }

// New creates a Mixer for the given output format. maxFrameBytes bounds the
// largest single PullMix request (scratch is sized to it).
func New(format audio.AudioFormat, maxFrameBytes int) *Mixer {
	m := &Mixer{
		format:  format,
		byID:    make(map[uint32]*Stream),
		scratch: make([]byte, maxFrameBytes),
		accum:   make([]int32, maxFrameBytes/2), // S16 samples
	}
	empty := []*Stream{}
	m.snap.Store(&empty)
	m.SetMasterGain(1.0)
	return m
}

// AddStream registers a sender stream, allocating its jitter buffer.
// prefillMs is how much audio must accumulate before playout begins; capacityMs
// bounds the buffer (older audio is dropped past it).
func (m *Mixer) AddStream(id uint32, format audio.AudioFormat, prefillMs, capacityMs int) *Stream {
	m.mu.Lock()
	defer m.mu.Unlock()
	if s, ok := m.byID[id]; ok {
		return s
	}
	rb := ring.New(format.FrameBytes(capacityMs))
	baseRatio := float64(format.SampleRate) / float64(m.format.SampleRate)
	s := &Stream{
		ID:           id,
		format:       format,
		rb:           rb,
		prefillBytes: format.FrameBytes(prefillMs),
		baseRatio:    baseRatio,
		rs:           resample.New(rb, baseRatio),
		ctrl:         drift.New(format.FrameBytes(prefillMs)),
	}
	s.SetGain(1.0)
	m.byID[id] = s
	m.rebuildSnapshot()
	return s
}

// GetOrAdd returns the stream for id, creating it if absent. The common case
// (stream already exists) is served lock-free from the snapshot, so it is cheap
// to call per received packet.
func (m *Mixer) GetOrAdd(id uint32, format audio.AudioFormat, prefillMs, capacityMs int) *Stream {
	for _, s := range *m.snap.Load() {
		if s.ID == id {
			return s
		}
	}
	return m.AddStream(id, format, prefillMs, capacityMs)
}

// RemoveStream drops a sender stream.
func (m *Mixer) RemoveStream(id uint32) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.byID[id]; ok {
		delete(m.byID, id)
		m.rebuildSnapshot()
	}
}

// Streams returns the current set of active streams (lock-free snapshot).
// Safe to call from any goroutine; intended for UI telemetry.
func (m *Mixer) Streams() []*Stream { return *m.snap.Load() }

// Telemetry getters (safe from any goroutine).

func (s *Stream) Peaks() (l, r float32) {
	return math.Float32frombits(s.peakL.Load()), math.Float32frombits(s.peakR.Load())
}
func (s *Stream) DriftCorrection() float32        { return math.Float32frombits(s.corr.Load()) }
func (s *Stream) Gain() float32                   { return s.gainValue() }
func (s *Stream) IsMuted() bool                   { return s.muted.Load() }
func (s *Stream) IsSolo() bool                    { return s.solo.Load() }
func (s *Stream) IsPrimed() bool                  { return s.primed.Load() }
func (s *Stream) FillBytes() int                  { return s.rb.Len() }
func (s *Stream) PrefillBytes() int               { return s.prefillBytes }
func (s *Stream) SourceFormat() audio.AudioFormat { return s.format }
func (s *Stream) LastPacketNS() int64             { return s.lastNS.Load() }

func (m *Mixer) rebuildSnapshot() {
	list := make([]*Stream, 0, len(m.byID))
	for _, s := range m.byID {
		list = append(list, s)
	}
	m.snap.Store(&list)
}

// PullMix fills out (S16LE interleaved) with the summed, clipped mix of all
// active streams. Called from the playback realtime thread. Allocation-free.
func (m *Mixer) PullMix(out []byte) {
	n := min(len(out)/2, len(m.accum)) // S16 sample count
	accum := m.accum[:n]
	for i := range accum {
		accum[i] = 0
	}

	streams := *m.snap.Load()
	anySolo := false
	for _, s := range streams {
		if s.solo.Load() {
			anySolo = true
			break
		}
	}
	for _, s := range streams {
		if s.muted.Load() || (anySolo && !s.solo.Load()) {
			continue
		}
		if !s.primed.Load() {
			if s.rb.Len() < s.prefillBytes {
				continue // still filling jitter buffer
			}
			s.primed.Store(true)
		}
		// Drift-trim the resample ratio off the current buffer fill, then pull
		// output-rate audio (the resampler reads source-rate frames from rb and
		// handles underrun concealment internally).
		corr := s.ctrl.Update(s.rb.Len())
		s.rs.SetRatio(s.baseRatio * (1 + corr))
		buf := m.scratch[:len(out)]
		s.rs.ReadOutput(buf)
		gain := s.gainValue()
		var pkL, pkR int32 // post-gain peak per channel (interleaved L,R)
		for i := range n {
			sample := int16(uint16(buf[2*i]) | uint16(buf[2*i+1])<<8)
			v := int32(float32(sample) * gain)
			accum[i] += v
			if v < 0 {
				v = -v
			}
			if i&1 == 0 {
				if v > pkL {
					pkL = v
				}
			} else if v > pkR {
				pkR = v
			}
		}
		s.peakL.Store(math.Float32bits(float32(pkL) / 32768))
		s.peakR.Store(math.Float32bits(float32(pkR) / 32768))
		s.corr.Store(math.Float32bits(float32(corr)))
	}

	// Apply master gain, clip to int16, write little-endian into out.
	master := math.Float32frombits(m.masterGain.Load())
	for i := range n {
		v := int32(float32(accum[i]) * master)
		if v > 32767 {
			v = 32767
		} else if v < -32768 {
			v = -32768
		}
		out[2*i] = byte(v)
		out[2*i+1] = byte(v >> 8)
	}
}
