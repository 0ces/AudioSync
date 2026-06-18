// Package engine is the control-plane facade over the audio pipeline. It owns a
// running receiver (mixer + playback + UDP receiver), exposes a JSON-friendly
// Snapshot for the UI to render, and accepts commands (volume, mute, rename).
// The desktop app (Wails) and the CLI both drive the pipeline through here.
package engine

import (
	"context"
	"maps"
	"net"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/eplata/audiosync/internal/audio"
	"github.com/eplata/audiosync/internal/config"
	"github.com/eplata/audiosync/internal/discovery"
	"github.com/eplata/audiosync/internal/mixer"
	"github.com/eplata/audiosync/internal/transport"
)

// ConnState is a source's connection liveness.
type ConnState string

const (
	ConnLive    ConnState = "live"
	ConnStalled ConnState = "stalled"
	ConnGone    ConnState = "gone"
)

// Health summarizes a source's jitter-buffer condition.
type Health string

const (
	HealthGood     Health = "good"
	HealthFilling  Health = "filling"
	HealthStarving Health = "starving"
)

// Source is one sender's state for the UI (a mixer channel strip).
type Source struct {
	ID         uint32    `json:"id"`
	Name       string    `json:"name"`
	Platform   string    `json:"platform"` // windows|macos|linux|unknown
	SampleRate uint32    `json:"sampleRate"`
	PeakL      float32   `json:"peakL"` // 0..1
	PeakR      float32   `json:"peakR"`
	Volume     float32   `json:"volume"` // gain, 1.0 == 100%
	Muted      bool      `json:"muted"`
	Solo       bool      `json:"solo"`
	Health     Health    `json:"health"`
	DriftPPM   float32   `json:"driftPpm"`
	Conn       ConnState `json:"conn"`
}

// Snapshot is the full UI state at one instant.
type Snapshot struct {
	Role           string   `json:"role"`
	Running        bool     `json:"running"`
	ListenAddr     string   `json:"listenAddr"`
	OutputDevice   string   `json:"outputDevice"`
	OutputDeviceID string   `json:"outputDeviceId"`
	MasterVolume   float32  `json:"masterVolume"`
	PrefillMs      int      `json:"prefillMs"`
	BufferMs       int      `json:"bufferMs"`
	Sources        []Source `json:"sources"`
}

// Engine owns the running receiver pipeline.
type Engine struct {
	cfg config.Config

	mu      sync.Mutex
	running bool
	cancel  context.CancelFunc
	mx      *mixer.Mixer
	pb      *audio.Playback
	recv    *transport.Receiver
	adv     *discovery.Advertiser

	userNames map[uint32]string             // set via Rename (highest priority)
	metaNames map[uint32]string             // hostname reported by the sender
	platforms map[uint32]transport.Platform // OS reported by the sender

	masterVol  float32 // linear master gain
	deviceID   string  // selected output device hex id ("" == default)
	deviceName string  // display name of the selected output device
}

// New creates an Engine bound to cfg (not yet started). Used by the CLI, which
// already has a parsed config.
func New(cfg config.Config) *Engine {
	return &Engine{
		cfg:        cfg,
		userNames:  make(map[uint32]string),
		metaNames:  make(map[uint32]string),
		platforms:  make(map[uint32]transport.Platform),
		masterVol:  1.0,
		deviceName: "Default Output",
	}
}

// NewReceiver creates a receiver-role Engine listening on listen (e.g. ":4010")
// with default timing. This is the config-free entry point for the desktop UI.
func NewReceiver(listen string) *Engine {
	cfg, err := config.Parse([]string{"-role", "receiver", "-listen", listen})
	if err != nil {
		cfg = config.Config{Role: "receiver", Listen: listen, SampleRate: 48000, FrameMs: 5, PrefillMs: 20, BufferMs: 200}
	}
	return New(cfg)
}

