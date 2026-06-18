package audio

import (
	"encoding/hex"
	"fmt"
	"unsafe"

	"github.com/gen2brain/malgo"
)

// PullFunc fills out (interleaved PCM in the device format) with the next
// buffer of audio. It is called from miniaudio's realtime playback thread and
// must not block or allocate.
type PullFunc func(out []byte)

// OutputDevice describes a selectable system output device.
type OutputDevice struct {
	ID      string `json:"id"` // hex-encoded malgo device id ("" == system default)
	Name    string `json:"name"`
	Default bool   `json:"default"`
}

// Playback drives a system output device via miniaudio (malgo). The device's
// realtime thread repeatedly calls the supplied PullFunc to source audio.
type Playback struct {
	ctx    *malgo.AllocatedContext
	device *malgo.Device
	pull   PullFunc
	devID  malgo.DeviceID // kept alive while referenced by the device config
}

// NewPlayback opens the default output device in the given format.
func NewPlayback(format AudioFormat, frameMs int, pull PullFunc) (*Playback, error) {
	return NewPlaybackOnDevice(format, frameMs, pull, "")
}

// NewPlaybackOnDevice opens a specific output device by hex id (from
// ListOutputDevices). An empty id selects the system default. frameMs sets the
// device period (latency vs stability). Currently supports S16LE only.
func NewPlaybackOnDevice(format AudioFormat, frameMs int, pull PullFunc, deviceID string) (*Playback, error) {
	if format.Sample != FormatS16LE {
		return nil, fmt.Errorf("playback: only S16LE supported, got %d", format.Sample)
	}
	ctx, err := malgo.InitContext(nil, malgo.ContextConfig{}, nil)
	if err != nil {
		return nil, fmt.Errorf("playback: init context: %w", err)
	}

	cfg := malgo.DefaultDeviceConfig(malgo.Playback)
	cfg.Playback.Format = malgo.FormatS16
	cfg.Playback.Channels = uint32(format.Channels)
	cfg.SampleRate = format.SampleRate
	cfg.PeriodSizeInMilliseconds = uint32(frameMs)
	cfg.PerformanceProfile = malgo.LowLatency

	p := &Playback{ctx: ctx, pull: pull}
	if deviceID != "" {
		raw, derr := hex.DecodeString(deviceID)
		if derr != nil || len(raw) != len(p.devID) {
			_ = ctx.Uninit()
			ctx.Free()
			return nil, fmt.Errorf("playback: invalid device id")
		}
		copy(p.devID[:], raw)
		cfg.Playback.DeviceID = unsafe.Pointer(&p.devID)
	}

	device, err := malgo.InitDevice(ctx.Context, cfg, malgo.DeviceCallbacks{
		Data: func(out, _ []byte, _ uint32) {
			if len(out) > 0 {
				p.pull(out)
			}
		},
	})
	if err != nil {
		_ = ctx.Uninit()
		ctx.Free()
		return nil, fmt.Errorf("playback: init device: %w", err)
	}
	p.device = device
	return p, nil
}

// Start begins playback.
func (p *Playback) Start() error {
	if err := p.device.Start(); err != nil {
		return fmt.Errorf("playback: start: %w", err)
	}
	return nil
}

// Close stops the device and releases the context.
func (p *Playback) Close() error {
	if p.device != nil {
		p.device.Uninit()
	}
	if p.ctx != nil {
		_ = p.ctx.Uninit()
		p.ctx.Free()
	}
	return nil
}

// ListOutputDevices enumerates the system's playback devices.
func ListOutputDevices() ([]OutputDevice, error) {
	ctx, err := malgo.InitContext(nil, malgo.ContextConfig{}, nil)
	if err != nil {
		return nil, fmt.Errorf("playback: init context: %w", err)
	}
	defer func() {
		_ = ctx.Uninit()
		ctx.Free()
	}()

	infos, err := ctx.Devices(malgo.Playback)
	if err != nil {
		return nil, fmt.Errorf("playback: enumerate devices: %w", err)
	}
	out := make([]OutputDevice, 0, len(infos))
	for i := range infos {
		out = append(out, OutputDevice{
			ID:      hex.EncodeToString(infos[i].ID[:]),
			Name:    infos[i].Name(),
			Default: infos[i].IsDefault != 0,
		})
	}
	return out, nil
}