// StartReceiver builds and starts the receiver pipeline. Idempotent.
func (e *Engine) StartReceiver() error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.running {
		return nil
	}

	format := e.cfg.Format()
	mx := mixer.New(format, format.FrameBytes(100))
	mx.SetMasterGain(e.masterVol)
	pb, err := audio.NewPlaybackOnDevice(format, e.cfg.FrameMs, mx.PullMix, e.deviceID)
	if err != nil {
		return err
	}
	if err := pb.Start(); err != nil {
		_ = pb.Close()
		return err
	}

	maxDatagram := format.FrameBytes(e.cfg.FrameMs) + transport.HeaderSize + 256
	recv, err := transport.NewReceiver(e.cfg.Listen, maxDatagram)
	if err != nil {
		_ = pb.Close()
		return err
	}

	ctx, cancel := context.WithCancel(context.Background())
	prefill, capacity := e.cfg.PrefillMs, e.cfg.BufferMs
	go recv.Serve(ctx, transport.Handlers{
		Audio: func(h transport.Header, pcm []byte) {
			s := mx.GetOrAdd(h.StreamID, h.Format, prefill, capacity)
			s.Write(pcm)
		},
		Meta: func(m transport.Meta) {
			e.mu.Lock()
			e.metaNames[m.StreamID] = m.Name
			e.platforms[m.StreamID] = m.Platform
			e.mu.Unlock()
		},
	})

	// Advertise on the LAN so senders can auto-discover this receiver.
	var adv *discovery.Advertiser
	if _, portStr, perr := net.SplitHostPort(recv.LocalAddr().String()); perr == nil {
		if port, cerr := strconv.Atoi(portStr); cerr == nil {
			host, _ := os.Hostname()
			adv, _ = discovery.Advertise(host, port) // best-effort
		}
	}

	e.mx, e.pb, e.recv, e.adv, e.cancel, e.running = mx, pb, recv, adv, cancel, true
	return nil
}

// Stop tears down the pipeline. Idempotent.
func (e *Engine) Stop() {
	e.mu.Lock()
	defer e.mu.Unlock()
	if !e.running {
		return
	}
	e.cancel()
	e.adv.Close()
	_ = e.recv.Close()
	_ = e.pb.Close()
	e.running, e.mx, e.pb, e.recv, e.adv = false, nil, nil, nil, nil
}

// Running reports whether the pipeline is active.
func (e *Engine) Running() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.running
}

// Snapshot assembles the current UI state.
func (e *Engine) Snapshot() Snapshot {
	e.mu.Lock()
	mx, running := e.mx, e.running
	listen := e.cfg.Listen
	if e.recv != nil {
		listen = e.recv.LocalAddr().String()
	}
	snap := Snapshot{
		Role:           e.cfg.Role,
		Running:        running,
		ListenAddr:     listen,
		OutputDevice:   e.deviceName,
		OutputDeviceID: e.deviceID,
		MasterVolume:   e.masterVol,
		PrefillMs:      e.cfg.PrefillMs,
		BufferMs:       e.cfg.BufferMs,
	}
	if mx == nil {
		e.mu.Unlock()
		return snap
	}
	// Copy identity maps under lock, then release before touching the mixer.
	userNames := make(map[uint32]string, len(e.userNames))
	maps.Copy(userNames, e.userNames)
	metaNames := make(map[uint32]string, len(e.metaNames))
	maps.Copy(metaNames, e.metaNames)
	platforms := make(map[uint32]transport.Platform, len(e.platforms))
	maps.Copy(platforms, e.platforms)
	e.mu.Unlock()

	now := time.Now().UnixNano()
	for _, s := range mx.Streams() {
		pl, pr := s.Peaks()
		snap.Sources = append(snap.Sources, Source{
			ID:         s.ID,
			Name:       resolveName(userNames, metaNames, s.ID),
			Platform:   platformString(platforms[s.ID]),
			SampleRate: s.SourceFormat().SampleRate,
			PeakL:      pl,
			PeakR:      pr,
			Volume:     s.Gain(),
			Muted:      s.IsMuted(),
			Solo:       s.IsSolo(),
			Health:     health(s),
			DriftPPM:   s.DriftCorrection() * 1e6,
			Conn:       conn(now, s.LastPacketNS()),
		})
	}
	return snap
}

// SetVolume sets a source's linear gain (1.0 == 100%).
func (e *Engine) SetVolume(id uint32, v float32) {
	e.withStream(id, func(s *mixer.Stream) { s.SetGain(v) })
}

// SetMuted mutes/unmutes a source.
func (e *Engine) SetMuted(id uint32, m bool) {
	e.withStream(id, func(s *mixer.Stream) { s.SetMuted(m) })
}

// SetSolo solos/unsolos a source. While any source is soloed, only soloed
// sources are audible.
func (e *Engine) SetSolo(id uint32, s bool) {
	e.withStream(id, func(st *mixer.Stream) { st.SetSolo(s) })
}

// SetLatencyProfile adjusts the jitter-buffer prefill and capacity (ms) and
// restarts the pipeline if running. Lower prefill = less latency, more risk of
// dropouts under jitter.
func (e *Engine) SetLatencyProfile(prefillMs, bufferMs int) error {
	e.mu.Lock()
	if prefillMs > 0 {
		e.cfg.PrefillMs = prefillMs
	}
	if bufferMs > 0 {
		e.cfg.BufferMs = bufferMs
	}
	wasRunning := e.running
	e.mu.Unlock()
	if !wasRunning {
		return nil
	}
	e.Stop()
	return e.StartReceiver()
}

// SetMasterVolume sets the master output gain (1.0 == 100%).
func (e *Engine) SetMasterVolume(v float32) {
	e.mu.Lock()
	e.masterVol = v
	mx := e.mx
	e.mu.Unlock()
	if mx != nil {
		mx.SetMasterGain(v)
	}
}

// OutputDevice is a selectable system output device (public mirror of the
// internal audio type, so the UI module can name it).
type OutputDevice struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Default bool   `json:"default"`
}

// ListOutputDevices returns the system's playback devices for the UI picker.
func (e *Engine) ListOutputDevices() ([]OutputDevice, error) {
	devs, err := audio.ListOutputDevices()
	if err != nil {
		return nil, err
	}
	out := make([]OutputDevice, len(devs))
	for i, d := range devs {
		out[i] = OutputDevice{ID: d.ID, Name: d.Name, Default: d.Default}
	}
	return out, nil
}

// SetOutputDevice switches playback to the device with the given hex id (""
// selects the system default). If running, playback is restarted on the new
// device; mixer state and incoming streams are unaffected.
func (e *Engine) SetOutputDevice(id, name string) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.deviceID = id
	if name != "" {
		e.deviceName = name
	}
	if !e.running {
		return nil
	}
	// Restart playback on the new device, keeping the same mixer.
	_ = e.pb.Close()
	pb, err := audio.NewPlaybackOnDevice(e.cfg.Format(), e.cfg.FrameMs, e.mx.PullMix, id)
	if err != nil {
		return err
	}
	if err := pb.Start(); err != nil {
		_ = pb.Close()
		return err
	}
	e.pb = pb
	return nil
}

// Rename sets a user-facing label for a source (overrides the sender's name).
func (e *Engine) Rename(id uint32, name string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.userNames[id] = name
}

// resolveName picks the best label: user override > sender-reported > default.
func resolveName(user, meta map[uint32]string, id uint32) string {
	if n := user[id]; n != "" {
		return n
	}
	if n := meta[id]; n != "" {
		return n
	}
	return defaultName(id)
}

func platformString(p transport.Platform) string {
	switch p {
	case transport.PlatformWindows:
		return "windows"
	case transport.PlatformMacOS:
		return "macos"
	case transport.PlatformLinux:
		return "linux"
	default:
		return "unknown"
	}
}

func (e *Engine) withStream(id uint32, fn func(*mixer.Stream)) {
	e.mu.Lock()
	mx := e.mx
	e.mu.Unlock()
	if mx == nil {
		return
	}
	for _, s := range mx.Streams() {
		if s.ID == id {
			fn(s)
			return
		}
	}
}

func defaultName(id uint32) string {
	return "Machine " + itoa(id)
}

func itoa(v uint32) string {
	if v == 0 {
		return "0"
	}
	var b [10]byte
	i := len(b)
	for v > 0 {
		i--
		b[i] = byte('0' + v%10)
		v /= 10
	}
	return string(b[i:])
}

func health(s *mixer.Stream) Health {
	fill, target := s.FillBytes(), s.PrefillBytes()
	switch {
	case !s.IsPrimed() || (target > 0 && fill < target/4):
		return HealthStarving
	case target > 0 && fill > target*2:
		return HealthFilling
	default:
		return HealthGood
	}
}

func conn(nowNS, lastNS int64) ConnState {
	if lastNS == 0 {
		return ConnGone
	}
	age := time.Duration(nowNS - lastNS)
	switch {
	case age < 250*time.Millisecond:
		return ConnLive
	case age < 2*time.Second:
		return ConnStalled
	default:
		return ConnGone
	}
}
